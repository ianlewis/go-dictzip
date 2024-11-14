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
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"math"
	"os"
	"time"
)

const (
	// DefaultChunkSize is the default chunk size used when writing dictzip files.
	DefaultChunkSize = math.MaxUint16
)

const (
	// NoCompression performs no compression on the input.
	NoCompression = flate.NoCompression

	// BestSpeed provides the lowest level of compression but the fastest
	// performance.
	BestSpeed = flate.BestSpeed

	// BestCompression provides the highest level of compression but the slowest
	// performance.
	BestCompression = flate.BestCompression

	// DefaultCompression is the default compression level used for compressing
	// chunks. It provides a balance between compression and performance.
	DefaultCompression = flate.DefaultCompression

	// HuffmanOnly disables Lempel-Ziv match searching and only performs Huffman
	// entropy encoding. See [flate.HuffmanOnly].
	HuffmanOnly = flate.HuffmanOnly
)

// Writer implements [io.WriteCloser] for writing dictzip files. Writer writes
// chunks to a temporary file during write and copies the resulting data to the
// final file when [Writer.Close] is called.
//
// For this reason, [Writer.Close] must be called in order to write the file
// correctly.
type Writer struct {
	// Header is written to the file when [Writer.Close] is called.
	Header

	// tmp is the temporary file where chunks will be written.
	tmp *os.File

	// hasData is true if data has been written to the chunk buffer but hasn't
	// been finalized and written to tmp. We need this because we can't simply
	// call z.Flush and check chunkBuf.Len due to the fact that flate.Writer
	// will write sync markers on every call to Flush even if no data has been
	// written.
	hasData bool

	// chunkBuf is the current compressed chunk.
	chunkBuf *bytes.Buffer

	// compressor is the compression writer used to write the current
	// compressed chunk to chunkBuf.
	compressor *flate.Writer

	// w is the io.Writer for the final destination for the compressed file.
	w io.Writer

	// digest is the CRC-32 digest (IEEE polynomial).
	// See RFC-1952 Section 2.3.1.
	digest hash.Hash32

	// isize is the total size of the uncompressed input.
	isize int64

	// level is the compression level being used.
	level int

	// closed indicates the writer has been closed.
	closed bool
}

// NewWriter initializes a new dictzip [Writer] with the default compression
// level and chunk size.
//
// The OS Header is always set to [OSUnknown] (0xff) by default.
func NewWriter(w io.Writer) (*Writer, error) {
	return NewWriterLevel(w, DefaultCompression, DefaultChunkSize)
}

// NewWriterLevel initializes a new dictzip [Writer] with the given compression
// level and chunk size.
//
// The OS Header is always set to [OSUnknown] (0xff) by default.
func NewWriterLevel(w io.Writer, level, chunkSize int) (*Writer, error) {
	tmp, err := os.CreateTemp("", "dictzip.*")
	if err != nil {
		return nil, fmt.Errorf("%w: creating temp file: %w", errDictzip, err)
	}

	var buf bytes.Buffer
	fw, err := flate.NewWriter(&buf, level)
	if err != nil {
		return nil, fmt.Errorf("%w: initializing deflate writer: %w", errDictzip, err)
	}

	digest := crc32.NewIEEE()
	z := Writer{
		Header: Header{
			OS: OSUnknown,
		},
		tmp:        tmp,
		hasData:    false,
		chunkBuf:   &buf,
		compressor: fw,
		w:          w,
		digest:     digest,
		level:      level,
	}
	z.chunkSize = chunkSize

	return &z, nil
}

func (z *Writer) Write(p []byte) (int, error) {
	if z.closed {
		return 0, fmt.Errorf("%w: Write called on closed writer", errDictzip)
	}

	// Write chunks to z.compressor, resetting the Writer, and flushing chunks
	// to the z.tmp as necessary.
	var i int
	for i < len(p) {
		// Get the end index by adding the chunk size minus any already written
		// part of the current chunk.
		j := i + z.chunkSize - int(z.isize%int64(z.chunkSize))
		if j > len(p) {
			j = len(p)
		}

		// Compress the data to chunkBuf.
		n, err := z.compressor.Write(p[i:j])
		z.isize += int64(n)
		if err != nil {
			return i + n, fmt.Errorf("%w: compressing: %w", errDictzip, err)
		}
		// Update the CRC-32 digest.
		_, err = z.digest.Write(p[i : i+n])
		if err != nil {
			return i + n, fmt.Errorf("%w: updating digest: %w", errDictzip, err)
		}
		i += n
		if n > 0 {
			z.hasData = true
		}

		if z.isize%int64(z.chunkSize) == 0 {
			err = z.flushCompressor()
			if err != nil {
				return i, err
			}
		}
	}

	return i, nil
}

// Close closes the writer by writing the header with calculated offsets and
// copying chunks from the temporary file to the final output file.
func (z *Writer) Close() error {
	if z.closed {
		return nil
	}
	z.closed = true
	defer z.tmp.Close()

	// Flush any compressed data chunks to z.tmp.
	if err := z.flushCompressor(); err != nil {
		return err
	}

	// Close the compressor. This will add some trailing markers.
	if err := z.compressor.Close(); err != nil {
		return fmt.Errorf("%w: compressing: %w", errDictzip, err)
	}

	// Write header to z.w
	if err := z.writeHeader(); err != nil {
		return fmt.Errorf("%w: writing header: %w", errDictzip, err)
	}

	// Copy chunks from tmp to z.w
	if err := z.tmp.Sync(); err != nil {
		return fmt.Errorf("%w: sync: %w", errDictzip, err)
	}
	if _, err := z.tmp.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("%w: seek: %w", errDictzip, err)
	}
	if _, err := io.Copy(z.w, z.tmp); err != nil {
		return fmt.Errorf("%w: writing chunks: %w", errDictzip, err)
	}

	// Copy remaining data from chunkBuf to z.w. This is needed to write final
	// deflate markers.
	if _, err := io.Copy(z.w, z.chunkBuf); err != nil {
		return fmt.Errorf("%w: writing final chunk: %w", errDictzip, err)
	}

	// Write the CRC-32 and ISIZE
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint32(buf[0:4], z.digest.Sum32())
	//nolint:gosec // we intentionally take the isize modulo 2^32 per RFC-1952 Section 2.3.1.
	binary.LittleEndian.PutUint32(buf[4:8], uint32(z.isize))
	if _, err := z.w.Write(buf); err != nil {
		return fmt.Errorf("%w: writing CRC-32 and isize: %w", errDictzip, err)
	}

	return nil
}

func (z *Writer) writeHeader() error {
	header := make([]byte, 10)
	header[0] = hdrGzipID1
	header[1] = hdrGzipID2
	header[2] = hdrDeflateCM
	header[3] = flgEXTRA
	if z.Name != "" {
		header[3] |= flgNAME
	}
	if z.Comment != "" {
		header[3] |= flgCOMMENT
	}
	if z.ModTime.After(time.Unix(0, 0)) {
		// Section 2.3.1, the zero value for MTIME means that the
		// modified time is not set.
		// NOTE: since this is a uint32 timestamp, it should work until 2106-02-07.
		//nolint:gosec // We will allow overflow of modtime. It is not a security issue.
		binary.LittleEndian.PutUint32(header[4:8], uint32(z.ModTime.Unix()))
	}
	if z.level == BestCompression {
		header[8] = 2
	} else if z.level == BestSpeed {
		header[8] = 4
	}
	header[9] = z.OS
	if _, err := z.w.Write(header); err != nil {
		return fmt.Errorf("%w: writing header: %w", errDictzip, err)
	}

	if err := z.writeExtra(); err != nil {
		return err
	}

	if z.Name != "" {
		if err := z.writeString(z.Name); err != nil {
			return err
		}
	}

	if z.Comment != "" {
		if err := z.writeString(z.Comment); err != nil {
			return err
		}
	}

	return nil
}

// writeExtra writes the EXTRA header starting with XLEN. The Dictzip random
// access chunk size subfield is included first followed by user-specified
// extra subfields in z.Extra.
func (z *Writer) writeExtra() error {
	// The extra header is written as follows.
	// The RA random access dictzip field is written first.
	// - RA subfield
	//   - SI1 (1 byte) - gzip
	//   - SI2 (1 byte) - gzip
	//   - LEN (2 bytes) - gzip
	//   - VER (2 bytes) - dictzip
	//   - CHLEN (2 bytes) - dictzip
	//   - CHCNT (2 bytes) - dictzip
	//   - Chunk sizes (each 2 bytes).
	// - User-specified z.Extra data.

	// CHLEN
	chlen := z.chunkSize
	if chlen > math.MaxUint16 {
		return fmt.Errorf("%w: CHLEN exceeded: %v", ErrHeader, chlen)
	}

	// CHCNT
	chcnt := len(z.sizes)
	if chcnt > math.MaxUint16 {
		return fmt.Errorf("%w: CHCNT exceeded: %v", ErrHeader, chcnt)
	}

	// LEN field (includes VER, CHLEN, CHCNT, chunk sizes)
	raLen := 6 + (chcnt * 2)

	// XLEN (includes SI1, SI2, LEN, RA subfield, user-specified extra subfields)
	xlen := 4 + raLen + len(z.Extra)
	if xlen > math.MaxUint16 {
		return fmt.Errorf("%w: XLEN exceeded: %v", ErrHeader, xlen)
	}

	// NOTE: Include 2 extra bytes for xlen itself.
	extra := make([]byte, 2+xlen)

	// Set XLEN (length of extra excluding XLEN)
	//nolint:gosec // xlen max value is checked above.
	binary.LittleEndian.PutUint16(extra[0:2], uint16(xlen))

	// Write the RA subfield.
	extra[2] = hdrDictzipSI1
	extra[3] = hdrDictzipSI2
	//nolint:gosec // raLen max value is checked above.
	binary.LittleEndian.PutUint16(extra[4:6], uint16(raLen)) // LEN
	binary.LittleEndian.PutUint16(extra[6:8], uint16(1))     // VER
	//nolint:gosec // chlen max value is checked above.
	binary.LittleEndian.PutUint16(extra[8:10], uint16(chlen))
	// NOTE: chcnt max value is checked above. gosec doesn't seem to care about this.
	binary.LittleEndian.PutUint16(extra[10:12], uint16(chcnt))

	i := 12
	for _, chSize := range z.sizes {
		if chSize > math.MaxUint16 {
			return fmt.Errorf("%w: chunk size exceeded: %v", ErrHeader, chSize)
		}
		//nolint:gosec // chSize max value is checked above.
		binary.LittleEndian.PutUint16(extra[i:i+2], uint16(chSize))
		i += 2
	}

	// Set the user specified extra data.
	_ = copy(extra[i:], z.Extra)

	_, err := z.w.Write(extra)
	if err != nil {
		return fmt.Errorf("%w: writing EXTRA: %w", errDictzip, err)
	}
	return nil
}

func (z *Writer) flushCompressor() error {
	if z.hasData {
		// NOTE: we need to flush the flate writer to make sure it has
		// written all compressed data to chunkBuf.
		if err := z.compressor.Flush(); err != nil {
			return fmt.Errorf("%w: compressing: %w", errDictzip, err)
		}

		// Append the compressed chunk's length to the sizes.
		z.sizes = append(z.sizes, z.chunkBuf.Len())

		// Copy chunkBuf to tmp.
		if _, err := io.Copy(z.tmp, z.chunkBuf); err != nil {
			return fmt.Errorf("%w: compressing: %w", errDictzip, err)
		}

		// Reset the chunk buffer and flate writer.
		z.chunkBuf.Reset()
		z.compressor.Reset(z.chunkBuf)
		z.hasData = false
	}

	return nil
}

// writeString writes a string header value to z.w. The string
// is encoded in ISO 8859-1, Latin-1 and terminated with a zero byte.
func (z *Writer) writeString(s string) error {
	// Strings are ISO 8859-1, Latin-1 (RFC 1952, section 2.3.1).
	b := make([]byte, 0, len(s))
	for _, r := range s {
		if r == 0 || r > 0xff {
			return fmt.Errorf("%w: non-Latin-1 header string", ErrHeader)
		}
		b = append(b, byte(r))
	}
	// strings are terminated by a zero byte.
	b = append(b, byte(0))
	_, err := z.w.Write(b)
	if err != nil {
		return fmt.Errorf("%w: writing string header: %w", errDictzip, err)
	}
	return nil
}
