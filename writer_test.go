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
	"compress/gzip"
	"io"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func verifyGzip(t *testing.T, compressed *bytes.Buffer, writes [][]byte) {
	t.Helper()

	// Verify that the output is gzip compatible.
	gr, err := gzip.NewReader(compressed)
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	rb, err := io.ReadAll(gr)
	if err != nil {
		t.Fatalf("io.ReadAll: %v", err)
	}

	// Flatten the input writes
	flattened := []byte{}
	for _, b := range writes {
		flattened = append(flattened, b...)
	}

	if diff := cmp.Diff(flattened, rb); diff != "" {
		t.Errorf("gzip.Read (-want, +got):\n%s", diff)
	}
}

func TestWriter(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		fname     string
		fcomment  string
		modtime   time.Time
		os        byte
		extra     []byte
		level     int
		chunkSize int
		// data is uncompressed data to write. Each entry in the slice causes
		// one call to Write.
		data [][]byte

		bytes    []byte // final compressed file contents.
		newErr   error
		writeErr error
	}{
		{
			// NOTE: Curiously, dictzip will compress an empty file
			//       but dictunzip will throw an error for empty files.
			name: "empty file with name",

			fname:     "empty.txt",
			os:        OSUnknown,
			chunkSize: DefaultChunkSize,
			level:     DefaultCompression,
			data:      nil,

			bytes: []byte{
				// Header
				hdrGzipID1,
				hdrGzipID2,
				hdrDeflateCM,
				flgEXTRA | flgNAME,     // FLG
				0x00, 0x00, 0x00, 0x00, // MTIME
				0x0,       // XFL
				OSUnknown, // OS

				// EXTRA
				0xa, 0x0, // XLEN // 10
				0x52, 0x41, // 'R', 'A'
				0x6, 0x0, // LEN // 6
				0x1, 0x0, // VER // 1
				0xff, 0xff, // CHLEN // 65535
				0x0, 0x0, // CHCNT // 0

				// NAME // empty.txt
				0x65, 0x6d, 0x70, 0x74, 0x79, 0x2e, 0x74, 0x78, 0x74, 0x0,

				0x01, 0x00, 0x00, 0xff, 0xff, // Empty deflate data (sync/end marker)

				0x0, 0x0, 0x0, 0x0, // CRC32
				0x0, 0x0, 0x0, 0x0, // ISIZE
			},
		},
		{
			// NOTE: Curiously, dictzip will compress an empty file
			//       but dictunzip will throw an error for empty files.
			name: "empty file with comment",

			fcomment:  "fcomment.txt",
			os:        OSUnknown,
			chunkSize: DefaultChunkSize,
			level:     DefaultCompression,
			data:      nil,

			bytes: []byte{
				// Header
				hdrGzipID1,
				hdrGzipID2,
				hdrDeflateCM,
				flgEXTRA | flgCOMMENT,  // FLG
				0x00, 0x00, 0x00, 0x00, // MTIME
				0x0,       // XFL
				OSUnknown, // OS

				// EXTRA
				0xa, 0x0, // XLEN // 10
				0x52, 0x41, // 'R', 'A'
				0x6, 0x0, // LEN // 6
				0x1, 0x0, // VER // 1
				0xff, 0xff, // CHLEN // 65535
				0x0, 0x0, // CHCNT // 0

				// COMMENT // fcomment.txt
				0x66, 0x63, 0x6f, 0x6d, 0x6d, 0x65, 0x6e, 0x74, 0x2e, 0x74, 0x78, 0x74, 0x0,

				0x01, 0x00, 0x00, 0xff, 0xff, // Empty deflate data (sync/end marker)

				0x0, 0x0, 0x0, 0x0, // CRC32
				0x0, 0x0, 0x0, 0x0, // ISIZE
			},
		},
		{
			// NOTE: Curiously, dictzip will compress an empty file
			//       but dictunzip will throw an error for empty files.
			name: "empty file with extra",

			os: OSUnknown,
			extra: []byte{
				'A', 'Z', // SI
				0x3, 0x0, // LEN
				0xab, 0xcd, 0xef,
			},
			chunkSize: DefaultChunkSize,
			level:     DefaultCompression,
			data:      nil,

			bytes: []byte{
				// Header
				hdrGzipID1,
				hdrGzipID2,
				hdrDeflateCM,
				flgEXTRA,               // FLG
				0x00, 0x00, 0x00, 0x00, // MTIME
				0x0,       // XFL
				OSUnknown, // OS

				// EXTRA
				0x11, 0x0, // XLEN // 17
				0x52, 0x41, // 'R', 'A'
				0x6, 0x0, // LEN // 6
				0x1, 0x0, // VER // 1
				0xff, 0xff, // CHLEN // 65535
				0x0, 0x0, // CHCNT // 0

				// User-specified EXTRA sub-field.
				'A', 'Z', // SI
				0x3, 0x0, // LEN
				0xab, 0xcd, 0xef,

				0x01, 0x00, 0x00, 0xff, 0xff, // Empty deflate data (sync/end marker)

				0x0, 0x0, 0x0, 0x0, // CRC32
				0x0, 0x0, 0x0, 0x0, // ISIZE
			},
		},
		{
			// NOTE: Curiously, dictzip will compress an empty file
			//       but dictunzip will throw an error for empty files.
			name: "empty file with modtime",

			os:        OSUnknown,
			modtime:   time.Date(1981, 10, 29, 10, 1, 0, 0, time.UTC),
			chunkSize: DefaultChunkSize,
			level:     DefaultCompression,
			data:      nil,

			bytes: []byte{
				// Header
				hdrGzipID1,
				hdrGzipID2,
				hdrDeflateCM,
				flgEXTRA,               // FLG
				0x5c, 0x8b, 0x3e, 0x16, // MTIME // 373197660
				0x0,       // XFL
				OSUnknown, // OS

				// EXTRA
				0xa, 0x0, // XLEN // 10
				0x52, 0x41, // 'R', 'A'
				0x6, 0x0, // LEN // 6
				0x1, 0x0, // VER // 1
				0xff, 0xff, // CHLEN // 65535
				0x0, 0x0, // CHCNT // 0

				0x01, 0x00, 0x00, 0xff, 0xff, // Empty deflate data (sync/end marker)

				0x0, 0x0, 0x0, 0x0, // CRC32
				0x0, 0x0, 0x0, 0x0, // ISIZE
			},
		},
		{
			// NOTE: Curiously, dictzip will compress an empty file
			//       but dictunzip will throw an error for empty files.
			name: "empty file with os",

			os:        OSUnix,
			chunkSize: DefaultChunkSize,
			level:     DefaultCompression,
			data:      nil,

			bytes: []byte{
				// Header
				hdrGzipID1,
				hdrGzipID2,
				hdrDeflateCM,
				flgEXTRA,               // FLG
				0x00, 0x00, 0x00, 0x00, // MTIME
				0x0,    // XFL
				OSUnix, // OS

				// EXTRA
				0xa, 0x0, // XLEN // 10
				0x52, 0x41, // 'R', 'A'
				0x6, 0x0, // LEN // 6
				0x1, 0x0, // VER // 1
				0xff, 0xff, // CHLEN // 65535
				0x0, 0x0, // CHCNT // 0

				0x01, 0x00, 0x00, 0xff, 0xff, // Empty deflate data (sync/end marker)

				0x0, 0x0, 0x0, 0x0, // CRC32
				0x0, 0x0, 0x0, 0x0, // ISIZE
			},
		},
		{
			// NOTE: Curiously, dictzip will compress an empty file
			//       but dictunzip will throw an error for empty files.
			name: "empty file with xfl",

			os:        OSUnknown,
			chunkSize: DefaultChunkSize,
			level:     BestSpeed,
			data:      nil,

			bytes: []byte{
				// Header
				hdrGzipID1,
				hdrGzipID2,
				hdrDeflateCM,
				flgEXTRA,               // FLG
				0x00, 0x00, 0x00, 0x00, // MTIME
				XFLFastest, // XFL
				OSUnknown,  // OS

				// EXTRA
				0xa, 0x0, // XLEN // 10
				0x52, 0x41, // 'R', 'A'
				0x6, 0x0, // LEN // 6
				0x1, 0x0, // VER // 1
				0xff, 0xff, // CHLEN // 65535
				0x0, 0x0, // CHCNT // 0

				0x01, 0x00, 0x00, 0xff, 0xff, // Empty deflate data (sync/end marker)

				0x0, 0x0, 0x0, 0x0, // CRC32
				0x0, 0x0, 0x0, 0x0, // ISIZE
			},
		},
		{
			name: "single chunk single write",

			os:        OSUnknown,
			chunkSize: DefaultChunkSize,
			level:     DefaultCompression,
			data: [][]byte{
				[]byte("foo bar baz"),
			},

			bytes: []byte{
				// Header
				hdrGzipID1,
				hdrGzipID2,
				hdrDeflateCM,
				flgEXTRA,               // FLG
				0x00, 0x00, 0x00, 0x00, // MTIME
				0x0,       // XFL
				OSUnknown, // OS

				// EXTRA
				0xc, 0x0, // XLEN // 12
				0x52, 0x41, // 'R', 'A'
				0x8, 0x0, // LEN // 8
				0x1, 0x0, // VER // 1
				0xff, 0xff, // CHLEN // 65535
				0x1, 0x0, // CHCNT // 1

				// Chunk sizes.
				0x11, 0x0, // 17

				// 17 byte chunk of compressed data
				0x4a, 0xcb, 0xcf, 0x57, 0x48, 0x4a, 0x2c, 0x52,
				0x48, 0x4a, 0xac, 0x02, 0x00, 0x00, 0x00, 0xff, 0xff,

				0x01, 0x00, 0x00, 0xff, 0xff, // sync/end marker.

				0x61, 0xde, 0x62, 0xf2, // CRC-32
				0x0b, 0x00, 0x00, 0x00, // ISIZE // 11 (len of "foo bar baz")
			},
		},
		{
			name: "multi-chunk single write exact",

			os:        OSUnknown,
			chunkSize: 6,
			level:     DefaultCompression,
			data: [][]byte{
				[]byte("chunk1chunk2chunk3chunk4"),
			},

			bytes: []byte{
				// Header
				hdrGzipID1,
				hdrGzipID2,
				hdrDeflateCM,
				flgEXTRA,               // FLG
				0x00, 0x00, 0x00, 0x00, // MTIME
				0x0,       // XFL
				OSUnknown, // OS

				// EXTRA
				0x12, 0x0, // XLEN // 18
				0x52, 0x41, // 'R', 'A'
				0xe, 0x0, // LEN // 14
				0x1, 0x0, // VER // 1
				0x6, 0x0, // CHLEN // 6
				0x4, 0x0, // CHCNT // 4

				// Chunk sizes.
				0xc, 0x0, // 12
				0xc, 0x0, // 12
				0xc, 0x0, // 12
				0xc, 0x0, // 12

				// compressed data (4 chunks of 12 bytes each).
				0x4a, 0xce, 0x28, 0xcd, 0xcb, 0x36, 0x04, 0x00, 0x00, 0x00, 0xff, 0xff,
				0x4a, 0xce, 0x28, 0xcd, 0xcb, 0x36, 0x02, 0x00, 0x00, 0x00, 0xff, 0xff,
				0x4a, 0xce, 0x28, 0xcd, 0xcb, 0x36, 0x06, 0x00, 0x00, 0x00, 0xff, 0xff,
				0x4a, 0xce, 0x28, 0xcd, 0xcb, 0x36, 0x01, 0x00, 0x00, 0x00, 0xff, 0xff,

				0x01, 0x00, 0x00, 0xff, 0xff, // sync/end marker.

				0x85, 0x42, 0x75, 0x46, // CRC-32
				0x18, 0x00, 0x00, 0x00, // ISIZE // 24 (len of data)
			},
		},
		{
			name: "multi-chunk single write non-exact",

			os:        OSUnknown,
			chunkSize: 6,
			level:     DefaultCompression,
			data: [][]byte{
				[]byte("chunk1chunk2chunk3last"),
			},

			bytes: []byte{
				// Header
				hdrGzipID1,
				hdrGzipID2,
				hdrDeflateCM,
				flgEXTRA,               // FLG
				0x00, 0x00, 0x00, 0x00, // MTIME
				0x0,       // XFL
				OSUnknown, // OS

				// EXTRA
				0x12, 0x0, // XLEN // 18
				0x52, 0x41, // 'R', 'A'
				0xe, 0x0, // LEN // 14
				0x1, 0x0, // VER // 1
				0x6, 0x0, // CHLEN // 6
				0x4, 0x0, // CHCNT // 4

				// Chunk sizes.
				0xc, 0x0, // 12
				0xc, 0x0, // 12
				0xc, 0x0, // 12
				0x0a, 0x00, // 10

				// compressed data (three 12 byte chunks and one 10 byte chunk).
				0x4a, 0xce, 0x28, 0xcd, 0xcb, 0x36, 0x04, 0x00, 0x00, 0x00, 0xff, 0xff,
				0x4a, 0xce, 0x28, 0xcd, 0xcb, 0x36, 0x02, 0x00, 0x00, 0x00, 0xff, 0xff,
				0x4a, 0xce, 0x28, 0xcd, 0xcb, 0x36, 0x06, 0x00, 0x00, 0x00, 0xff, 0xff,
				0xca, 0x49, 0x2c, 0x2e, 0x01, 0x00, 0x00, 0x00, 0xff, 0xff,

				0x01, 0x00, 0x00, 0xff, 0xff, // sync/end marker.

				0x70, 0xee, 0xf4, 0x09, // CRC-32
				0x16, 0x00, 0x00, 0x00, // ISIZE // 22 (len of data)
			},
		},
		{
			name: "multi-chunk multi-write",

			os:        OSUnknown,
			chunkSize: 6,
			level:     DefaultCompression,
			data: [][]byte{
				[]byte("chun"),
				[]byte("k1chunk"),
				[]byte("2chunk3"),
				[]byte("la"),
				[]byte("st"),
			},

			bytes: []byte{
				// Header
				hdrGzipID1,
				hdrGzipID2,
				hdrDeflateCM,
				flgEXTRA,               // FLG
				0x00, 0x00, 0x00, 0x00, // MTIME
				0x0,       // XFL
				OSUnknown, // OS

				// EXTRA
				0x12, 0x0, // XLEN // 18
				0x52, 0x41, // 'R', 'A'
				0xe, 0x0, // LEN // 14
				0x1, 0x0, // VER // 1
				0x6, 0x0, // CHLEN // 65535
				0x4, 0x0, // CHCNT // 4

				// Chunk sizes.
				0xc, 0x0, // 12
				0xc, 0x0, // 12
				0xc, 0x0, // 12
				0x0a, 0x00, // 10

				// compressed data (three 12 byte chunks and one 10 byte chunk).
				0x4a, 0xce, 0x28, 0xcd, 0xcb, 0x36, 0x04, 0x00, 0x00, 0x00, 0xff, 0xff,
				0x4a, 0xce, 0x28, 0xcd, 0xcb, 0x36, 0x02, 0x00, 0x00, 0x00, 0xff, 0xff,
				0x4a, 0xce, 0x28, 0xcd, 0xcb, 0x36, 0x06, 0x00, 0x00, 0x00, 0xff, 0xff,
				0xca, 0x49, 0x2c, 0x2e, 0x01, 0x00, 0x00, 0x00, 0xff, 0xff,

				0x01, 0x00, 0x00, 0xff, 0xff, // sync/end marker.

				0x70, 0xee, 0xf4, 0x09, // CRC-32
				0x16, 0x00, 0x00, 0x00, // ISIZE // 22 (len of data)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer

			z, err := NewWriterLevel(&buf, tc.level, tc.chunkSize)
			if diff := cmp.Diff(tc.newErr, err, cmpopts.EquateErrors()); diff != "" {
				t.Fatalf("NewWriter (-want, +got):\n%s", diff)
			}
			// Skip other tests in the event of an error.
			if err != nil {
				return
			}
			defer func() {
				if err := z.Close(); err != nil {
					t.Fatalf("Close: %v", err)
				}
			}()

			z.Name = tc.fname
			z.Comment = tc.fcomment
			z.ModTime = tc.modtime
			z.OS = tc.os
			z.Extra = tc.extra

			for _, data := range tc.data {
				n, err := z.Write(data)
				if diff := cmp.Diff(tc.writeErr, err, cmpopts.EquateErrors()); diff != "" {
					t.Fatalf("Write (-want, +got):\n%s", diff)
				}
				// Skip other tests in the event of an error.
				if err != nil {
					return
				}

				if diff := cmp.Diff(len(data), n); diff != "" {
					t.Errorf("Write (-want, +got):\n%s", diff)
				}
			}

			// Close the writer
			if diff := cmp.Diff(nil, z.Close(), cmpopts.EquateErrors()); diff != "" {
				t.Fatalf("z.Close (-want, +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.bytes, buf.Bytes()); diff != "" {
				t.Errorf("data (-want, +got):\n%s", diff)
			}

			verifyGzip(t, &buf, tc.data)
		})
	}
}
