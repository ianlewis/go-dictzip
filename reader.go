// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dictzip

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

var (
	errInvalidIDHeader        = errors.New("invalid ID header")
	errMissingHeader          = errors.New("dictzip header data not found")
	errUnsupportedCompression = errors.New("unsupported compression method")
	errUnsupportedVersion     = errors.New("unsupported dictzip version")
)

// readCloseResetter is an interface that wraps the io.ReadCloser and
// flate.Resetter interfaces. This is used because the flate.NewReader
// unfortunately returns an io.ReadCloser instead of a concrete type.
type readCloseResetter interface {
	io.ReadCloser
	flate.Resetter
}

// Reader implements [io.Reader] and [io.ReaderAt]. It provides random access
// to the compressed data.
type Reader struct {
	r         io.ReadSeeker
	z         readCloseResetter
	chunkSize int64
	offsets   []int64
}

// NewReader returns a new dictzip [Reader] reading compressed data from the
// given reader. It does not assume control of the given [io.Reader]. It is the
// responsibility of the caller to Close on that reader when it is not longer
// used.
//
// It is the callers responsibility to call [Close] on the returned [Reader]
// when done.
func NewReader(r io.ReadSeeker) (*Reader, error) {
	fr := flate.NewReader(r)
	z := &Reader{
		r: r,
		z: fr.(readCloseResetter),
	}
	if err := z.Reset(r); err != nil {
		return nil, err
	}

	return z, nil
}

// Reset discards the reader's state and resets it to the initial state as
// returned by NewReader but reading from the r instead.
func (z *Reader) Reset(r io.Reader) error {
	// Read the first 10 bytes of the header.
	chunkSize, offsets, err := readHeader(r)
	if err != nil {
		return err
	}

	z.chunkSize = chunkSize
	z.offsets = offsets

	//nolint:wrapcheck // error does not need to be wrapped
	return z.z.Reset(r, nil)
}

// Close closes the reader. It does not close the underlying io.Reader.
func (z *Reader) Close() error {
	//nolint:wrapcheck // error does not need to be wrapped
	return z.z.Close()
}

// Read implements [io.Reader].
func (z *Reader) Read(_ []byte) (int, error) {
	// TODO(#3): implement Read
	return 0, nil
}

// ReadAt implements the [io.ReaderAt] interface.
func (z *Reader) ReadAt(p []byte, off int64) (int, error) {
	buf, err := z.readChunk(off, int64(len(p)))
	if err != nil {
		return 0, err
	}
	copy(p, buf)
	return len(p), nil
}

func (z *Reader) Seek(_ int64, _ int) (int64, error) {
	// TODO(#3): implement Seek
	return 0, nil
}

func (z *Reader) readChunk(offset, size int64) ([]byte, error) {
	chunkNum := offset / z.chunkSize
	chunkOffset := z.offsets[chunkNum]

	if _, err := z.r.Seek(chunkOffset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("Seek: %w", err)
	}

	// Reset the flate.Reader
	if err := z.z.Reset(z.r, nil); err != nil {
		return nil, fmt.Errorf("Reset: %w", err)
	}

	// The offset into the file at the start of the chunk.
	chunkFileOffset := chunkNum * z.chunkSize

	// The size to read from the chunk. Includes some amount of data
	// (offset - chunkFileOffset bytes) at the beginning of the chunk that will
	// be discarded.
	chunkReadSize := size + (offset - chunkFileOffset)

	buf := make([]byte, chunkReadSize)
	_, err := io.ReadFull(z.z, buf)
	if err != nil {
		return nil, fmt.Errorf("decompressing: %w", err)
	}

	return buf[chunkReadSize-size:], nil
}

// gzip Header Values
//nolint:godot // diagram
/*
+---+---+---+---+---+---+---+---+---+---+
|ID1|ID2|CM |FLG|     MTIME     |XFL|OS |
+---+---+---+---+---+---+---+---+---+---+
*/
var (
	// hdrGzipID is the gzip header values for bytes ID1 and ID2.
	hdrGzipID = []byte{0x1f, 0x8b}

	// hdrDeflateCM is the deflate CM (Compression method).
	hdrDeflateCM = byte(0x08)
)

// FLG (Flags).
// bit 0 : FTEXT (ignored).
// bit 1 : FHCRC (ignored).
// bit 2 : FEXTRA (required for dictzip).
// bit 3 : FNAME (ignored).
// bit 4 : FCOMMENT (ignored).
// bit 5 : reserved (ignored).
// bit 6 : reserved (ignored).
// bit 7 : reserved	(ignored).
var (
	flgCRC     = byte(2)
	flgEXTRA   = byte(4)
	flgNAME    = byte(8)
	flgCOMMENT = byte(16)
)

// readFlg reads and validates the gzip header, and returns the FLG byte.
func readFlg(r io.Reader) (int, byte, error) {
	head := make([]byte, 10)
	n, err := io.ReadFull(r, head)
	if err != nil {
		return n, 0, fmt.Errorf("reading dictzip header: %w", err)
	}

	// ID1, ID2 headers must be 31 and 139 respectively.
	if !bytes.Equal(head[0:2], hdrGzipID) {
		return n, head[3], fmt.Errorf("%w: %x", errInvalidIDHeader, head[0:2])
	}

	if head[2] != hdrDeflateCM {
		return n, head[3], fmt.Errorf("%w: %x", errUnsupportedCompression, head[2])
	}

	// NOTE: MTIME, XFL, OS are ignored.

	return n, head[3], nil
}

// readExtra parses the EXTRA header. It returns dictzip chunk size before //
// compression (before compression all chunks have equal size), and a list of
// chunk sizes after compression.
func readExtra(r io.Reader) (int, int64, []int64, error) {
	var totalRead int

	// FEXTRA
	buf := make([]byte, 2)
	n, err := io.ReadFull(r, buf)
	totalRead += n
	if err != nil {
		return totalRead, 0, nil, fmt.Errorf("reading EXTRA XLEN: %w", err)
	}
	xlen := binary.LittleEndian.Uint16(buf)

	extra := make([]byte, xlen)
	n, err = io.ReadFull(r, extra)
	totalRead += n
	if err != nil {
		return totalRead, 0, nil, fmt.Errorf("reading EXTRA: %w", err)
	}

	// NOTE: The EXTRA field could could contain multiple sub-fields.
	var chunkSize int64
	var sizes []int64

	er := bytes.NewReader(extra)
	var foundRAField bool
	for er.Len() > 0 {
		// Read SI1, SI2, and LEN
		buf = make([]byte, 4)
		_, err = io.ReadFull(er, buf)
		if err != nil {
			return totalRead, 0, nil, fmt.Errorf("reading EXTRA: %w", err)
		}

		si1 := buf[0]
		si2 := buf[1]
		extraLen := binary.LittleEndian.Uint16(buf[2:])

		// Read the subfield data.
		extraBuf := make([]byte, extraLen)
		_, err = io.ReadFull(er, extraBuf)
		if err != nil {
			return totalRead, 0, nil, fmt.Errorf("reading EXTRA: %w", err)
		}

		// This is the dictzip 'R'andom 'A'ccess data field.
		if si1 == 'R' && si2 == 'A' {
			var err error
			chunkSize, sizes, err = readExtraSizes(bytes.NewReader(extraBuf))
			if err != nil {
				return totalRead, 0, nil, err
			}
			foundRAField = true
		}
	}

	if !foundRAField {
		return totalRead, 0, nil, fmt.Errorf("%w: no RA EXTRA field", errMissingHeader)
	}

	return totalRead, chunkSize, sizes, nil
}

// readExtraSizes reads the dictzip uncompressed chunk size and compressed
// chunk sizes from the EXTRA field data.
func readExtraSizes(r io.Reader) (int64, []int64, error) {
	var buf []byte

	// Read VER
	buf = make([]byte, 2)
	_, err := io.ReadFull(r, buf)
	if err != nil {
		return 0, nil, fmt.Errorf("reading VER: %w", err)
	}
	ver := binary.LittleEndian.Uint16(buf)

	if ver != 1 {
		return 0, nil, fmt.Errorf("%w: %d", errUnsupportedVersion, ver)
	}

	// Read CHLEN
	buf = make([]byte, 2)
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return 0, nil, fmt.Errorf("reading CHLEN: %w", err)
	}
	chlen := binary.LittleEndian.Uint16(buf)

	// Read CHCNT
	buf = make([]byte, 2)
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return 0, nil, fmt.Errorf("reading CHLEN: %w", err)
	}
	chcnt := binary.LittleEndian.Uint16(buf)

	var offsets []int64
	for i := 0; i < int(chcnt); i++ {
		buf = make([]byte, 2)
		_, err = io.ReadFull(r, buf)
		if err != nil {
			return 0, nil, fmt.Errorf("reading CHLEN: %w", err)
		}
		offsets = append(offsets, int64(binary.LittleEndian.Uint16(buf)))
	}

	return int64(chlen), offsets, nil
}

// readHeader reads the gzip header for dictzip specific headers and returns
// offsets and blocksize used for random access.
func readHeader(r io.Reader) (int64, []int64, error) {
	var chunkSize int64
	var sizes []int64
	var startOffset int

	n, flg, err := readFlg(r)
	startOffset += n
	if err != nil {
		return 0, nil, err
	}

	if flg&flgEXTRA == 0 {
		return 0, nil, fmt.Errorf("%w: no EXTRA field", errMissingHeader)
	}

	// Read the EXTRA field
	n, chunkSize, sizes, err = readExtra(r)
	startOffset += n
	if err != nil {
		return 0, nil, err
	}

	// Skip the NAME
	if flg&flgNAME != 0 {
		buf := make([]byte, 1)
		for {
			n, err := io.ReadFull(r, buf)
			startOffset += n
			if err != nil {
				return 0, nil, fmt.Errorf("reading name header: %w", err)
			}
			if buf[0] == 0 {
				break
			}
		}
	}

	// Skip the COMMENT
	if flg&flgCOMMENT != 0 {
		buf := make([]byte, 1)
		for {
			n, err := io.ReadFull(r, buf)
			startOffset += n
			if err != nil {
				return 0, nil, fmt.Errorf("reading comment header: %w", err)
			}
			if buf[0] == 0 {
				break
			}
		}
	}

	// Skip the CRC
	// TODO(#6): Perform a CRC check
	if flg&flgCRC != 0 {
		buf := make([]byte, 2)
		n, err := io.ReadFull(r, buf)
		startOffset += n
		if err != nil {
			return 0, nil, fmt.Errorf("reading comment header: %w", err)
		}
	}

	// calculate the offsets
	offsets := make([]int64, len(sizes)+1)
	offsets[0] = int64(startOffset)
	for i := 0; i < len(sizes); i++ {
		offsets[i+1] = offsets[i] + sizes[i]
	}

	return chunkSize, offsets, nil
}
