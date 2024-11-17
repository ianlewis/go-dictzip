# Changelog

All notable changes to go-dictzip will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0] - 2024-11-17

### Added

- An `io.Writer` implementation was added for writing dictzip files.
- A [dictzip(1)](https://linux.die.net/man/1/dictzip) compatible command was
  added supporting compression, decompression, and listing archive contents.
- The library now supports Go 1.20+.

## [0.1.0] - 2024-11-09

- Initial release
- dictzip `Reader` implementation.

[0.1.0]: https://github.com/ianlewis/go-dictzip/releases/tag/v0.1.0
[0.2.0]: https://github.com/ianlewis/go-dictzip/releases/tag/v0.2.0
