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
	"hash"
	"hash/crc32"
	"io"
	"strings"
	"time"
)

var (
	// errDictzip is the base error for all go-dictzip errors.
	errDictzip = errors.New("dictzip")

	// ErrHeader indicates an error with gzip header data.
	ErrHeader = fmt.Errorf("%w: invalid header", errDictzip)

	errUnsupportedSeek = fmt.Errorf("%w: unsupported seek mode", errDictzip)
	errNegativeOffset  = fmt.Errorf("%w: negative offset", errDictzip)
)

const (
	// OSFAT represents an FAT filesystem OS (MS-DOS, OS/2, NT/Win32).
	OSFAT byte = iota

	// OSAmiga represents the Amiga OS.
	OSAmiga

	// OSVMS represents VMS (or OpenVMS).
	OSVMS

	// OSUnix represents Unix operating systems.
	OSUnix

	// OSVM represents VM/CMS.
	OSVM

	// OSAtari represents Atari TOS.
	OSAtari

	// OSHPFS represents HPFS filesystem (OS/2, NT).
	OSHPFS

	// OSMacintosh represents the Macintosh operating system.
	OSMacintosh

	// OSZSystem represents Z-System.
	OSZSystem

	// OSCPM represents the CP/M operating system.
	OSCPM

	// OSTOPS20 represents the TOPS-20 operating system.
	OSTOPS20

	// OSNTFS represents an NTFS filesystem OS (NT).
	OSNTFS

	// OSQDOS represents QDOS.
	OSQDOS

	// OSAcorn represents Acorn RISCOS.
	OSAcorn

	// OSUnknown represents an unknown operating system.
	OSUnknown = 0xff
)

const (
	// XFLSlowest indicates that the compressor used maximum compression (e.g. slowest algorithm).
	XFLSlowest byte = 0x2

	// XFLFastest indicates that the compressor used the fastest algorithm.
	XFLFastest byte = 0x4
)

func headerErr(err error) error {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return fmt.Errorf("%w: %w", ErrHeader, err)
	}
	return fmt.Errorf("%w: %w", errDictzip, err)
}

// readCloseResetter is an interface that wraps the io.ReadCloser and
// flate.Resetter interfaces. This is used because the flate.NewReader
// unfortunately returns an io.ReadCloser instead of a concrete type.
type readCloseResetter interface {
	io.ReadCloser
	flate.Resetter
}

// Header is the gzip file header.
//
// Strings must be UTF-8 encoded and may only contain Unicode code points
// U+0001 through U+00FF, due to limitations of the gzip file format.
type Header struct {
	// Comment is the COMMENT header field.
	Comment string

	// Extra includes all EXTRA sub-fields except the dictzip RA sub-field.
	Extra []byte

	// ModTime is the MTIME modification time field.
	ModTime time.Time

	// Name is the NAME header field.
	Name string

	// OS is the OS header field.
	OS byte

	// chunkSize is the size of uncompressed dictzip chunks.
	chunkSize int

	// sizes is a list of sizes of the compressed chunks in the file.
	sizes []int
}

// ChunkSize returns the dictzip uncompressed data chunk size.
func (h *Header) ChunkSize() int {
	return h.chunkSize
}

// Sizes returns the dictzip sizes for the compressed data chunks.
func (h *Header) Sizes() []int {
	return h.sizes
}

// Reader implements [io.Reader] and [io.ReaderAt]. It provides random access
// to the compressed data.
type Reader struct {
	// Header is the gzip header data and is valid after [NewReader] or
	// [Reader.Reset].
	Header

	r io.ReadSeeker
	z readCloseResetter

	// offset is the offset into the uncompressed data.
	offset int64

	// offsets is a list of offsets to the compressed chunks in the file.
	offsets []int64

	// digest is the CRC-32 digest (IEEE polynomial).
	// See RFC-1952 Section 2.3.1.
	digest hash.Hash32
}

// NewReader returns a new dictzip [Reader] reading compressed data from the
// given reader. It does not assume control of the given [io.Reader]. It is the
// responsibility of the caller to Close on that reader when it is not longer
// used.
//
// NewReader will call Seek on the given reader to ensure that it is being read
// from the beginning.
//
// It is the callers responsibility to call [Reader.Close] on the returned
// [Reader] when done.
func NewReader(r io.ReadSeeker) (*Reader, error) {
	fr := flate.NewReader(r)
	z := &Reader{
		z: fr.(readCloseResetter),
	}
	if err := z.Reset(r); err != nil {
		return nil, err
	}

	return z, nil
}

// Reset discards the reader's state and resets it to the initial state as
// returned by NewReader but reading from the r instead.
//
// Reset will call Seek on the given reader to ensure that it is being read
// from the beginning.
func (z *Reader) Reset(r io.ReadSeeker) error {
	z.r = r
	z.offset = 0
	if _, err := r.Seek(z.offset, io.SeekStart); err != nil {
		return fmt.Errorf("%w: Seek: %w", errDictzip, err)
	}

	// Read the first 10 bytes of the header.
	_, chunkSize, offsets, err := z.readHeader()
	if err != nil {
		return err
	}
	z.chunkSize = chunkSize
	z.offsets = offsets

	if err := z.z.Reset(r, nil); err != nil {
		return fmt.Errorf("%w: Reset: %w", errDictzip, err)
	}

	return nil
}

// Close closes the reader. It does not close the underlying io.Reader.
func (z *Reader) Close() error {
	//nolint:wrapcheck // error does not need to be wrapped
	return z.z.Close()
}

// Read implements [io.Reader].
func (z *Reader) Read(p []byte) (int, error) {
	buf, err := z.readChunk(z.offset, len(p))
	n := copy(p, buf)
	z.offset += int64(n)
	return n, err
}

// ReadAt implements [io.ReaderAt.ReadAt].
func (z *Reader) ReadAt(p []byte, off int64) (int, error) {
	buf, err := z.readChunk(off, len(p))
	return copy(p, buf), err
}

// Seek implements [io.Seeker.Seek].
func (z *Reader) Seek(offset int64, whence int) (int64, error) {
	var err error

	switch whence {
	case io.SeekStart:
		if offset < 0 {
			err = errNegativeOffset
		} else {
			z.offset = offset
		}
	case io.SeekCurrent:
		newOffset := z.offset + offset
		if newOffset < 0 {
			err = errNegativeOffset
		} else {
			z.offset = newOffset
		}
	default:
		err = fmt.Errorf("%w: %v", errUnsupportedSeek, whence)
	}

	return z.offset, err
}

// readChunk reads and decompresses data of size at offset. It returns the
// number of bytes advanced in the underlying reader and bytes read.
func (z *Reader) readChunk(offset int64, size int) ([]byte, error) {
	chunkNum := offset / int64(z.chunkSize)
	if chunkNum >= int64(len(z.offsets)) {
		// NOTE: We are trying to seek past the end of the file.
		return nil, io.EOF
	}
	chunkOffset := z.offsets[chunkNum]

	if _, err := z.r.Seek(chunkOffset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("Seek: %w", err)
	}

	// Reset the flate.Reader
	if err := z.z.Reset(z.r, nil); err != nil {
		return nil, fmt.Errorf("Reset: %w", err)
	}

	// The offset into the file at the start of the chunk.
	chunkFileOffset := chunkNum * int64(z.chunkSize)

	// The size to read from the chunk. Includes some amount of data
	// (readStart bytes) at the beginning of the chunk that will
	// be discarded.
	int64size := int64(size)
	readStart := (offset - chunkFileOffset)
	chunkReadSize := int64size + readStart

	buf := make([]byte, chunkReadSize)
	totalRead := int64(0)
	var err error

	// Attempt to read the full amount requested.
	// NOTE: It seems that the flate.Reader may read less than the given buffer
	// size and still not return an error if reading across a sync marker. This
	// is different than most io.Reader implementations.
	for err == nil && totalRead < chunkReadSize {
		var n int
		n, err = z.z.Read(buf[totalRead:])
		totalRead += int64(n)
	}

	// Check if we read less bytes than the start of our read.
	if totalRead < readStart {
		//nolint:wrapcheck // we must return unwrapped io.EOF for io.Reader
		return nil, err
	}

	//nolint:wrapcheck // we must return unwrapped io.EOF for io.Reader
	return buf[readStart:totalRead], err
}

// gzip Header Values
//nolint:godot // diagram
/*
+---+---+---+---+---+---+---+---+---+---+
|ID1|ID2|CM |FLG|     MTIME     |XFL|OS |
+---+---+---+---+---+---+---+---+---+---+
*/
const (
	// hdrGzipID1 is the gzip header value for ID1
	hdrGzipID1 byte = 0x1f

	// hdrGzipID2 is the gzip header value for ID2
	hdrGzipID2 byte = 0x8b

	// hdrDeflateCM is the deflate CM (Compression method).
	hdrDeflateCM byte = 0x08
)

const (
	// hdrDictzipSI1 is the dictzip random access subfield ID value SI1.
	hdrDictzipSI1 = byte('R')

	// hdrDictzipSI2 is the dictzip random access subfield ID value SI2.
	hdrDictzipSI2 = byte('A')
)

// FLG (Flags).
// bit 0 : FTEXT (ignored).
// bit 1 : FHCRC.
// bit 2 : FEXTRA (required for dictzip).
// bit 3 : FNAME.
// bit 4 : FCOMMENT.
// bit 5 : reserved (ignored).
// bit 6 : reserved (ignored).
// bit 7 : reserved	(ignored).
const (
	flgCRC     = byte(1 << 1)
	flgEXTRA   = byte(1 << 2)
	flgNAME    = byte(1 << 3)
	flgCOMMENT = byte(1 << 4)
)

// readFlg reads and validates the gzip header, and returns the FLG byte.
func (z *Reader) readFlg() (int, byte, error) {
	head := make([]byte, 10)
	n, err := io.ReadFull(z.r, head)
	if err != nil {
		return n, 0, headerErr(fmt.Errorf("reading header: %w", err))
	}

	if head[0] != hdrGzipID1 || head[1] != hdrGzipID2 {
		return n, head[3], fmt.Errorf("%w: ID1,ID2: %x", ErrHeader, head[0:2])
	}

	if head[2] != hdrDeflateCM {
		return n, head[3], fmt.Errorf("%w: CM: %x", ErrHeader, head[2])
	}

	// NOTE: The zero value for MTIME means that the modified time is not set.
	if mtime := binary.LittleEndian.Uint32(head[4:8]); mtime > 0 {
		z.Header.ModTime = time.Unix(int64(mtime), 0)
	}

	// NOTE: XFL (head[8]) is ignored.

	z.Header.OS = head[9]

	z.digest = crc32.NewIEEE()

	return n, head[3], nil
}

// readExtra parses the EXTRA header. It returns dictzip chunk size before
// compression (before compression all chunks have equal size), and a list of
// chunk sizes after compression.
func (z *Reader) readExtra() (int, int, []int, error) {
	var totalRead int

	// FEXTRA
	buf := make([]byte, 2)
	n, err := io.ReadFull(z.r, buf)
	totalRead += n
	if err != nil {
		return totalRead, 0, nil, headerErr(fmt.Errorf("EXTRA XLEN: %w", err))
	}
	xlen := binary.LittleEndian.Uint16(buf)
	z.digest.Write(buf)

	extra := make([]byte, xlen)
	n, err = io.ReadFull(z.r, extra)
	totalRead += n
	if err != nil {
		return totalRead, 0, nil, headerErr(fmt.Errorf("reading EXTRA: %w", err))
	}
	z.digest.Write(extra)

	// NOTE: The EXTRA field could could contain multiple sub-fields.
	var chunkSize int
	var sizes []int

	er := bytes.NewReader(extra)
	var foundRAField bool
	for er.Len() > 0 {
		// Read SI1, SI2, and LEN
		buf = make([]byte, 4)
		_, err = io.ReadFull(er, buf)
		if err != nil {
			return totalRead, 0, nil, headerErr(fmt.Errorf("reading EXTRA: %w", err))
		}

		si1 := buf[0]
		si2 := buf[1]
		extraLen := binary.LittleEndian.Uint16(buf[2:])

		// Read the subfield data.
		extraBuf := make([]byte, extraLen)
		_, err = io.ReadFull(er, extraBuf)
		if err != nil {
			return totalRead, 0, nil, headerErr(fmt.Errorf("reading EXTRA: %w", err))
		}

		// This is the dictzip 'R'andom 'A'ccess data field.
		if si1 == hdrDictzipSI1 && si2 == hdrDictzipSI2 {
			var err error
			chunkSize, sizes, err = readExtraSizes(bytes.NewReader(extraBuf))
			if err != nil {
				return totalRead, 0, nil, err
			}
			foundRAField = true
		} else {
			// Append the non-RA extra data field.
			z.Extra = append(z.Extra, buf...)
			z.Extra = append(z.Extra, extraBuf...)
		}
	}

	if !foundRAField {
		return totalRead, 0, nil, fmt.Errorf("%w: no RA EXTRA field", ErrHeader)
	}

	return totalRead, chunkSize, sizes, nil
}

// readExtraSizes reads the dictzip uncompressed chunk size and compressed
// chunk sizes from the EXTRA field data.
func readExtraSizes(r io.Reader) (int, []int, error) {
	var buf []byte

	// Read VER
	buf = make([]byte, 2)
	_, err := io.ReadFull(r, buf)
	if err != nil {
		return 0, nil, headerErr(fmt.Errorf("VER: %w", err))
	}
	ver := binary.LittleEndian.Uint16(buf)

	if ver != 1 {
		return 0, nil, fmt.Errorf("%w: unsupported version: %d", ErrHeader, ver)
	}

	// Read CHLEN
	buf = make([]byte, 2)
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return 0, nil, headerErr(fmt.Errorf("CHLEN: %w", err))
	}
	chlen := binary.LittleEndian.Uint16(buf)

	// Read CHCNT
	buf = make([]byte, 2)
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return 0, nil, headerErr(fmt.Errorf("CHCNT: %w", err))
	}
	chcnt := binary.LittleEndian.Uint16(buf)

	// Read Sizes
	var sizes []int
	for i := 0; i < int(chcnt); i++ {
		buf = make([]byte, 2)
		_, err = io.ReadFull(r, buf)
		if err != nil {
			return 0, nil, headerErr(fmt.Errorf("chunk sizes: %w", err))
		}
		sizes = append(sizes, int(binary.LittleEndian.Uint16(buf)))
	}

	return int(chlen), sizes, nil
}

// readString reads a null terminated string from z.r.
func (z *Reader) readString() (int64, string, error) {
	var totalRead int64
	var b strings.Builder

	strBuf := make([]byte, 512)
	buf := make([]byte, 1)
	for i := 0; ; i++ {
		if i >= len(strBuf) {
			return totalRead, b.String(), fmt.Errorf("%w: string header len exceeded", ErrHeader)
		}

		n, err := io.ReadFull(z.r, buf)
		totalRead += int64(n)
		if err != nil {
			return totalRead, "", headerErr(fmt.Errorf("string header: %w", err))
		}
		strBuf[i] = buf[0]

		if buf[0] == 0 {
			// NOTE: The CRC digest includes the zero byte null terminator.
			z.digest.Write(strBuf[:i+1])

			// Strings are ISO 8859-1, Latin-1 (RFC 1952, section 2.3.1).
			s := make([]rune, 0, i)
			for _, v := range strBuf[:i] {
				s = append(s, rune(v))
			}
			return totalRead, string(s), nil
		}
	}
}

// readHeader reads the gzip header for dictzip specific headers and returns
// offsets and blocksize used for random access.
func (z *Reader) readHeader() (int64, int, []int64, error) {
	var chunkSize int
	var sizes []int
	var startOffset int64

	n, flg, err := z.readFlg()
	startOffset += int64(n)
	if err != nil {
		return startOffset, 0, nil, err
	}

	if flg&flgEXTRA == 0 {
		return startOffset, 0, nil, fmt.Errorf("%w: no EXTRA field", ErrHeader)
	}

	// Read the EXTRA field
	n, chunkSize, sizes, err = z.readExtra()
	startOffset += int64(n)
	if err != nil {
		return startOffset, 0, nil, err
	}
	z.sizes = sizes

	// Read the NAME field.
	if flg&flgNAME != 0 {
		n, fname, err := z.readString()
		startOffset += n
		if err != nil {
			return startOffset, 0, nil, err
		}
		z.Name = fname
	}

	// Read the COMMENT field.
	if flg&flgCOMMENT != 0 {
		n, fcomment, err := z.readString()
		startOffset += n
		if err != nil {
			return startOffset, 0, nil, err
		}
		z.Comment = fcomment
	}

	// Perform a CRC check.
	if flg&flgCRC != 0 {
		buf := make([]byte, 2)
		n, err := io.ReadFull(z.r, buf)
		startOffset += int64(n)
		if err != nil {
			return startOffset, 0, nil, headerErr(fmt.Errorf("CRC-16: %w", err))
		}
		digest := binary.LittleEndian.Uint16(buf)
		//nolint:gosec // we intentionally take the two lowest order bits of the CRC digest.
		if digest != uint16(z.digest.Sum32()) {
			return startOffset, 0, nil, fmt.Errorf("%w: bad CRC-16 digest", ErrHeader)
		}
	}

	// Calculate the dictzip offsets.
	offsets := make([]int64, len(sizes)+1)
	offsets[0] = startOffset
	for i := 0; i < len(sizes); i++ {
		offsets[i+1] = offsets[i] + int64(sizes[i])
	}

	return startOffset, chunkSize, offsets, nil
}
