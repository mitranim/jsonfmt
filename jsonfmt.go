/*
Flexible JSON formatter. Features:

	* Preserves order.
	* Fits dicts and lists on a single line until a certain width (configurable).
	* Supports comments (configurable).
	* Supports trailing commas (configurable).
	* Fixes missing or broken punctuation.
	* Tiny Go library + optional tiny CLI.

Current limitations:

	* Always permissive. Unrecognized non-whitespace is treated as arbitrary
	  content on par with strings, numbers, etc.
	* Slower than `json.Indent` from the Go standard library.
	* Input must be UTF-8.

Source and readme: https://github.com/mitranim/jsonfmt.
*/
package jsonfmt

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"
	"unsafe"
)

/*
Default configuration. To override, make a copy:

	conf := jsonfmt.Default
	conf.CommentLine = "#"
	content = jsonfmt.Fmt(content, conf)

See `Conf` for details.
*/
var Default = Conf{
	Indent:            "  ",
	Width:             80,
	CommentLine:       "//",
	CommentBlockStart: "/*",
	CommentBlockEnd:   "*/",
	TrailingComma:     false,
	StripComments:     false,
}

/*
Configuration passed to `Fmt`. See the variable `Default`.

`Indent` controls multi-line output. When empty, jsonfmt will not emit separator
spaces or newlines, except at the end of single-line comments. To enforce
single-line output, use `Indent: ""` and `StripComments: true`.

`Width` is the width limit for single-line formatting. If 0, jsonfmt will prefer
multi-line mode. Note that `Indent` must be set for multi-line.

`CommentLine` starts a single-line comment. If empty, single-line comments won't
be detected, and will be treated as arbitrary content surrounded by punctuation.

`CommentBlockStart` and `CommentBlockEnd` must both be set to work. If only one
is set, the other is ignored. Nested block comments are supported. If unset,
block comments will not be detected, and will be treated as arbitrary content
surrounded by punctuation.

`TrailingComma` controls trailing commas for last elements in dicts and lists in
multi-line mode. In single-line mode, trailing commas are always omitted.

`StripComments` omits all comments from the output. To enforce single-line mode,
specify this together with `Indent: ""`. Otherwise, single-line comments are
always followed by a newline.
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

// Formats JSON according to the config. See `Conf`.
func Fmt(source []byte, conf Conf) []byte {
	fmter := fmter{source: bytesToMutableString(source), conf: conf}
	fmter.top()
	return fmter.buf.Bytes()
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
			assert(self.didAny())
			continue
		}

		if self.didAny() {
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
	} else if self.isNextCommentSingle() {
		self.commentSingle()
	} else if self.isNextCommentMulti() {
		self.commentMulti()
	} else {
		self.atom()
	}
}

func (self *fmter) didAny() bool {
	return self.scanned((*fmter).any)
}

func (self *fmter) dict() {
	if !self.preferSingle() || !self.scanned((*fmter).dictSingle) {
		self.dictMulti()
	}
}

func (self *fmter) dictSingle() {
	prev := self.snap()
	defer self.rollbackMulti(prev)

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
			assert(self.didAny())
			continue
		}

		if key {
			assert(self.didAny())
			self.writeByte(':')
			self.writeMaybeSeparator()
			key = false
			continue
		}

		assert(self.didAny())
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
			assert(self.didAny())
			continue
		}

		if key {
			self.writeMaybeNewlineIndent()
			assert(self.didAny())
			self.writeByte(':')
			self.writeMaybeSeparator()
			key = false
			continue
		}

		assert(self.didAny())
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
	defer self.rollbackMulti(prev)

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
			assert(self.didAny())
			continue
		}

		assert(self.didAny())
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
			assert(self.didAny())
			continue
		}

		self.writeMaybeNewlineIndent()
		assert(self.didAny())
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
	prefix := self.nextCommentSingle()
	assert(prefix != "")

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
	prefix, suffix := self.nextCommentMulti()
	assert(prefix != "" && suffix != "")

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
	for !self.isNextSpace() && !self.isNextTerminal() {
		self.char()
	}
}

func (self *fmter) char() {
	char, size := utf8.DecodeRuneInString(self.rest())
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
	for i := 0; i < self.indent; i++ {
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

func (self *fmter) nextCommentSingle() string {
	prefix := self.conf.CommentLine
	if prefix != "" && strings.HasPrefix(self.rest(), prefix) {
		return prefix
	}
	return ""
}

func (self *fmter) nextCommentMulti() (string, string) {
	prefix := self.conf.CommentBlockStart
	suffix := self.conf.CommentBlockEnd
	if prefix != "" && suffix != "" && strings.HasPrefix(self.rest(), prefix) {
		return prefix, suffix
	}
	return "", ""
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
			assert(self.didAny())
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

func (self *fmter) snap() *fmter {
	prev := self.snapshot
	snapshot := *self
	self.snapshot = &snapshot
	return prev
}

var rollback = new(struct{})

func (self *fmter) rollbackMulti(prev *fmter) {
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
	return ""
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

func (self *fmter) isNextCommentSingle() bool {
	return self.nextCommentSingle() != ""
}

func (self *fmter) isNextCommentMulti() bool {
	prefix, suffix := self.nextCommentMulti()
	return prefix != "" && suffix != ""
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
	return self.isNextCommentSingle() ||
		self.isNextCommentMulti()
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
	return self.conf.Indent != ""
}

func must(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%+v", err)
		os.Exit(1)
	}
}

func bytesToMutableString(bytes []byte) string {
	return *(*string)(unsafe.Pointer(&bytes))
}

func assert(ok bool) {
	if !ok {
		panic("[jsonfmt] assertion failure, see the stacktrace")
	}
}
