// Copyright 2015 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package newlzma

import (
	"errors"
	"io"
)

// rangeEncoder implements range encoding of single bits. The low value can
// overflow therefore we need uint64. The cache value is used to handle
// overflows.
type rangeEncoder struct {
	w        *lbcWriter
	nrange   uint32
	low      uint64
	cacheLen int64
	cache    byte
}

const maxInt64 = 1<<63 - 1

// newRangeEncoder creates a new range encoder.
func newRangeEncoder(w io.Writer) (re *rangeEncoder, err error) {
	return &rangeEncoder{
		w:        limitByteWriter(w, maxInt64),
		nrange:   0xffffffff,
		cacheLen: 1}, nil
}

func newRangeEncoderLimit(w io.Writer, limit int64) (re *rangeEncoder, err error) {
	if limit < 5 {
		return nil, errors.New("limit must be larger or equal 5")
	}
	return &rangeEncoder{
		w:        limitByteWriter(w, limit),
		nrange:   0xffffffff,
		cacheLen: 1}, nil
}

// Compressed returns the number of bytes that has been written to the
// unterlying writer.
func (e *rangeEncoder) Compressed() int64 {
	return e.w.n
}

// Available returns the number of bytes that still can be written. The
// method takes the bytes that will be currently written by Close into
// account.
func (e *rangeEncoder) Available() int64 {
	return e.w.limit - (e.w.n + e.cacheLen + 4)
}

// writeByte writes a single byte to the underlying writer. An error is
// returned if the limit is reached. The written byte will be counted if
// the underlying writer doesn't return an error.
func (e *rangeEncoder) writeByte(c byte) error {
	if e.Available() < 1 {
		return ErrLimit
	}
	return e.w.WriteByte(c)
}

// DirectEncodeBit encodes the least-significant bit of b with probability 1/2.
func (e *rangeEncoder) DirectEncodeBit(b uint32) error {
	e.nrange >>= 1
	e.low += uint64(e.nrange) & (0 - (uint64(b) & 1))
	if err := e.normalize(); err != nil {
		return err
	}

	return nil
}

// EncodeBit encodes the least significant bit of b. The p value will be
// updated by the function depending on the bit encoded.
func (e *rangeEncoder) EncodeBit(b uint32, p *prob) error {
	bound := p.bound(e.nrange)
	if b&1 == 0 {
		e.nrange = bound
		p.inc()
	} else {
		e.low += uint64(bound)
		e.nrange -= bound
		p.dec()
	}
	if err := e.normalize(); err != nil {
		return err
	}

	return nil
}

// Close writes a complete copy of the low value.
func (e *rangeEncoder) Close() error {
	for i := 0; i < 5; i++ {
		if err := e.shiftLow(); err != nil {
			return err
		}
	}
	return nil
}

// shiftLow shifts the low value for 8 bit. The shifted byte is written into
// the byte writer. The cache value is used to handle overflows.
func (e *rangeEncoder) shiftLow() error {
	if uint32(e.low) < 0xff000000 || (e.low>>32) != 0 {
		tmp := e.cache
		for {
			err := e.writeByte(tmp + byte(e.low>>32))
			if err != nil {
				return err
			}
			tmp = 0xff
			e.cacheLen--
			if e.cacheLen <= 0 {
				if e.cacheLen < 0 {
					panic("negative cacheLen")
				}
				break
			}
		}
		e.cache = byte(uint32(e.low) >> 24)
	}
	e.cacheLen++
	e.low = uint64(uint32(e.low) << 8)
	return nil
}

// normalize handles shifts of nrange and low.
func (e *rangeEncoder) normalize() error {
	const top = 1 << 24
	if e.nrange >= top {
		return nil
	}
	e.nrange <<= 8
	return e.shiftLow()
}

// rangeDecoder decodes single bits of the range encoding stream.
type rangeDecoder struct {
	r      *lbcReader
	nrange uint32
	code   uint32
}

// newRangeDecoder initializes a range decoder. It reads five bytes from the
// reader and therefore may return an error.
func newRangeDecoder(r io.Reader) (d *rangeDecoder, err error) {
	d = &rangeDecoder{r: limitByteReader(r, maxInt64)}
	err = d.init()
	return
}

// newRangeDecoderLimit creates a range decoder with an explicit limit.
func newRangeDecoderLimit(r io.Reader, limit int64) (d *rangeDecoder, err error) {
	if limit < 5 {
		return nil, errors.New("limit must be larger or equal 5")
	}
	d = &rangeDecoder{r: limitByteReader(r, limit)}
	err = d.init()
	return
}

//  Compressed returns the number of bytes that the range encoder read
//  from the underlying reader.
func (d *rangeDecoder) Compressed() int64 {
	return d.r.n
}

// possiblyAtEnd checks whether the decoder may be at the end of the stream.
func (d *rangeDecoder) possiblyAtEnd() bool {
	return d.code == 0
}

// DirectDecodeBit decodes a bit with probability 1/2. The return value b will
// contain the bit at the least-significant position. All other bits will be
// zero.
func (d *rangeDecoder) DirectDecodeBit() (b uint32, err error) {
	d.nrange >>= 1
	d.code -= d.nrange
	t := 0 - (d.code >> 31)
	d.code += d.nrange & t

	// d.code will stay less then d.nrange

	if err = d.normalize(); err != nil {
		return 0, err
	}

	b = (t + 1) & 1

	return b, nil
}

// decodeBit decodes a single bit. The bit will be returned at the
// least-significant position. All other bits will be zero. The probability
// value will be updated.
func (d *rangeDecoder) DecodeBit(p *prob) (b uint32, err error) {
	bound := p.bound(d.nrange)
	if d.code < bound {
		d.nrange = bound
		p.inc()
		b = 0
	} else {
		d.code -= bound
		d.nrange -= bound
		p.dec()
		b = 1
	}

	// d.code will stay less then d.nrange

	if err = d.normalize(); err != nil {
		return 0, err
	}

	return b, nil
}

// init initializes the range decoder, by reading from the byte reader.
func (d *rangeDecoder) init() error {
	d.nrange = 0xffffffff
	d.code = 0

	b, err := d.r.ReadByte()
	if err != nil {
		return err
	}
	if b != 0 {
		return errors.New("first byte not zero")
	}

	for i := 0; i < 4; i++ {
		if err = d.updateCode(); err != nil {
			return err
		}
	}

	if d.code >= d.nrange {
		return errors.New("newRangeDecoder: d.code >= d.nrange")
	}

	return nil
}

// updateCode reads a new byte into the code.
func (d *rangeDecoder) updateCode() error {
	b, err := d.r.ReadByte()
	if err != nil {
		return err
	}
	d.code = (d.code << 8) | uint32(b)
	return nil
}

// normalize the top value and update the code value.
func (d *rangeDecoder) normalize() error {
	// assume d.code < d.nrange
	const top = 1 << 24
	if d.nrange < top {
		d.nrange <<= 8
		// d.code < d.nrange will be maintained
		if err := d.updateCode(); err != nil {
			return err
		}
	}
	return nil
}