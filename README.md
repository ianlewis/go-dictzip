# go-dictzip

[![Go Reference](https://pkg.go.dev/badge/github.com/ianlewis/go-dictzip.svg)](https://pkg.go.dev/github.com/ianlewis/go-dictzip)
[![codecov](https://codecov.io/gh/ianlewis/go-dictzip/graph/badge.svg?token=LJVTOT3ZHE)](https://codecov.io/gh/ianlewis/go-dictzip)
[![tests](https://github.com/ianlewis/go-dictzip/actions/workflows/pre-submit.units.yml/badge.svg)](https://github.com/ianlewis/go-dictzip/actions/workflows/pre-submit.units.yml)

go-dictzip is a Go library for reading and writing [`dictzip`](https://linux.die.net/man/1/dictzip)

## Status

The API is currently _unstable_ and will change. This package will use [module
version numbering](https://golang.org/doc/modules/version-numbers) to manage
versions and compatibility.

## Installation

To install this package run

`go get github.com/ianlewis/go-dictzip`

## Examples

You can open a dictionary file and read it much like a normal reader.
Random access can be performed using the `ReadAt` method.

```golang
// Open the dictionary.
f, _ := os.Open("dictionary.dict.dz")
r, _ := dictzip.NewReader(f)

buf := make([]byte, 12)
_, _ = r.ReadAt(buf, 5)
```

## Related projects

- [pebbe/dictzip](https://github.com/pebbe/dictzip)

## References

- [dictzip(1)](https://linux.die.net/man/1/dictzip) - Linux man page
- [RFC 1952](https://datatracker.ietf.org/doc/html/rfc1952) - GZIP file format specification
