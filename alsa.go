// Copyright 2015 Cocoon Alarm Ltd.
//
// See LICENSE file for terms and conditions.

// Package alsa provides Go bindings to the ALSA library.
package alsa

import (
	"errors"
	"fmt"
	"runtime"
	"unsafe"
)

/*
#cgo LDFLAGS: -lasound
#include <alsa/asoundlib.h>
#include <stdint.h>
*/
import "C"

// Format is the type used for specifying sample formats.
type Format C.snd_pcm_format_t

// The range of sample formats supported by ALSA.
const (
	FormatS8 		= C.SND_PCM_FORMAT_S8
	FormatU8 		= C.SND_PCM_FORMAT_U8
	FormatS16LE 		= C.SND_PCM_FORMAT_S16_LE
	FormatS16BE 		= C.SND_PCM_FORMAT_S16_BE
	FormatU16LE	 	= C.SND_PCM_FORMAT_U16_LE
	FormatU16BE 		= C.SND_PCM_FORMAT_U16_BE
	FormatS24LE 		= C.SND_PCM_FORMAT_S24_LE
	FormatS24BE	 	= C.SND_PCM_FORMAT_S24_BE
	FormatU24LE 		= C.SND_PCM_FORMAT_U24_LE
	FormatU24BE 		= C.SND_PCM_FORMAT_U24_BE
	FormatS32LE 		= C.SND_PCM_FORMAT_S32_LE
	FormatS32BE 		= C.SND_PCM_FORMAT_S32_BE
	FormatU32LE 		= C.SND_PCM_FORMAT_U32_LE
	FormatU32BE 		= C.SND_PCM_FORMAT_U32_BE
	FormatFloatLE 		= C.SND_PCM_FORMAT_FLOAT_LE
	FormatFloatBE 		= C.SND_PCM_FORMAT_FLOAT_BE
	FormatFloat64LE 	= C.SND_PCM_FORMAT_FLOAT64_LE
	FormatFloat64BE		= C.SND_PCM_FORMAT_FLOAT64_BE
)

type device struct {
	h *C.snd_pcm_t
	Channels int
	Format Format
	Rate int
	frames int
}

func createError(errorMsg string, errorCode C.int) (err error) {
	strError := C.GoString(C.snd_strerror(errorCode))
	err = fmt.Errorf("%s: %s", errorMsg, strError)
	return
}

func (d *device) createDevice(deviceName string, channels int, format Format, rate int, playback bool) (err error) {
	deviceCString := C.CString(deviceName)
	defer C.free(unsafe.Pointer(deviceCString))
	var ret C.int
	if playback {
		ret = C.snd_pcm_open(&d.h, deviceCString, C.SND_PCM_STREAM_PLAYBACK, 0)
	} else {
		ret = C.snd_pcm_open(&d.h, deviceCString, C.SND_PCM_STREAM_CAPTURE, 0)
	}
	if ret < 0 {
		return fmt.Errorf("could not open ALSA device %s", deviceName)
	}
	runtime.SetFinalizer(d, (*device).Close)
	var hwParams *C.snd_pcm_hw_params_t
	ret = C.snd_pcm_hw_params_malloc(&hwParams)
	if ret < 0 {
		return createError("could not alloc hw params", ret)
	}
	defer C.snd_pcm_hw_params_free(hwParams)
	ret = C.snd_pcm_hw_params_any(d.h, hwParams)
	if ret < 0 {
		return createError("could not set default hw params", ret)
	}
	ret = C.snd_pcm_hw_params_set_access(d.h, hwParams, C.SND_PCM_ACCESS_RW_INTERLEAVED)
	if ret < 0 {
		return createError("could not set access params", ret)
	}
	ret = C.snd_pcm_hw_params_set_format(d.h, hwParams, C.snd_pcm_format_t(format))
	if ret < 0 {
		return createError("could not set format params", ret)
	}
	ret = C.snd_pcm_hw_params_set_channels(d.h, hwParams, C.uint(channels))
	if ret < 0 {
		return createError("could not set channels params", ret)
	}
	ret = C.snd_pcm_hw_params_set_rate(d.h, hwParams, C.uint(rate), 0)
	if ret < 0 {
		return createError("could not set rate params", ret)
	}
	ret = C.snd_pcm_hw_params(d.h, hwParams)
	if ret < 0 {
		return createError("could not set hw params", ret)
	}
	var frames C.snd_pcm_uframes_t
	ret = C.snd_pcm_hw_params_get_period_size(hwParams, &frames, nil)
	if ret < 0 {
		return createError("could not get period size", ret)
	}
	d.frames = int(frames)
	d.Channels = channels
	d.Format = format
	d.Rate = rate
	return
}

// Close closes a device and frees the resources associated with it.
func (d *device) Close() {
	if d.h != nil {
		C.snd_pcm_drain(d.h)
		C.snd_pcm_close(d.h)
		d.h = nil
	}
	runtime.SetFinalizer(d, nil)
}

func formatSampleSize(format Format) (s int) {
	switch format {
	case FormatS8, FormatU8:
		return 1
	case FormatS16LE, FormatS16BE, FormatU16LE, FormatU16BE:
		return 2
	case FormatS24LE, FormatS24BE, FormatU24LE, FormatU24BE, FormatS32LE, FormatS32BE, FormatU32LE, FormatU32BE, FormatFloatLE, FormatFloatBE:
		return 4
	case FormatFloat64LE, FormatFloat64BE:
		return 8
	}
	panic("unsupported format")
}

// CaptureDevice is an ALSA device configured to record audio.
type CaptureDevice struct {
	device
}

// NewCaptureDevice creates a new CaptureDevice object.
func NewCaptureDevice(deviceName string, channels int, format Format, rate int) (c *CaptureDevice, err error) {
	c = new(CaptureDevice)
	err = c.createDevice(deviceName, channels, format, rate, false)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *CaptureDevice) doRead(size int) (frames int, bufPtr unsafe.Pointer, err error) {
	bufPtr = C.malloc(C.size_t(size))
	ret := C.snd_pcm_readi(c.h, bufPtr, C.snd_pcm_uframes_t(c.frames))
	if ret == -C.EPIPE {
		C.snd_pcm_prepare(c.h)
		return 0, nil, errors.New("overrun\n")
	} else if ret < 0 {
		return 0, nil, createError("read error", C.int(ret))
	}
	return int(ret), bufPtr, nil
}

// ReadS8 returns a buffer of signed 8bit audio data.
func (c *CaptureDevice) ReadS8() (buffer []int8, err error) {
	sampleSize := formatSampleSize(c.Format)
	if sampleSize != 1 {
		return nil, errors.New("incompatible data format")
	}
	frames, bufPtr, err := c.doRead(c.frames * c.Channels * sampleSize)
	if err != nil {
		return nil, err
	}
	buffer = make([]int8, frames * c.Channels)
	cBuffer := (*[1 << 30]C.int8_t)(unsafe.Pointer(bufPtr))
	for i := 0; i < frames; i++ {
		for ch := 0; ch < c.Channels; ch++ {
			buffer[i + ch] = int8(cBuffer[i + ch])
		}
	}
	C.free(bufPtr)
	return buffer, nil
}

// ReadU8 returns a buffer of unsigned 8bit audio data.
func (c *CaptureDevice) ReadU8() (buffer []uint8, err error) {
	sampleSize := formatSampleSize(c.Format)
	if sampleSize != 1 {
		return nil, errors.New("incompatible data format")
	}
	frames, bufPtr, err := c.doRead(c.frames * c.Channels * sampleSize)
	if err != nil {
		return nil, err
	}
	buffer = make([]uint8, frames * c.Channels)
	cBuffer := (*[1 << 30]C.uint8_t)(unsafe.Pointer(bufPtr))
	for i := 0; i < frames; i++ {
		for ch := 0; ch < c.Channels; ch++ {
			buffer[i + ch] = uint8(cBuffer[i + ch])
		}
	}
	C.free(bufPtr)
	return buffer, nil
}

// ReadS16 returns a buffer of signed 16bit audio data.
func (c *CaptureDevice) ReadS16() (buffer []int16, err error) {
	sampleSize := formatSampleSize(c.Format)
	if sampleSize != 2 {
		return nil, errors.New("incompatible data format")
	}
	frames, bufPtr, err := c.doRead(c.frames * c.Channels * sampleSize)
	if err != nil {
		return nil, err
	}
	buffer = make([]int16, frames * c.Channels)
	cBuffer := (*[1 << 30]C.int16_t)(unsafe.Pointer(bufPtr))
	for i := 0; i < frames; i++ {
		for ch := 0; ch < c.Channels; ch++ {
			buffer[i + ch] = int16(cBuffer[i + ch])
		}
	}
	C.free(bufPtr)
	return buffer, nil
}

// ReadU16 returns a buffer of unsigned 16bit audio data.
func (c *CaptureDevice) ReadU16() (buffer []uint16, err error) {
	sampleSize := formatSampleSize(c.Format)
	if sampleSize != 2 {
		return nil, errors.New("incompatible data format")
	}
	frames, bufPtr, err := c.doRead(c.frames * c.Channels * sampleSize)
	if err != nil {
		return nil, err
	}
	buffer = make([]uint16, frames * c.Channels)
	cBuffer := (*[1 << 30]C.uint16_t)(unsafe.Pointer(bufPtr))
	for i := 0; i < frames; i++ {
		for ch := 0; ch < c.Channels; ch++ {
			buffer[i + ch] = uint16(cBuffer[i + ch])
		}
	}
	C.free(bufPtr)
	return buffer, nil
}

// ReadS32 returns a buffer of signed 32bit audio data.
func (c *CaptureDevice) ReadS32() (buffer []int32, err error) {
	sampleSize := formatSampleSize(c.Format)
	if sampleSize != 4 {
		return nil, errors.New("incompatible data format")
	}
	frames, bufPtr, err := c.doRead(c.frames * c.Channels * sampleSize)
	if err != nil {
		return nil, err
	}
	buffer = make([]int32, frames * c.Channels)
	cBuffer := (*[1 << 30]C.int32_t)(unsafe.Pointer(bufPtr))
	for i := 0; i < frames; i++ {
		for ch := 0; ch < c.Channels; ch++ {
			buffer[i + ch] = int32(cBuffer[i + ch])
		}
	}
	C.free(bufPtr)
	return buffer, nil
}

// ReadU32 returns a buffer of unsigned 32bit audio data.
func (c *CaptureDevice) ReadU32() (buffer []uint32, err error) {
	sampleSize := formatSampleSize(c.Format)
	if sampleSize != 4 {
		return nil, errors.New("incompatible data format")
	}
	frames, bufPtr, err := c.doRead(c.frames * c.Channels * sampleSize)
	if err != nil {
		return nil, err
	}
	buffer = make([]uint32, frames * c.Channels)
	cBuffer := (*[1 << 30]C.uint32_t)(unsafe.Pointer(bufPtr))
	for i := 0; i < frames; i++ {
		for ch := 0; ch < c.Channels; ch++ {
			buffer[i + ch] = uint32(cBuffer[i + ch])
		}
	}
	C.free(bufPtr)
	return buffer, nil
}

// ReadFloat returns a buffer of floating point 32bit audio data.
func (c *CaptureDevice) ReadFloat() (buffer []float32, err error) {
	sampleSize := formatSampleSize(c.Format)
	if sampleSize != 4 {
		return nil, errors.New("incompatible data format")
	}
	frames, bufPtr, err := c.doRead(c.frames * c.Channels * sampleSize)
	if err != nil {
		return nil, err
	}
	buffer = make([]float32, frames * c.Channels)
	cBuffer := (*[1 << 30]C.float)(unsafe.Pointer(bufPtr))
	for i := 0; i < frames; i++ {
		for ch := 0; ch < c.Channels; ch++ {
			buffer[i + ch] = float32(cBuffer[i + ch])
		}
	}
	C.free(bufPtr)
	return buffer, nil
}

// ReadFloat64 returns a buffer of floating point 64bit audio data.
func (c *CaptureDevice) ReadFloat64() (buffer []float64, err error) {
	sampleSize := formatSampleSize(c.Format)
	if sampleSize != 8 {
		return nil, errors.New("incompatible data format")
	}
	frames, bufPtr, err := c.doRead(c.frames * c.Channels * sampleSize)
	if err != nil {
		return nil, err
	}
	buffer = make([]float64, frames * c.Channels)
	cBuffer := (*[1 << 30]C.double)(unsafe.Pointer(bufPtr))
	for i := 0; i < frames; i++ {
		for ch := 0; ch < c.Channels; ch++ {
			buffer[i + ch] = float64(cBuffer[i + ch])
		}
	}
	C.free(bufPtr)
	return buffer, nil
}

// PlaybackDevice is an ALSA device configured to playback audio.
type PlaybackDevice struct {
	device
}

// NewPlaybackDevice creates a new PlaybackDevice object.
func NewPlaybackDevice(deviceName string, channels int, format Format, rate int) (p *PlaybackDevice, err error) {
	p = new(PlaybackDevice)
	err = p.createDevice(deviceName, channels, format, rate, true)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (p *PlaybackDevice) doWrite(buffer unsafe.Pointer, samples int) (written int, err error) {
	frames := samples / p.Channels
	ret := C.snd_pcm_writei(p.h, buffer, C.snd_pcm_uframes_t(frames))
	if ret == -C.EPIPE {
		C.snd_pcm_prepare(p.h)
		return 0, errors.New("underrun\n")
	} else if ret < 0 {
		return 0, createError("write error", C.int(ret))
	}
	return int(ret), nil
}

// WriteS8 writes a buffer of signed 8bit data to a playback device.
func (p *PlaybackDevice) WriteS8(buffer []int8) (frames int, err error) {
	if formatSampleSize(p.Format) != 1 {
		return 0, errors.New("incompatible data format")
	}
	samples := len(buffer)
	bufPtr := C.malloc(C.size_t(samples))
	cBuffer := (*[1 << 30]C.int8_t)(unsafe.Pointer(bufPtr))
	for i := 0; i < samples; i++ {
		cBuffer[i] = C.int8_t(buffer[i])
	}
	frames, err = p.doWrite(bufPtr, samples)
	C.free(bufPtr)
	return
}

// WriteU8 writes a buffer of unsigned 8bit data to a playback device.
func (p *PlaybackDevice) WriteU8(buffer []uint8) (frames int, err error) {
	if formatSampleSize(p.Format) != 1 {
		return 0, errors.New("incompatible data format")
	}
	samples := len(buffer)
	bufPtr := C.malloc(C.size_t(samples))
	cBuffer := (*[1 << 30]C.uint8_t)(unsafe.Pointer(bufPtr))
	for i := 0; i < samples; i++ {
		cBuffer[i] = C.uint8_t(buffer[i])
	}
	frames, err = p.doWrite(bufPtr, samples)
	C.free(bufPtr)
	return
}

// WriteS16 writes a buffer of signed 16bit data to a playback device.
func (p *PlaybackDevice) WriteS16(buffer []int16) (frames int, err error) {
	if formatSampleSize(p.Format) != 2 {
		return 0, errors.New("incompatible data format")
	}
	samples := len(buffer)
	bufPtr := C.malloc(C.size_t(samples) * 2)
	cBuffer := (*[1 << 30]C.int16_t)(unsafe.Pointer(bufPtr))
	for i := 0; i < samples; i++ {
		cBuffer[i] = C.int16_t(buffer[i])
	}
	frames, err = p.doWrite(bufPtr, samples)
	C.free(bufPtr)
	return
}

// WriteU16 writes a buffer of unsigned 16bit data to a playback device.
func (p *PlaybackDevice) WriteU16(buffer []uint16) (frames int, err error) {
	if formatSampleSize(p.Format) != 2 {
		return 0, errors.New("incompatible data format")
	}
	samples := len(buffer)
	bufPtr := C.malloc(C.size_t(samples) * 2)
	cBuffer := (*[1 << 30]C.uint16_t)(unsafe.Pointer(bufPtr))
	for i := 0; i < samples; i++ {
		cBuffer[i] = C.uint16_t(buffer[i])
	}
	frames, err = p.doWrite(bufPtr, samples)
	C.free(bufPtr)
	return
}

// WriteS32 writes a buffer of signed 32bit data to a playback device.
func (p *PlaybackDevice) WriteS32(buffer []int32) (frames int, err error) {
	if formatSampleSize(p.Format) != 4 {
		return 0, errors.New("incompatible data format")
	}
	samples := len(buffer)
	bufPtr := C.malloc(C.size_t(samples) * 4)
	cBuffer := (*[1 << 30]C.int32_t)(unsafe.Pointer(bufPtr))
	for i := 0; i < samples; i++ {
		cBuffer[i] = C.int32_t(buffer[i])
	}
	frames, err = p.doWrite(bufPtr, samples)
	C.free(bufPtr)
	return
}

// WriteU32 writes a buffer of unsigned 32bit data to a playback device.
func (p *PlaybackDevice) WriteU32(buffer []uint32) (frames int, err error) {
	if formatSampleSize(p.Format) != 4 {
		return 0, errors.New("incompatible data format")
	}
	samples := len(buffer)
	bufPtr := C.malloc(C.size_t(samples) * 4)
	cBuffer := (*[1 << 30]C.uint32_t)(unsafe.Pointer(bufPtr))
	for i := 0; i < samples; i++ {
		cBuffer[i] = C.uint32_t(buffer[i])
	}
	frames, err = p.doWrite(bufPtr, samples)
	C.free(bufPtr)
	return
}

// WriteFloat writes a buffer of floating point 32bit data to a playback device.
func (p *PlaybackDevice) WriteFloat(buffer []float32) (frames int, err error) {
	if formatSampleSize(p.Format) != 4 {
		return 0, errors.New("incompatible data format")
	}
	samples := len(buffer)
	bufPtr := C.malloc(C.size_t(samples) * 4)
	cBuffer := (*[1 << 30]C.float)(unsafe.Pointer(bufPtr))
	for i := 0; i < samples; i++ {
		cBuffer[i] = C.float(buffer[i])
	}
	frames, err = p.doWrite(bufPtr, samples)
	C.free(bufPtr)
	return
}

// WriteFloat64 writes a buffer of floating point 64bit data to a playback device.
func (p *PlaybackDevice) WriteFloat64(buffer []float64) (frames int, err error) {
	if formatSampleSize(p.Format) != 8 {
		return 0, errors.New("incompatible data format")
	}
	samples := len(buffer)
	bufPtr := C.malloc(C.size_t(samples) * 8)
	cBuffer := (*[1 << 30]C.double)(unsafe.Pointer(bufPtr))
	for i := 0; i < samples; i++ {
		cBuffer[i] = C.double(buffer[i])
	}
	frames, err = p.doWrite(bufPtr, samples)
	C.free(bufPtr)
	return
}
