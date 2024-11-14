# go-dictzip

[![Go Reference](https://pkg.go.dev/badge/github.com/ianlewis/go-dictzip.svg)](https://pkg.go.dev/github.com/ianlewis/go-dictzip)
[![codecov](https://codecov.io/gh/ianlewis/go-dictzip/graph/badge.svg?token=LJVTOT3ZHE)](https://codecov.io/gh/ianlewis/go-dictzip)
[![tests](https://github.com/ianlewis/go-dictzip/actions/workflows/pre-submit.units.yml/badge.svg)](https://github.com/ianlewis/go-dictzip/actions/workflows/pre-submit.units.yml)

go-dictzip is a Go library for reading and writing
[`dictzip`](https://linux.die.net/man/1/dictzip) files.

## Status

The API is currently _unstable_ and will change. This package will use [module
version numbering](https://golang.org/doc/modules/version-numbers) to manage
versions and compatibility.

## Installation

To install this package run

`go get github.com/ianlewis/go-dictzip`

## Examples

### Reading compressed files.

You can open a dictionary file and read it much like a normal reader.

```golang
// Open the dictionary.
f, _ := os.Open("dictionary.dict.dz")
r, _ := dictzip.NewReader(f)
defer r.Close()

uncompressedData, _ = io.ReadAll(r)
```

### Random access.

Random access can be performed using the `ReadAt` method.

```golang
// Open the dictionary.
f, _ := os.Open("dictionary.dict.dz")
r, _ := dictzip.NewReader(f)
defer r.Close()

buf := make([]byte, 12)
_, _ = r.ReadAt(buf, 5)
```

### Writing compressed files.

Dictzip files can be written using the `dictzip.Writer`. Compressed data is
stored in chunks and chunk sizes are stored in the archive header allowing for
more efficient random access.

```golang
// Open the dictionary.
f, _ := os.Open("dictionary.dict.dz", os.O_WRONLY|os.O_CREATE, 0o644)
w, _ := dictzip.NewWriter(f)
defer w.Close()

buf := []byte("Hello World!")
_, _ = r.Write(buf)
```

## dictzip Command

This repository also includes a `dictzip` command that is compatible with the
[dictzip(1)](https://linux.die.net/man/1/dictzip) command.

```shell
# compress dictionary.dict to dictionary.dict.dz
$ dictzip dictionary.dict

# decompress dictionary.dict.dz to dictionary.dict
$ dictzip -d dictionary.dict.dz
```

## Related projects

- [pebbe/dictzip](https://github.com/pebbe/dictzip)

## References

- [dictzip(1)](https://linux.die.net/man/1/dictzip) - Linux man page
- [RFC 1952](https://datatracker.ietf.org/doc/html/rfc1952) - GZIP file format specification
