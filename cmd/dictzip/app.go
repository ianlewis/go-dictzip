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
)

const (
	// ExitCodeSuccess is successful error code.
	ExitCodeSuccess int = iota

	// ExitCodeFlagParseError is the exit code for a flag parsing error.
	ExitCodeFlagParseError

	// ExitCodeUnknownError is the exit code for an unknown error.
	ExitCodeUnknownError
)

// ErrDictzip is a parent error for all dictzip command errors.
var ErrDictzip = errors.New("dictzip")

// ErrFlagParse is a flag parsing error.
var ErrFlagParse = fmt.Errorf("%w: parsing flags", ErrDictzip)

// ErrUnsupported indicates a feature is unsupported.
var ErrUnsupported = fmt.Errorf("%w: unsupported", ErrDictzip)

//nolint:gochecknoinits // init needed needed for global variable.
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
			&cli.BoolFlag{
				Name:               "no-name",
				Usage:              "don't save the original filename and timestamp",
				Aliases:            []string{"n"},
				DisableDefaultText: true,
			},
			&cli.BoolFlag{
				Name:               "keep",
				Usage:              "do not delete original file",
				Aliases:            []string{"k"},
				DisableDefaultText: true,
			},
			&cli.BoolFlag{
				Name:               "list",
				Usage:              "list compressed file contents",
				Aliases:            []string{"l"},
				DisableDefaultText: true,
			},
			&cli.BoolFlag{
				Name:               "test",
				Usage:              "test compressed file integrity",
				Aliases:            []string{"t"},
				DisableDefaultText: true,
			},
			&cli.BoolFlag{
				Name:               "license",
				Usage:              "display software license",
				Aliases:            []string{"L"},
				DisableDefaultText: true,
			},
			&cli.BoolFlag{
				Name:               "stdout",
				Usage:              "write to stdout (decompression only)",
				Aliases:            []string{"c"},
				DisableDefaultText: true,
			},

			&cli.BoolFlag{
				Name:               "verbose",
				Usage:              "verbose mode",
				Aliases:            []string{"v"},
				DisableDefaultText: true,
			},

			// NOTE: -D --debug flag is not supported.

			&cli.Int64Flag{
				Name:    "start",
				Usage:   "starting `offset` for decompression (decimal)",
				Aliases: []string{"s"},
				Value:   0,
			},
			&cli.Int64Flag{
				Name:        "size",
				Usage:       "`size` for decompression (decimal)",
				Aliases:     []string{"e"},
				DefaultText: "whole file",
				Value:       -1,
			},
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
				Aliases:            []string{"V"},
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
				return printVersion(c)
			}

			if c.Bool("license") {
				return printLicense(c)
			}

			if c.Bool("list") || c.Bool("test") {
				for _, path := range c.Args().Slice() {
					l := list{
						path: path,
					}
					if err := l.Run(); err != nil {
						return err
					}
				}
				return nil
			}

			// If --start or --size are specified --decompress is implied.
			if c.IsSet("start") || c.IsSet("size") {
				if err := c.Set("decompress", "true"); err != nil {
					return fmt.Errorf("%w: internal error: %w", ErrDictzip, err)
				}
			}

			// decompress
			if c.Bool("decompress") {
				// If --stdout is specified, --keep is implied.
				if c.Bool("stdout") {
					if err := c.Set("keep", "true"); err != nil {
						return fmt.Errorf("%w: internal error: %w", ErrDictzip, err)
					}
				}

				for _, path := range c.Args().Slice() {
					d := decompress{
						path:    path,
						force:   c.Bool("force"),
						keep:    c.Bool("keep"),
						stdout:  c.Bool("stdout"),
						verbose: c.Bool("verbose"),
						start:   c.Int64("start"),
						size:    c.Int64("size"),
					}
					if err := d.Run(); err != nil {
						return err
					}
				}
				return nil
			}

			// compress
			for _, path := range c.Args().Slice() {
				// compress
				c := compress{
					path:    path,
					force:   c.Bool("force"),
					noName:  c.Bool("no-name"),
					keep:    c.Bool("keep"),
					verbose: c.Bool("verbose"),
				}
				if err := c.Run(); err != nil {
					return err
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
