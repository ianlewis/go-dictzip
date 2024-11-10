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
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ianlewis/go-dictzip"
)

type decompress struct {
	path  string
	force bool
}

var errTruncate = errors.New("cannot truncate filename")

func (d *decompress) Run() error {
	newPath := strings.TrimRight(d.path, filepath.Ext(d.path))
	if newPath == d.path {
		return fmt.Errorf("%w: %q", errTruncate, d.path)
	}

	from, err := os.Open(d.path)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer from.Close()

	flags := os.O_CREATE | os.O_WRONLY
	if !d.force {
		flags |= os.O_EXCL
	}
	// TODO(#13): carry over timestamp if d.NoName not set.
	// TODO(#13): Restore original file name from the name header?
	dst, err := os.OpenFile(newPath, flags, 0644)
	if err != nil {
		return fmt.Errorf("opening target file: %w", err)
	}
	defer dst.Close()

	z, err := dictzip.NewReader(from)
	if err != nil {
		return fmt.Errorf("reading archive: %w", err)
	}

	_, err = io.Copy(dst, z)
	if err != nil {
		return fmt.Errorf("decompressing file %q: %w", from.Name(), err)
	}

	err = os.Remove(d.path)
	if err != nil {
		return fmt.Errorf("removing file: %w", err)
	}

	return nil
}
