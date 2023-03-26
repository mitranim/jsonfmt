/*
Command line tool for jsonfmt.

Installation:

	go get -u github.com/mitranim/jsonfmt

Usage:

	jsonfmt -h

Source and readme: https://github.com/mitranim/jsonfmt.
*/
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/mitranim/jsonfmt"
)

const help = `jsonfmt is a command-line JSON formatter. It reads from stdin and
writes to stdout. For files, use pipe and redirect:

	cat <src_file>.json | jsonfmt <flags>
	cat <src_file>.json | jsonfmt <flags> > <out_file>.json

In addition to CLI, it's also available as a Go library:

	https://github.com/mitranim/jsonfmt

Settings:

`

func main() {
	conf := jsonfmt.Default

	flag.StringVar(&conf.Indent, `i`, conf.Indent, `indentation`)
	flag.Uint64Var(&conf.Width, `w`, conf.Width, `line width`)
	flag.StringVar(&conf.CommentLine, `l`, conf.CommentLine, `beginning of line comment`)
	flag.StringVar(&conf.CommentBlockStart, `b`, conf.CommentBlockStart, `beginning of block comment`)
	flag.StringVar(&conf.CommentBlockEnd, `e`, conf.CommentBlockEnd, `end of block comment`)
	flag.BoolVar(&conf.TrailingComma, `t`, conf.TrailingComma, `trailing commas when multiline`)
	flag.BoolVar(&conf.StripComments, `s`, conf.StripComments, `strip comments`)

	flag.Usage = func() {
		fmt.Fprint(flag.CommandLine.Output(), help)
		flag.PrintDefaults()
	}

	flag.Parse()
	args()

	source, err := io.ReadAll(os.Stdin)
	if err != nil {
		fail(fmt.Errorf(`[jsonfmt] failed to read: %w`, err))
	}

	_, err = os.Stdout.Write(jsonfmt.FormatBytes(conf, source))
	if err != nil {
		fail(fmt.Errorf(`[jsonfmt] failed to write: %w`, err))
	}
}

func fail(err error) {
	fmt.Fprintf(flag.CommandLine.Output(), `%+v`, err)
	os.Exit(1)
}

func args() {
	args := flag.Args()
	if len(args) == 0 {
		return
	}

	if args[0] == `help` {
		flag.Usage()
		os.Exit(0)
	}

	fail(fmt.Errorf(`[jsonfmt] unexpected arguments %q`, args))
}
