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

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ianlewis/go-dictzip"
)

type decompress struct {
	path    string
	force   bool
	keep    bool
	stdout  bool
	verbose bool
}

var errTruncate = fmt.Errorf("%w: cannot truncate filename", ErrDictzip)

func (d *decompress) Run() error {
	newPath := strings.TrimRight(d.path, filepath.Ext(d.path))
	if newPath == d.path {
		return fmt.Errorf("%w: %q", errTruncate, d.path)
	}

	from, err := os.Open(d.path)
	if err != nil {
		return fmt.Errorf("%w: opening file: %w", ErrDictzip, err)
	}
	defer from.Close()

	flags := os.O_CREATE | os.O_WRONLY
	if !d.force {
		// Do not overwrite existing files unless --force is specified.
		flags |= os.O_EXCL
	}

	var dst io.WriteCloser

	if d.stdout {
		dst = os.Stdout
	} else {
		dst, err = os.OpenFile(newPath, flags, 0o644)
		if err != nil {
			return fmt.Errorf("%w: opening target file: %w", ErrDictzip, err)
		}
		defer dst.Close()
	}

	uncompressedSize, sizes, err := d.decompress(dst, from)
	if err != nil {
		return err
	}

	if d.verbose {
		var compressedSize int64
		for _, size := range sizes {
			compressedSize += int64(size)
		}

		remaining := uncompressedSize
		for i, size := range sizes {
			chunkSize := int64(dictzip.DefaultChunkSize)
			if remaining < chunkSize {
				chunkSize = remaining
			}
			remaining -= chunkSize

			fmt.Printf("chunk %d: %d -> %d (%.2f%%) of %d total\n", i+1, size, chunkSize,
				(1-float64(size)/float64(chunkSize))*100, uncompressedSize)
		}
	}

	if !d.keep && !d.stdout {
		err = os.Remove(d.path)
		if err != nil {
			return fmt.Errorf("%w: removing file: %w", ErrDictzip, err)
		}
	}

	return nil
}

func (d *decompress) decompress(dst io.Writer, src *os.File) (n int64, sizes []int, err error) {
	z, err := dictzip.NewReader(src)
	if err != nil {
		err = fmt.Errorf("%w: reading archive: %w", ErrDictzip, err)
		return
	}
	sizes = z.Sizes()
	defer func() {
		// NOTE: this sets the returned error in the deferred func.
		clsErr := z.Close()
		if err == nil {
			err = clsErr
		}
	}()

	n, err = io.Copy(dst, z)
	if err != nil {
		err = fmt.Errorf("%w: decompressing file %q: %w", ErrDictzip, src.Name(), err)
		return
	}

	return
}
