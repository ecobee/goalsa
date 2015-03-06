// Copyright 2015 Cocoon Alarm Ltd.
//
// See LICENSE file for terms and conditions.

package alsa

import (
	"testing"

	"github.com/jbert/testify/assert"
)

func TestCapture(t *testing.T) {
	a := assert.New(t)

	c, err := NewCaptureDevice("nonexistent", 1, FormatS16LE, 44100)

	a.Equal(c, (*CaptureDevice)(nil), "capture device is nil")
	a.Error(err, "no device error")

	c, err = NewCaptureDevice("null", 1, FormatS32LE, 44100)

	a.NoError(err, "created capture device")

	b1, err := c.ReadS8()

	a.Error(err, "wrong type error")
	a.Equal(b1, ([]int8)(nil), "no buffer allocated")

	b2, err := c.ReadS32()

	samples := 44100 / 32
	a.NoError(err, "read samples ok")
	a.Equal(len(b2), samples, "correct number of samples read")

	c.Close()
}

func TestPlayback(t *testing.T) {
	a := assert.New(t)

	p, err := NewPlaybackDevice("nonexistent", 1, FormatS16LE, 44100)

	a.Equal(p, (*PlaybackDevice)(nil), "playback device is nil")
	a.Error(err, "no device error")

	p, err = NewPlaybackDevice("null", 1, FormatS32LE, 44100)

	a.NoError(err, "created playback device")

	b1 := make([]int8, 100)
	frames, err := p.WriteS8(b1)

	a.Error(err, "wrong type error")
	a.Equal(frames, 0, "no frames written")

	b2 := make([]int32, 100)
	frames, err = p.WriteS32(b2)

	a.NoError(err, "buffer written ok")
	a.Equal(frames, 100, "100 frames written")

	p.Close()
}
