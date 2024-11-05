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
