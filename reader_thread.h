#pragma once

#include <alsa/asoundlib.h>

typedef struct reader_thread_state_s reader_thread_state;

extern const char *reader_thread_error;

reader_thread_state *reader_thread_start(snd_pcm_t *h, int bytes, int frames, int bufcount);
void reader_thread_stop(reader_thread_state *s);
int reader_thread_poll(reader_thread_state *s, void *buf);

