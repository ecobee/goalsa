// Copyright 2015-2016 Cocoon Labs Ltd.
//
// See LICENSE file for terms and conditions.

package alsa

import (
	"testing"

	"github.com/cocoonlife/testify/assert"
)

func TestCapture(t *testing.T) {
	a := assert.New(t)

	c, err := NewCaptureDevice("nonexistent", 1, FormatS16LE, 44100,
		BufferParams{})

	a.Equal(c, (*CaptureDevice)(nil), "capture device is nil")
	a.Error(err, "no device error")

	c, err = NewCaptureDevice("null", 1, FormatS32LE, 0, BufferParams{})

	a.Equal(c, (*CaptureDevice)(nil), "capture device is nil")
	a.Error(err, "bad rate error")

	c, err = NewCaptureDevice("null", 1, FormatS32LE, 44100, BufferParams{})

	a.NoError(err, "created capture device")

	b1 := make([]int8, 100)
	samples, err := c.Read(b1)

	a.Error(err, "wrong type error")
	a.Equal(samples, 0, "no samples read")

	b2 := make([]int16, 200)
	samples, err = c.Read(b2)

	a.Error(err, "wrong type error")
	a.Equal(samples, 0, "no samples read")

	b3 := make([]float64, 50)
	samples, err = c.Read(b3)

	a.Error(err, "wrong type error")
	a.Equal(samples, 0, "no samples read")

	b4 := make([]int32, 200)
	samples, err = c.Read(b4)

	a.NoError(err, "read samples ok")
	a.Equal(len(b2), samples, "correct number of samples read")

	c.Close()
}

func TestPlayback(t *testing.T) {
	a := assert.New(t)

	p, err := NewPlaybackDevice("nonexistent", 1, FormatS16LE, 44100,
		BufferParams{})

	a.Equal(p, (*PlaybackDevice)(nil), "playback device is nil")
	a.Error(err, "no device error")

	p, err = NewPlaybackDevice("null", 0, FormatS32LE, 44100,
		BufferParams{})

	a.Equal(p, (*PlaybackDevice)(nil), "playback device is nil")
	a.Error(err, "bad channels error")

	p, err = NewPlaybackDevice("null", 1, FormatS32LE, 44100,
		BufferParams{})

	a.NoError(err, "created playback device")

	b1 := make([]int8, 100)
	frames, err := p.Write(b1)

	a.Error(err, "wrong type error")
	a.Equal(frames, 0, "no frames written")

	b2 := make([]int16, 100)
	frames, err = p.Write(b2)

	a.Error(err, "wrong type error")
	a.Equal(frames, 0, "no frames written")

	b3 := make([]float64, 100)
	frames, err = p.Write(b3)

	a.Error(err, "wrong type error")
	a.Equal(frames, 0, "no frames written")

	b4 := make([]int32, 100)
	frames, err = p.Write(b4)

	a.NoError(err, "buffer written ok")
	a.Equal(frames, 100, "100 frames written")

	p.Close()
}
