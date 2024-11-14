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
	"time"

	"github.com/ianlewis/go-dictzip"
)

type compress struct {
	path    string
	force   bool
	noName  bool
	keep    bool
	verbose bool
}

func (c *compress) Run() error {
	newPath := c.path + ".dz"

	from, err := os.Open(c.path)
	if err != nil {
		return fmt.Errorf("%w: opening file: %w", ErrDictzip, err)
	}
	defer from.Close()

	var fName string
	var modTime time.Time
	if !c.noName {
		var fInfo os.FileInfo
		fInfo, err = from.Stat()
		if err != nil {
			return fmt.Errorf("%w: stat %q: %w", ErrDictzip, from.Name(), err)
		}
		modTime = fInfo.ModTime()
		fName = filepath.Base(from.Name())
	}

	flags := os.O_CREATE | os.O_WRONLY
	if !c.force {
		// Do not overwrite existing files unless --force is specified.
		flags |= os.O_EXCL
	}

	dst, err := os.OpenFile(newPath, flags, 0o644)
	if err != nil {
		return fmt.Errorf("%w: opening target file: %w", ErrDictzip, err)
	}
	defer dst.Close()

	uncompressedSize, sizes, err := c.compress(dst, from, fName, modTime)
	if err != nil {
		return err
	}

	remaining := uncompressedSize
	if c.verbose {
		var compressedSize int64
		for _, size := range sizes {
			compressedSize += int64(size)
		}
		chunkSize := int64(dictzip.DefaultChunkSize)
		if remaining < chunkSize {
			chunkSize = remaining
		}
		remaining -= chunkSize
		for i, size := range sizes {
			fmt.Printf("chunk %d: %d -> %d (%.2f%%) of %d total\n", i+1, chunkSize, size,
				(1-float64(size)/float64(chunkSize))*100, uncompressedSize)
		}
	}

	if !c.keep {
		err = os.Remove(c.path)
		if err != nil {
			return fmt.Errorf("%w: removing file: %w", ErrDictzip, err)
		}
	}

	return nil
}

func (c *compress) compress(dst io.Writer, src *os.File, name string, modTime time.Time) (n int64, sizes []int, err error) {
	z, err := dictzip.NewWriter(dst)
	if err != nil {
		err = fmt.Errorf("%w: creating writer: %w", ErrDictzip, err)
		return
	}
	z.ModTime = modTime
	z.Name = name
	defer func() {
		// NOTE: this sets the returned error in the deferred func.
		clsErr := z.Close()
		if err == nil {
			err = clsErr
		}
		if clsErr != nil {
			return
		}
		sizes = z.Sizes()
	}()

	n, err = io.Copy(z, src)
	if err != nil {
		err = fmt.Errorf("%w: decompressing file %q: %w", ErrDictzip, src.Name(), err)
		return
	}
	return
}
