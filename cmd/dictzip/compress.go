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
	path   string
	force  bool
	noName bool
	keep   bool
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

	z, err := dictzip.NewWriter(dst)
	if err != nil {
		return fmt.Errorf("%w: creating writer: %w", ErrDictzip, err)
	}
	z.ModTime = modTime
	z.Name = fName
	defer z.Close()

	_, err = io.Copy(z, from)
	if err != nil {
		return fmt.Errorf("%w: decompressing file %q: %w", ErrDictzip, from.Name(), err)
	}

	if !c.keep {
		err = os.Remove(c.path)
		if err != nil {
			return fmt.Errorf("%w: removing file: %w", ErrDictzip, err)
		}
	}

	return nil
}
