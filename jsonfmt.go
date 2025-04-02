/*
Flexible JSON formatter. Features:

  - Preserves order.
  - Fits dicts and lists on a single line until a certain width (configurable).
  - Supports comments (configurable).
  - Supports trailing commas (configurable).
  - Fixes missing or broken punctuation.
  - Tiny Go library + optional tiny CLI.

Current limitations:

  - Always permissive. Unrecognized non-whitespace is treated as arbitrary
    content on par with strings, numbers, etc.
  - Slower than `json.Indent` from the Go standard library.
  - Input must be UTF-8.

Source and readme: https://github.com/mitranim/jsonfmt.
*/
package jsonfmt

import (
	"bytes"
	"encoding/json"
	"strings"
	"unicode/utf8"
	"unsafe"
)

/*
Default configuration. To override, make a copy:

	conf := jsonfmt.Default
	conf.CommentLine = `#`
	content = jsonfmt.FormatBytes(conf, content)

See `Conf` for details.
*/
var Default = Conf{
	Indent:            `  `,
	Width:             80,
	CommentLine:       `//`,
	CommentBlockStart: `/*`,
	CommentBlockEnd:   `*/`,
	TrailingComma:     false,
	StripComments:     false,
}

/*
Configuration passed to `Format`. See the variable `Default`.

`.Indent` enables multi-line output. When empty, `jsonfmt` will not emit
separator spaces or newlines, except at the end of single-line comments.
When non-empty, `jsonfmt` will emit separator spaces, newlines, and indents
for contents of lists and dicts. To enforce single-line output, use
`.Indent = ""` and `.StripComments = true`.

`.Width` is the width limit for lists and dicts. If 0, then depending on other
configuration, `jsonfmt` will format lists and dicts either always in
multi-line mode, or always in single-line mode. If > 0, then `jsonfmt` will
attempt to format each list or dict entirely on a single line until the width
limit, falling back on multi-line mode when exceeding the width limit. Note
that multi-line mode also requires non-empty `.Indent`.

`.CommentLine` starts a single-line comment. If empty, single-line comments
won't be detected, and will be treated as arbitrary JSON content.

`.CommentBlockStart` and `.CommentBlockEnd` enable support for block comments.
If both are non-empty, block comments are detected. Nested block comments are
supported. If at least one is empty, then the other will be ignored, block
comments will not be detected, and will be treated as arbitrary JSON content.

`.TrailingComma` enables trailing commas for last elements in dicts and lists in
multi-line mode. In single-line mode, trailing commas are always omitted.

`.StripComments` omits all comments from the output. To enforce single-line
output, specify this together with `.Indent = ""`. When single-line comments
are not omitted from the output, they cause the output to contain newlines,
because each single-line comment must be followed by a newline.
*/
type Conf struct {
	Indent            string `json:"indent"`
	Width             uint64 `json:"width"`
	CommentLine       string `json:"commentLine"`
	CommentBlockStart string `json:"commentBlockStart"`
	CommentBlockEnd   string `json:"commentBlockEnd"`
	TrailingComma     bool   `json:"trailingComma"`
	StripComments     bool   `json:"stripComments"`
}

const (
	separator = ' '
	newline   = '\n'
)

// Describes various interchangeable text types.
type Text interface{ ~string | ~[]byte }

// Formats JSON according to the config. See `Conf`.
func Format[Out, Src Text](conf Conf, src Src) Out {
	fmter := fmter{source: text[string](src), conf: conf}
	fmter.top()
	return text[Out](fmter.buf.Bytes())
}

// Formats JSON text according to config, returning a string.
func FormatString[Src Text](conf Conf, src Src) string {
	return Format[string](conf, src)
}

// Formats JSON text according to config, returning bytes.
func FormatBytes[Src Text](conf Conf, src Src) []byte {
	return Format[[]byte](conf, src)
}

/*
Shortcut that combines formatting with `json.Unmarshal`. Allows to decode JSON
with comments or invalid punctuation, such as trailing commas. Slower than
simply using `json.Unmarshal`. Avoid this when your input is guaranteed to be
valid JSON, or when you should be enforcing valid JSON.
*/
func Unmarshal[Src Text](src Src, out any) error {
	return json.Unmarshal(Format[[]byte](Conf{}, src), out)
}

type fmter struct {
	source   string
	cursor   int
	conf     Conf
	buf      bytes.Buffer
	indent   int
	row      int
	col      int
	discard  bool
	snapshot *fmter
}

func (self *fmter) top() {
	for self.more() {
		if self.skipped() {
			continue
		}

		if self.isNextComment() {
			assert(self.scannedAny())
			self.writeMaybeCommentNewline()
			continue
		}

		if self.scannedAny() {
			self.writeMaybeNewline()
			continue
		}

		self.skipChar()
	}
}

func (self *fmter) any() {
	if self.isNextByte('{') {
		self.dict()
	} else if self.isNextByte('[') {
		self.list()
	} else if self.isNextByte('"') {
		self.string()
	} else if self.isNextCommentLine() {
		self.commentSingle()
	} else if self.isNextCommentBlock() {
		self.commentMulti()
	} else {
		self.atom()
	}
}

func (self *fmter) scannedAny() bool {
	return self.scanned((*fmter).any)
}

func (self *fmter) dict() {
	if !self.preferSingle() || !self.scanned((*fmter).dictSingle) {
		self.dictMulti()
	}
}

func (self *fmter) dictSingle() {
	prev := self.snap()
	defer self.maybeRollback(prev)

	assert(self.isNextByte('{'))
	self.byte()
	key := true

	for self.more() {
		if self.isNextByte('}') {
			self.byte()
			return
		}

		if self.skipped() {
			continue
		}

		if self.isNextComment() {
			assert(self.scannedAny())
			continue
		}

		if key {
			assert(self.scannedAny())
			self.writeByte(':')
			self.writeMaybeSeparator()
			key = false
			continue
		}

		assert(self.scannedAny())
		if self.hasNonCommentsBefore('}') {
			self.writeByte(',')
			self.writeMaybeSeparator()
		}
		key = true
	}
}

func (self *fmter) dictMulti() {
	assert(self.isNextByte('{'))
	self.indent++
	self.byte()
	self.writeMaybeNewline()
	key := true

	for self.more() {
		if self.isNextByte('}') {
			self.indent--
			self.writeMaybeNewlineIndent()
			self.byte()
			return
		}

		if self.skipped() {
			continue
		}

		if self.isNextComment() {
			self.writeMaybeCommentNewlineIndent()
			assert(self.scannedAny())
			continue
		}

		if key {
			self.writeMaybeNewlineIndent()
			assert(self.scannedAny())
			self.writeByte(':')
			self.writeMaybeSeparator()
			key = false
			continue
		}

		assert(self.scannedAny())
		if self.hasNonCommentsBefore('}') {
			self.writeByte(',')
		} else {
			self.writeMaybeTrailingComma()
		}
		key = true
	}
}

func (self *fmter) list() {
	if !self.preferSingle() || !self.scanned((*fmter).listSingle) {
		self.listMulti()
	}
}

func (self *fmter) listSingle() {
	prev := self.snap()
	defer self.maybeRollback(prev)

	assert(self.isNextByte('['))
	self.byte()

	for self.more() {
		if self.isNextByte(']') {
			self.byte()
			return
		}

		if self.skipped() {
			continue
		}

		if self.isNextComment() {
			assert(self.scannedAny())
			continue
		}

		assert(self.scannedAny())
		if self.hasNonCommentsBefore(']') {
			self.writeByte(',')
			self.writeMaybeSeparator()
		}
	}
}

func (self *fmter) listMulti() {
	assert(self.isNextByte('['))
	self.indent++
	self.byte()
	self.writeMaybeNewline()

	for self.more() {
		if self.isNextByte(']') {
			self.indent--
			self.writeMaybeNewlineIndent()
			self.byte()
			return
		}

		if self.skipped() {
			continue
		}

		if self.isNextComment() {
			self.writeMaybeCommentNewlineIndent()
			assert(self.scannedAny())
			continue
		}

		self.writeMaybeNewlineIndent()
		assert(self.scannedAny())
		if self.hasNonCommentsBefore(']') {
			self.writeByte(',')
		} else {
			self.writeMaybeTrailingComma()
		}
	}
}

func (self *fmter) string() {
	assert(self.isNextByte('"'))
	self.byte()

	for self.more() {
		if self.isNextByte('"') {
			self.byte()
			return
		}

		if self.isNextByte('\\') {
			self.byte()
			if self.more() {
				self.char()
			}
			continue
		}

		self.char()
	}
}

func (self *fmter) commentSingle() {
	prefix := self.nextCommentLinePrefix()
	assert(prefix != ``)

	if self.conf.StripComments {
		self.setDiscard(true)
		defer self.setDiscard(false)
	}

	self.strInc(prefix)

	for self.more() {
		if self.isNextPrefix("\r\n") {
			self.skipString("\r\n")
			self.writeNewline()
			return
		}

		if self.isNextByte('\n') || self.isNextByte('\r') {
			self.skipByte()
			self.writeNewline()
			return
		}

		self.char()
	}
}

func (self *fmter) commentMulti() {
	prefix, suffix := self.nextCommentBlockPrefixSuffix()
	assert(prefix != `` && suffix != ``)

	if self.conf.StripComments {
		self.setDiscard(true)
		defer self.setDiscard(false)
	}

	self.strInc(prefix)
	level := 1

	for self.more() {
		if self.isNextPrefix(suffix) {
			self.strInc(suffix)
			level--
			if level == 0 {
				return
			}
			continue
		}

		if self.isNextPrefix(prefix) {
			self.strInc(prefix)
			level++
			continue
		}

		self.char()
	}
}

func (self *fmter) atom() {
	for self.more() && !self.isNextSpace() && !self.isNextTerminal() {
		self.char()
	}
}

func (self *fmter) char() {
	char, size := utf8.DecodeRuneInString(self.rest())
	assert(size > 0)
	self.writeRune(char)
	self.cursor += size
}

func (self *fmter) byte() {
	self.writeByte(self.source[self.cursor])
	self.cursor++
}

// Performance seems fine, probably because `bytes.Buffer` short-circuits ASCII runes.
func (self *fmter) writeByte(char byte) {
	self.writeRune(rune(char))
}

// ALL writes must call this function.
func (self *fmter) writeRune(char rune) {
	if self.discard {
		return
	}

	if char == '\n' || char == '\r' {
		self.row++
		self.col = 0
	} else {
		self.col++
	}

	self.buf.WriteRune(char)

	if self.snapshot != nil && self.exceedsLine(self.snapshot) {
		panic(rollback)
	}
}

func (self *fmter) writeString(str string) {
	for _, char := range str {
		self.writeRune(char)
	}
}

func (self *fmter) writeMaybeSeparator() {
	if self.whitespace() {
		self.writeByte(separator)
	}
}

func (self *fmter) writeMaybeTrailingComma() {
	if self.conf.TrailingComma {
		self.writeByte(',')
	}
}

func (self *fmter) writeMaybeNewline() {
	if self.whitespace() && !self.hasNewlineSuffix() {
		self.writeByte(newline)
	}
}

func (self *fmter) writeNewline() {
	if !self.wrote((*fmter).writeMaybeNewline) {
		self.writeByte(newline)
	}
}

func (self *fmter) writeIndent() {
	for ind := 0; ind < self.indent; ind++ {
		self.writeString(self.conf.Indent)
	}
}

func (self *fmter) writeMaybeNewlineIndent() {
	if self.whitespace() {
		self.writeMaybeNewline()
		self.writeIndent()
	}
}

func (self *fmter) writeMaybeCommentNewlineIndent() {
	if !self.conf.StripComments {
		self.writeMaybeNewlineIndent()
	}
}

func (self *fmter) writeMaybeCommentNewline() {
	if !self.conf.StripComments {
		self.writeMaybeNewline()
	}
}

func (self *fmter) nextCommentLinePrefix() string {
	prefix := self.conf.CommentLine
	if prefix != `` && strings.HasPrefix(self.rest(), prefix) {
		return prefix
	}
	return ``
}

func (self *fmter) nextCommentBlockPrefixSuffix() (string, string) {
	prefix := self.conf.CommentBlockStart
	suffix := self.conf.CommentBlockEnd
	if prefix != `` && suffix != `` && strings.HasPrefix(self.rest(), prefix) {
		return prefix, suffix
	}
	return ``, ``
}

func (self *fmter) hasNonCommentsBefore(char byte) bool {
	prev := *self
	defer self.reset(&prev)

	for self.more() {
		if self.isNextByte(char) {
			return false
		}

		if self.skipped() {
			continue
		}

		if self.isNextComment() {
			assert(self.scannedAny())
			continue
		}

		return true
	}

	return false
}

func (self *fmter) reset(prev *fmter) {
	self.cursor = prev.cursor
	self.indent = prev.indent
	self.row = prev.row
	self.col = prev.col
	self.buf.Truncate(prev.buf.Len())
}

// Causes an escape and a minor heap allocation, but this isn't our bottleneck.
// Ensuring stack allocation in this particular case seems to have no effect on
// performance.
func (self *fmter) snap() *fmter {
	prev := self.snapshot
	snapshot := *self
	self.snapshot = &snapshot
	return prev
}

var rollback = new(struct{})

func (self *fmter) maybeRollback(prev *fmter) {
	snapshot := self.snapshot
	self.snapshot = prev

	val := recover()
	if val == rollback {
		self.reset(snapshot)
	} else if val != nil {
		panic(val)
	}
}

// Used for `defer`.
func (self *fmter) setDiscard(val bool) {
	self.discard = val
}

func (self *fmter) more() bool {
	return self.left() > 0
}

func (self *fmter) left() int {
	return len(self.source) - self.cursor
}

func (self *fmter) headByte() byte {
	if self.cursor < len(self.source) {
		return self.source[self.cursor]
	}
	return 0
}

func (self *fmter) rest() string {
	if self.more() {
		return self.source[self.cursor:]
	}
	return ``
}

func (self *fmter) isNextPrefix(prefix string) bool {
	return strings.HasPrefix(self.rest(), prefix)
}

func (self *fmter) isNextByte(char byte) bool {
	return self.headByte() == char
}

func (self *fmter) isNextSpace() bool {
	return self.isNextByte(' ') || self.isNextByte('\t') || self.isNextByte('\v') ||
		self.isNextByte('\n') || self.isNextByte('\r')
}

/*
We skip punctuation and insert it ourselves where appropriate. This allows us to
automatically fix missing or broken punctuation. The user can write lists or
dicts without punctuation, and we'll insert it. In JSON, this is completely
unambiguous.

Skipping `:` in lists also assists in the edge case of converting between lists
and dicts.
*/
func (self *fmter) isNextPunctuation() bool {
	return self.isNextByte(',') || self.isNextByte(':')
}

func (self *fmter) isNextCommentLine() bool {
	return self.nextCommentLinePrefix() != ``
}

func (self *fmter) isNextCommentBlock() bool {
	prefix, suffix := self.nextCommentBlockPrefixSuffix()
	return prefix != `` && suffix != ``
}

func (self *fmter) isNextTerminal() bool {
	return self.isNextByte('{') ||
		self.isNextByte('}') ||
		self.isNextByte('[') ||
		self.isNextByte(']') ||
		self.isNextByte(',') ||
		self.isNextByte(':') ||
		self.isNextByte('"') ||
		self.isNextComment()
}

func (self *fmter) isNextComment() bool {
	return self.isNextCommentLine() || self.isNextCommentBlock()
}

var (
	bytesLf = []byte("\n")
	bytesCr = []byte("\r")
)

func (self *fmter) hasNewlineSuffix() bool {
	content := self.buf.Bytes()
	return bytes.HasSuffix(content, bytesLf) || bytes.HasSuffix(content, bytesCr)
}

func (self *fmter) exceedsLine(prev *fmter) bool {
	return self.row > prev.row || self.conf.Width > 0 && self.col > int(self.conf.Width)
}

func (self *fmter) skipByte() {
	self.cursor++
}

func (self *fmter) skipChar() {
	_, size := utf8.DecodeRuneInString(self.rest())
	self.cursor += size
}

func (self *fmter) skipString(str string) {
	self.skipNBytes(len(str))
}

func (self *fmter) skipNBytes(n int) {
	self.cursor += n
}

func (self *fmter) strInc(str string) {
	self.writeString(str)
	self.skipString(str)
}

func (self *fmter) scanned(fun func(*fmter)) bool {
	start := self.cursor
	fun(self)
	return self.cursor > start
}

func (self *fmter) wrote(fun func(*fmter)) bool {
	start := self.buf.Len()
	fun(self)
	return self.buf.Len() > start
}

func (self *fmter) skipped() bool {
	if self.isNextSpace() || self.isNextPunctuation() {
		self.skipByte()
		return true
	}
	return false
}

func (self *fmter) preferSingle() bool {
	return self.conf.Width > 0
}

func (self *fmter) whitespace() bool {
	return self.conf.Indent != ``
}

// Allocation-free conversion between two text types.
func text[Out, Src Text](src Src) Out { return *(*Out)(unsafe.Pointer(&src)) }

func assert(ok bool) {
	if !ok {
		panic(`[jsonfmt] internal error: failed a condition that should never be failed, see the stacktrace`)
	}
}
