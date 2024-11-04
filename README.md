# repo-template-go

[![tests](https://github.com/ianlewis/repo-template/actions/workflows/pre-submit.units.yml/badge.svg)](https://github.com/ianlewis/repo-template/actions/workflows/pre-submit.units.yml)

Repository template for Go repos under github.com/ianlewis

## Usage

1. Update `go.mod`

   The `module` directive in `go.mod` must be updated to the correct module
   name. The `go` directive sohuld be updated to the required Go version.

   ```text
   module github.com/user/repo

   go 1.23.2
   ```
