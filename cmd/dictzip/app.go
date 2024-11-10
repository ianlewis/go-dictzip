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
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"
	"sigs.k8s.io/release-utils/version"
)

const (
	// ExitCodeSuccess is successful error code.
	ExitCodeSuccess int = iota

	// ExitCodeFlagParseError is the exit code for a flag parsing error.
	ExitCodeFlagParseError

	// ExitCodeUnknownError is the exit code for an unknown error.
	ExitCodeUnknownError
)

// ErrFlagParse is a flag parsing error.
var ErrFlagParse = errors.New("parsing flags")

// ErrUnsupported indicates a feature is unsupported.
var ErrUnsupported = errors.New("unsupported")

func init() {
	// Set the HelpFlag to a random name so that it isn't used. `cli` handles
	// the flag with the root command such that it takes a command name argument
	// but we don't use commands.
	//
	// This is done because `dictzip --help foo` will display a
	// "command foo not found" error instead of the help.
	//
	// This flag is hidden by the help output.
	// See: github.com/urfave/cli/issues/1809
	cli.HelpFlag = &cli.BoolFlag{
		// NOTE: Use a random name no one would guess.
		Name:               "d41d8cd98f00b204e980",
		DisableDefaultText: true,
	}
}

// check checks the error and panics if not nil.
func check(err error) {
	if err != nil {
		panic(err)
	}
}

// must checks the error and panics if not nil.
func must[T any](val T, err error) T {
	if err != nil {
		panic(err)
	}
	return val
}

func newDictzipApp() *cli.App {
	return &cli.App{
		Name:  filepath.Base(os.Args[0]),
		Usage: "Compress dictzip files.",
		Description: strings.Join([]string{
			"dictzip(1) compatible CLI written in Go.",
			"http://github.com/ianlewis/go-dictzip",
		}, "\n"),
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:               "decompress",
				Usage:              "decompress a dictzip file",
				Aliases:            []string{"d"},
				DisableDefaultText: true,
			},
			&cli.BoolFlag{
				Name:               "force",
				Usage:              "force overwrite of output file",
				Aliases:            []string{"f"},
				DisableDefaultText: true,
			},

			// TODO(#13): -n --no-name         don't save the original filename and timestamp
			// TODO(#13): -k --keep            do not delete original file
			// TODO(#13): -l --list            list compressed file contents
			// TODO(#13): -L --license         display software license
			// TODO(#13): -c --stdout          write to stdout (decompression only)
			// TODO(#13): -t --test            test compressed file integrity
			// TODO(#13): -v --verbose         verbose mode
			// TODO(#13): -V --version         display version number
			// TODO(#13): -D --debug           select debug option
			// TODO(#13): -s --start <offset>  starting offset for decompression (decimal)
			// TODO(#13): -e --size <offset>   size for decompression (decimal)
			// TODO(#13): -S --Start <offset>  starting offset for decompression (base64)
			// TODO(#13): -E --Size <offset>   size for decompression (base64)
			// TODO(#13): -p --pre <filter>    pre-compression filter
			// TODO(#13): -P --post <filter>   post-compression filter

			// Special flags are shown at the end.
			&cli.BoolFlag{
				Name:               "help",
				Usage:              "print this help text and exit",
				Aliases:            []string{"h"},
				DisableDefaultText: true,
			},
			&cli.BoolFlag{
				Name:               "version",
				Usage:              "print version information and exit",
				Aliases:            []string{"v"},
				DisableDefaultText: true,
			},
		},
		ArgsUsage:       "[PATH]...",
		Copyright:       "Google LLC",
		HideHelp:        true,
		HideHelpCommand: true,
		Action: func(c *cli.Context) error {
			if c.Bool("help") {
				check(cli.ShowAppHelp(c))
				return nil
			}

			if c.Bool("version") {
				versionInfo := version.GetVersionInfo()
				_ = must(fmt.Fprintf(c.App.Writer, `%s %s
Copyright (c) Google LLC

%s`, c.App.Name, versionInfo.GitVersion, versionInfo.String()))
				return nil
			}

			for _, path := range c.Args().Slice() {
				if c.Bool("decompress") {
					d := decompress{
						path:  path,
						force: c.Bool("force"),
					}
					return d.Run()
				} else {
					return fmt.Errorf("%w: compression not supported yet.", ErrUnsupported)
				}
			}

			return nil
		},
		ExitErrHandler: func(c *cli.Context, err error) {
			if err == nil {
				return
			}

			// ExitCode return an exit code for the given error.
			_ = must(fmt.Fprintf(c.App.ErrWriter, "%s: %v\n", c.App.Name, err))
			if errors.Is(err, ErrFlagParse) {
				cli.OsExiter(ExitCodeFlagParseError)
				return
			}

			cli.OsExiter(ExitCodeUnknownError)
		},
	}
}