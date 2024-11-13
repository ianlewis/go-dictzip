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

// Package dictzip implements the dictzip compression format.
// Dictzip compresses files using the gzip(1) algorithm (LZ77) in a manner which
// is completely compatible with the gzip file format.
// See: https://linux.die.net/man/1/dictzip
// See: https://linux.die.net/man/1/gzip
// See: https://datatracker.ietf.org/doc/html/rfc1952
//
// Unless otherwise informed clients should not assume implementations in this
// package are safe for parallel execution.
package dictzip
