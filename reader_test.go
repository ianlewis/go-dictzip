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
	"io"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestReader(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		data []byte

		fname     string
		fcomment  string
		os        byte
		extra     []byte
		chunkSize int
		offsets   []int64
		bytes     []byte
		newErr    error
		readErr   error
	}{
		{
			// NOTE: Curiously, dictzip will compress an empty file
			//       but dictunzip will throw an error for empty files.
			name: "empty file",
			data: []byte{
				// Header
				hdrGzipID1,
				hdrGzipID2,
				hdrDeflateCM,
				flgEXTRA | flgNAME,     // FLG
				0x00, 0x00, 0x00, 0x00, // MTIME
				0x2, // XFL
				0x3, // OS

				// EXTRA
				0xa, 0x0, // XLEN // 10
				0x52, 0x41, // 'R', 'A'
				0x6, 0x0, // LEN // 6
				0x1, 0x0, // VER // 1
				0xcb, 0xe3, // CHLEN // 58315
				0x0, 0x0, // CHCNT // 0

				// NAME // empty.txt
				0x65, 0x6d, 0x70, 0x74, 0x79, 0x2e, 0x74, 0x78, 0x74, 0x0,

				0x3, 0x0, 0x0, // Empty deflate data.

				0x0, 0x0, 0x0, 0x0, // CRC32
				0x0, 0x0, 0x0, 0x0, // ISIZE
			},

			fname: "empty.txt",
			bytes: []byte{},
			os:    0x3,
			extra: []byte{
				0x52, 0x41, // 'R', 'A'
				0x6, 0x0, // LEN // 6
				0x1, 0x0, // VER // 1
				0xcb, 0xe3, // CHLEN // 58315
				0x0, 0x0, // CHCNT // 0
			},
			chunkSize: 58315,
			offsets:   []int64{32},
		},
		{
			name: "fcomment",
			data: []byte{
				// Header
				hdrGzipID1,
				hdrGzipID2,
				hdrDeflateCM,
				flgEXTRA | flgCOMMENT,  // FLG
				0x67, 0x31, 0x2f, 0x67, // MTIME
				0x2, // XFL
				0x3, // OS

				// EXTRA
				0xa, 0x0, // XLEN = 10
				0x52, 0x41, // 'R', 'A'
				0x6, 0x0, // LEN = 6
				0x1, 0x0, // VER = 1
				0xcb, 0xe3, // CHLEN = 58315
				0x0, 0x0, // CHCNT = 0

				// COMMENT // fcomment.txt
				0x66, 0x63, 0x6f, 0x6d, 0x6d, 0x65, 0x6e, 0x74, 0x2e, 0x74, 0x78, 0x74, 0x0,

				0x3, 0x0, 0x0, // Empty deflate data.

				0x0, 0x0, 0x0, 0x0, // CRC32
				0x0, 0x0, 0x0, 0x0, // ISIZE
			},
			fcomment: "fcomment.txt",
			bytes:    []byte{},
			os:       0x3,
			extra: []byte{
				0x52, 0x41, // 'R', 'A'
				0x6, 0x0, // LEN = 6
				0x1, 0x0, // VER = 1
				0xcb, 0xe3, // CHLEN = 58315
				0x0, 0x0, // CHCNT = 0
			},
			chunkSize: 58315,
			offsets:   []int64{35},
		},
		{
			name: "with crc16",
			data: []byte{
				// Header
				hdrGzipID1,
				hdrGzipID2,
				hdrDeflateCM,
				flgEXTRA | flgCRC,      // FLG
				0x00, 0x00, 0x00, 0x00, // MTIME
				0x2, // XFL
				0x3, // OS

				// EXTRA
				0xa, 0x0, // XLEN // 10
				0x52, 0x41, // 'R', 'A'
				0x6, 0x0, // LEN // 6
				0x1, 0x0, // VER // 1
				0xcb, 0xe3, // CHLEN // 58315
				0x0, 0x0, // CHCNT // 0

				0xe3, 0xb2, // CRC16

				0x3, 0x0, 0x0, // Empty deflate data.

				0x0, 0x0, 0x0, 0x0, // CRC32
				0x0, 0x0, 0x0, 0x0, // ISIZE
			},
			bytes: []byte{},
			os:    0x3,
			extra: []byte{
				0x52, 0x41, // 'R', 'A'
				0x6, 0x0, // LEN // 6
				0x1, 0x0, // VER // 1
				0xcb, 0xe3, // CHLEN // 58315
				0x0, 0x0, // CHCNT // 0
			},
			chunkSize: 58315,
			offsets:   []int64{24},
		},
		{
			name: "bad crc16",
			data: []byte{
				// Header
				hdrGzipID1,
				hdrGzipID2,
				hdrDeflateCM,
				flgEXTRA | flgCRC,      // FLG
				0x00, 0x00, 0x00, 0x00, // MTIME
				0x2, // XFL
				0x3, // OS

				// EXTRA
				0xa, 0x0, // XLEN // 10
				0x52, 0x41, // 'R', 'A'
				0x6, 0x0, // LEN // 6
				0x1, 0x0, // VER // 1
				0xcb, 0xe3, // CHLEN // 58315
				0x0, 0x0, // CHCNT // 0

				0x00, 0x00, // CRC16

				0x3, 0x0, 0x0, // Empty deflate data.

				0x0, 0x0, 0x0, 0x0, // CRC32
				0x0, 0x0, 0x0, 0x0, // ISIZE
			},
			bytes:  []byte{},
			newErr: ErrHeader,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			z, err := NewReader(bytes.NewReader(tc.data))
			if diff := cmp.Diff(tc.newErr, err, cmpopts.EquateErrors()); diff != "" {
				t.Fatalf("NewReader (-want, +got):\n%s", diff)
			}
			// Skip other tests in the event of an error.
			if err != nil {
				return
			}
			defer z.Close()

			if diff := cmp.Diff(tc.fname, z.Name); diff != "" {
				t.Errorf("Name (-want, +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.fcomment, z.Comment); diff != "" {
				t.Errorf("Name (-want, +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.os, z.OS); diff != "" {
				t.Errorf("OS (-want, +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.extra, z.Extra); diff != "" {
				t.Errorf("OS (-want, +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.chunkSize, z.ChunkSize()); diff != "" {
				t.Errorf("ChunkSize (-want, +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.offsets, z.offsets); diff != "" {
				t.Errorf("Offsets (-want, +got):\n%s", diff)
			}

			b, err := io.ReadAll(z)
			if diff := cmp.Diff(tc.readErr, err, cmpopts.EquateErrors()); diff != "" {
				t.Fatalf("ReadAll (-want, +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.bytes, b); diff != "" {
				t.Errorf("ReadAll (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestReader_Read(t *testing.T) {
	t.Parallel()

	f, err := os.Open("internal/testdata/test.txt.dz")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	r, err := NewReader(f)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer r.Close()

	// r.offset should be at the beginning of the file.
	if diff := cmp.Diff(int64(0), r.offset); diff != "" {
		t.Fatalf("r.offset (-want, +got):\n%s", diff)
	}

	buf := make([]byte, 10)
	n, err := r.Read(buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if diff := cmp.Diff(10, n); diff != "" {
		t.Errorf("Read (-want, +got):\n%s", diff)
	}

	if diff := cmp.Diff([]byte("0123456789"), buf); diff != "" {
		t.Errorf("Read (-want, +got):\n%s", diff)
	}

	// r.offset should should be advanced by the # of read bytes.
	if diff := cmp.Diff(int64(10), r.offset); diff != "" {
		t.Errorf("r.offset (-want, +got):\n%s", diff)
	}

	buf2 := make([]byte, 15)
	n, err = r.Read(buf2)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if diff := cmp.Diff(15, n); diff != "" {
		t.Errorf("Read (-want, +got):\n%s", diff)
	}

	if diff := cmp.Diff([]byte("abcdefghijklmno"), buf2); diff != "" {
		t.Errorf("Read (-want, +got):\n%s", diff)
	}

	// r.offset should should be advanced by the # of read bytes.
	if diff := cmp.Diff(int64(25), r.offset); diff != "" {
		t.Errorf("r.offset (-want, +got):\n%s", diff)
	}
}

func TestReader_Seek(t *testing.T) {
	t.Parallel()

	f, err := os.Open("internal/testdata/test.txt.dz")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	r, err := NewReader(f)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer r.Close()

	// r.offset should be at the beginning of the file.
	if diff := cmp.Diff(int64(0), r.offset); diff != "" {
		t.Fatalf("r.offset (-want, +got):\n%s", diff)
	}

	off, err := r.Seek(25, io.SeekStart)
	if err != nil {
		t.Fatalf("Seek: %v", err)
	}

	if diff := cmp.Diff(int64(25), off); diff != "" {
		t.Errorf("Seek (-want, +got):\n%s", diff)
	}

	if diff := cmp.Diff(int64(25), r.offset); diff != "" {
		t.Errorf("r.offset (-want, +got):\n%s", diff)
	}

	// Negative value
	off, err = r.Seek(-12, io.SeekCurrent)
	if err != nil {
		t.Fatalf("Seek: %v", err)
	}

	if diff := cmp.Diff(int64(13), off); diff != "" {
		t.Errorf("Seek (-want, +got):\n%s", diff)
	}

	if diff := cmp.Diff(int64(13), r.offset); diff != "" {
		t.Errorf("r.offset (-want, +got):\n%s", diff)
	}

	// Positive value
	off, err = r.Seek(16, io.SeekCurrent)
	if err != nil {
		t.Fatalf("Seek: %v", err)
	}

	if diff := cmp.Diff(int64(29), off); diff != "" {
		t.Errorf("Seek (-want, +got):\n%s", diff)
	}

	if diff := cmp.Diff(int64(29), r.offset); diff != "" {
		t.Errorf("r.offset (-want, +got):\n%s", diff)
	}
}

func TestReader_Seek_SeekStart_negative(t *testing.T) {
	t.Parallel()

	f, err := os.Open("internal/testdata/test.txt.dz")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	r, err := NewReader(f)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer r.Close()

	// r.offset should be at the beginning of the file.
	if diff := cmp.Diff(int64(0), r.offset); diff != "" {
		t.Fatalf("r.offset (-want, +got):\n%s", diff)
	}

	off, err := r.Seek(-25, io.SeekStart)
	if diff := cmp.Diff(int64(0), off); diff != "" {
		t.Errorf("Seek (-want, +got):\n%s", diff)
	}

	if diff := cmp.Diff(int64(0), r.offset); diff != "" {
		t.Errorf("r.offset (-want, +got):\n%s", diff)
	}

	if diff := cmp.Diff(errNegativeOffset, err, cmpopts.EquateErrors()); diff != "" {
		t.Errorf("Seek (-want, +got):\n%s", diff)
	}
}

func TestReader_Seek_SeekCurrent_negative(t *testing.T) {
	t.Parallel()

	f, err := os.Open("internal/testdata/test.txt.dz")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	r, err := NewReader(f)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer r.Close()

	// r.offset should be at the beginning of the file.
	if diff := cmp.Diff(int64(0), r.offset); diff != "" {
		t.Fatalf("r.offset (-want, +got):\n%s", diff)
	}

	off, err := r.Seek(-25, io.SeekCurrent)
	if diff := cmp.Diff(int64(0), off); diff != "" {
		t.Errorf("Seek (-want, +got):\n%s", diff)
	}

	if diff := cmp.Diff(int64(0), r.offset); diff != "" {
		t.Errorf("r.offset (-want, +got):\n%s", diff)
	}

	if diff := cmp.Diff(errNegativeOffset, err, cmpopts.EquateErrors()); diff != "" {
		t.Errorf("Seek (-want, +got):\n%s", diff)
	}
}

func TestReader_Seek_SeekEnd(t *testing.T) {
	t.Parallel()

	f, err := os.Open("internal/testdata/test.txt.dz")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	r, err := NewReader(f)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer r.Close()

	// r.offset should be at the beginning of the file.
	if diff := cmp.Diff(int64(0), r.offset); diff != "" {
		t.Fatalf("r.offset (-want, +got):\n%s", diff)
	}

	// SeekEnd
	off, err := r.Seek(22, io.SeekEnd)
	if diff := cmp.Diff(int64(0), off); diff != "" {
		t.Errorf("Seek (-want, +got):\n%s", diff)
	}

	if diff := cmp.Diff(int64(0), r.offset); diff != "" {
		t.Errorf("r.offset (-want, +got):\n%s", diff)
	}

	if diff := cmp.Diff(errUnsupportedSeek, err, cmpopts.EquateErrors()); diff != "" {
		t.Errorf("Seek (-want, +got):\n%s", diff)
	}
}

func TestReader_ReadAt(t *testing.T) {
	t.Parallel()

	f, err := os.Open("internal/testdata/test.txt.dz")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	r, err := NewReader(f)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer r.Close()

	// r.offset should be at the beginning of the file.
	if diff := cmp.Diff(int64(0), r.offset); diff != "" {
		t.Fatalf("r.offset (-want, +got):\n%s", diff)
	}

	buf := make([]byte, 10)
	n, err := r.ReadAt(buf, 124)
	if err != nil {
		t.Fatalf("ReadAt: %v", err)
	}

	if diff := cmp.Diff(10, n); diff != "" {
		t.Fatalf("ReadAt (-want, +got):\n%s", diff)
	}

	if diff := cmp.Diff([]byte("0123456789"), buf); diff != "" {
		t.Fatalf("ReadAt (-want, +got):\n%s", diff)
	}

	// r.offset should still be at the beginning of the file.
	if diff := cmp.Diff(int64(0), r.offset); diff != "" {
		t.Fatalf("r.offset (-want, +got):\n%s", diff)
	}
}
