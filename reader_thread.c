// In a process with a mixture of IO and CPU bound goroutines
// (e.g. reading from alsa while compressing a jpeg snapshot) the IO bound
// thread can experience very high latencies as it gets stuck behind the cpu
// bound tasks. On a fully loaded machine this can exceed the maximum
// buffer size supported by our alsa interfaces (~250ms).
// The golang runtime/scheduler does not provide any mechanism to avoid this,
// with the suggested solution being "install more/faster cpus"
//
// Workaround this by creating a high proirity C thread to read into a larger
// buffer. The end-end latency may still be relatively poor with large amounts
// of jitter, but at least we won't be dropping audio.
// For simplicitly this buffer uses a fixed block size equal to the
// period of the underlying alsa device
//
// In theory this could probably be lockless. However that is hard.
// Hopefully the thunk down to C code before taking the lock in _poll
// is sufficient to isolate us from the terrible golang scheduler latency

// for pthread_setname_np
#define _GNU_SOURCE
#include <pthread.h>
#include <stdbool.h>

#include "reader_thread.h"

struct reader_thread_state_s {
    pthread_mutex_t mu;
    pthread_cond_t cond;
    snd_pcm_t *h;
    pthread_t tid;
    char *buf;
    int head_offset;
    int tail_offset;
    int period_frames;
    int period_bytes;
    int buf_len;
    bool stop;
    bool overrun;
    bool error;
};

const char *reader_thread_error = "no error";

static void *reader_thread_loop(void *arg)
{
    reader_thread_state *s = (reader_thread_state *)arg;
    // Enable realtime priority for this thread
    struct sched_param sched_param;
    sched_param.sched_priority = sched_get_priority_max(SCHED_FIFO);
    pthread_setschedparam(pthread_self(), SCHED_FIFO, &sched_param);
    pthread_setname_np(pthread_self(), "goalsa_reader");

    pthread_mutex_lock(&s->mu);
    while (!s->stop) {
        char *ptr = s->buf + s->head_offset;
        int next_offset = s->head_offset + s->period_bytes;
        // The last buffer may still be in use
        if (next_offset >= s->buf_len) {
            next_offset = 0;
        }
        // If the reader just can't keep up then we may see this anyway
        if (next_offset == s->tail_offset) {
            s->overrun = true;
        }
        pthread_mutex_unlock(&s->mu);
        int rc = snd_pcm_readi(s->h, ptr, s->period_frames);
        pthread_mutex_lock(&s->mu);
        if (rc == -EPIPE) {
            fprintf(stderr, "realtime alsa overrun");
            s->overrun = true;
            snd_pcm_prepare(s->h);
        } else if (rc < 0) {
            reader_thread_error = "snd_pcm_readi";
            s->error = true;
            s->stop = true;
        } else if (s->overrun) {
            // Drop data while waiting for _poll to clear overrun
        } else {
            if (s->head_offset == s->tail_offset) {
                pthread_cond_signal(&s->cond);
            }
            s->head_offset = next_offset;
        }
    }
    pthread_mutex_unlock(&s->mu);

    return NULL;
}

// Copy one block (period_bytes) of audio data into buf
// Returns 0 on success, 1 on overrun, -1 on error.
int reader_thread_poll(reader_thread_state *s, void *buf)
{
    int ret = -1;
    void *src = NULL;
    if (!buf) {
        reader_thread_error = "null buffer";
        return -1;
    }
    pthread_mutex_lock(&s->mu);
    while (s->head_offset == s->tail_offset && !(s->overrun || s->error)) {
        pthread_cond_wait(&s->cond, &s->mu);
    }
    if (s->error) {
        ret = -1;
    } else if (s->overrun) {
        ret = 1;
        s->overrun = false;
        s->tail_offset = s->head_offset;
    } else { // success
        ret = 0;
        src = s->buf + s->tail_offset;
        s->tail_offset += s->period_bytes;
        if (s->tail_offset >= s->buf_len) {
            s->tail_offset = 0;
        }
    }
    pthread_mutex_unlock(&s->mu);
    if (ret == 0) {
        memcpy(buf, src, s->period_bytes);
    }
    return ret;
}

reader_thread_state *reader_thread_start(snd_pcm_t *h, int bytes, int frames, int bufcount)
{
    reader_thread_state *s;
    pthread_attr_t attr;

    s = malloc(sizeof(*s));
    if (!s) {
        reader_thread_error = "malloc failed (state)";
        return NULL;
    }
    memset(s, 0, sizeof(*s));
    pthread_mutex_init(&s->mu, NULL);
    pthread_cond_init(&s->cond, NULL);

    s->h = h;
    s->period_frames = frames;
    s->period_bytes = bytes;
    s->buf_len = bytes * bufcount;
    s->buf = malloc(s->buf_len);
    if (!s->buf) {
        reader_thread_error = "malloc failed (buf)";
        goto error;
    }

    int rc = pthread_create(&s->tid, NULL, reader_thread_loop, s);
    if (rc) {
        reader_thread_error = "pthread_create failed";
        goto error;
    }

    return s;

error:
    free(s->buf);
    free(s);
    return NULL;
}

void reader_thread_stop(reader_thread_state *s)
{
    if (!s) {
        return;
    }

    s->stop = true;
    pthread_join(s->tid, NULL);
    free(s->buf);
    free(s);
}
