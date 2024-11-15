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

	"github.com/rodaine/table"

	"github.com/ianlewis/go-dictzip"
)

type list struct {
	path string
}

func (l *list) Run() error {
	f, err := os.Open(l.path)
	if err != nil {
		return fmt.Errorf("%w: opening file: %w", ErrDictzip, err)
	}
	defer f.Close()

	z, err := dictzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("%w: reading archive: %w", ErrDictzip, err)
	}
	defer z.Close()

	fInfo, err := f.Stat()
	if err != nil {
		return fmt.Errorf("%w: stat: %w", ErrDictzip, err)
	}

	compressed := fInfo.Size()
	uncompressed, err := io.Copy(io.Discard, z)
	if err != nil {
		return fmt.Errorf("%w: reading archive: %w", ErrDictzip, err)
	}

	tbl := table.New("type", "date", "time", "chunks", "size", "compressed", "uncompressed", "ratio", "name")
	tbl.AddRow(
		"dzip",
		z.ModTime.Format("2006-01-02"),
		z.ModTime.Format("15:04:05"),
		len(z.Sizes()),
		z.ChunkSize(),
		fmt.Sprintf("%d", compressed),
		fmt.Sprintf("%d", uncompressed),
		fmt.Sprintf("%.1f%%", (1-float64(compressed)/float64(uncompressed))*100),
		z.Name,
	)
	tbl.Print()

	return nil
}
