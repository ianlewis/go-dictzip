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

func TestReader_readFlg(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		header       []byte
		expectedRead int
		expectedFlg  byte
		expectedErr  error
	}{
		{
			name: "EXTRA",
			header: []byte{
				hdrGzipID[0],
				hdrGzipID[1],
				hdrDeflateCM,
				flgEXTRA,
				0x00,
				0x00,
				0x00,
				0x00,
				0x00,
				0x00,
			},
			expectedRead: 10,
			expectedFlg:  flgEXTRA,
		},
		{
			name: "invalid header",
			header: []byte{
				hdrGzipID[0],
				0x01, // NOTE: Invalid Header
				hdrDeflateCM,
				flgEXTRA,
				0x00,
				0x00,
				0x00,
				0x00,
				0x00,
				0x00,
			},
			expectedErr:  errInvalidIDHeader,
			expectedRead: 10,
			expectedFlg:  flgEXTRA,
		},
		{
			name: "unsupported cm",
			header: []byte{
				hdrGzipID[0],
				hdrGzipID[1],
				0x07, // NOTE: Unsupported compression method
				flgEXTRA,
				0x00,
				0x00,
				0x00,
				0x00,
				0x00,
				0x00,
			},
			expectedErr:  errUnsupportedCompression,
			expectedRead: 10,
			expectedFlg:  flgEXTRA,
		},
		{
			name: "unexpected EOF",
			header: []byte{
				hdrGzipID[0],
				hdrGzipID[1],
				hdrDeflateCM,
				flgEXTRA,
				0x00,
				0x00,
				0x00,
				0x00,
				0x00,
				// NOTE: too few bytes
			},
			expectedErr:  io.ErrUnexpectedEOF,
			expectedRead: 9,
			expectedFlg:  0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			n, flg, err := readFlg(bytes.NewReader(tc.header))

			if diff := cmp.Diff(tc.expectedErr, err, cmpopts.EquateErrors()); diff != "" {
				t.Fatalf("readFlg (-want, +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.expectedRead, n); diff != "" {
				t.Errorf("readFlg read (-want, +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.expectedFlg, flg); diff != "" {
				t.Errorf("readFlg flag (-want, +got):\n%s", diff)
			}
		})
	}
}
