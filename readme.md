## Overview

Flexible JSON formatter. Features:

* Preserves order.
* Fits dicts and lists on a single line until a certain width (configurable).
* Supports comments (configurable).
* Supports trailing commas (configurable).
* Fixes missing or broken punctuation.
* Tiny Go library.
* Optional tiny CLI.
* No dependencies.

See API documentation at https://godoc.org/github.com/mitranim/jsonfmt.

Current limitations:

* Always permissive. Unrecognized non-whitespace is treated as arbitrary content on par with strings, numbers, etc.
* Slower than `json.Indent` from the Go standard library.
* Input must be UTF-8.
* Input and output are `[]byte`, without streaming.
  * Streaming support could be added on demand.

## Installation

### Library

To use this as a library, simply import it:

```go
import "github.com/mitranim/jsonfmt"

formatted := jsonfmt.FormatString(`{}`, jsonfmt.Default)
formatted := jsonfmt.FormatBytes([]byte(`{}`), jsonfmt.Default)
```

### CLI

First, install Go: https://golang.org. Then run this:

```sh
go install github.com/mitranim/jsonfmt/jsonfmt@latest
```

This will compile the executable into `$GOPATH/bin/jsonfmt`. Make sure `$GOPATH/bin` is in your `$PATH` so the shell can discover the `jsonfmt` command. For example, my `~/.profile` contains this:

```sh
export GOPATH=~/go
export PATH=$PATH:$GOPATH/bin
```

Alternatively, you can run the executable using the full path. At the time of writing, `~/go` is the default `$GOPATH` for Go installations. Some systems may have a different one.

```sh
~/go/bin/jsonfmt
```

## Usage

See the library documentation on https://godoc.org/github.com/mitranim/jsonfmt.

For CLI usage, run `jsonfmt -h`.

## Examples

**Supports comments and trailing commas** (all configurable):

```jsonc
{// Line comment
"one": "two", /* Block comment */ "three": 40}
```

Output:

```jsonc
{
  // Line comment
  "one": "two",
  /* Block comment */
  "three": 40,
}
```

**Single-line until width limit** (configurable):

```jsonc
{
  "one": {"two": ["three"], "four": ["five"]},
  "six": {"seven": ["eight"], "nine": ["ten"], "eleven": ["twelve"], "thirteen": ["fourteen"]}
}
```

Output:

```jsonc
{
  "one": {"two": ["three"], "four": ["five"]},
  "six": {
    "seven": ["eight"],
    "nine": ["ten"],
    "eleven": ["twelve"],
    "thirteen": ["fourteen"],
  },
}
```

**Fix missing or broken punctuation**:

```jsonc
{"one" "two" "three" {"four" "five"} "six" ["seven": "eight"]},,,
```

Output:

```jsonc
{"one": "two", "three": {"four": "five"}, "six": ["seven", "eight"]}
```

## License

https://unlicense.org

## Misc

I'm receptive to suggestions. If this library _almost_ satisfies you but needs changes, open an issue or chat me up. Contacts: https://mitranim.com/#contacts
