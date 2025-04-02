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

Flags:

`

func main() {
	conf := jsonfmt.Default

	flag.Usage = usage
	flag.StringVar(&conf.Indent, `i`, conf.Indent, `indentation`)
	flag.Uint64Var(&conf.Width, `w`, conf.Width, `line width`)
	flag.StringVar(&conf.CommentLine, `l`, conf.CommentLine, `beginning of line comment`)
	flag.StringVar(&conf.CommentBlockStart, `b`, conf.CommentBlockStart, `beginning of block comment`)
	flag.StringVar(&conf.CommentBlockEnd, `e`, conf.CommentBlockEnd, `end of block comment`)
	flag.BoolVar(&conf.TrailingComma, `t`, conf.TrailingComma, `trailing commas when multiline`)
	flag.BoolVar(&conf.StripComments, `s`, conf.StripComments, `strip comments`)
	flag.Parse()

	args := flag.Args()

	if len(args) > 0 {
		if args[0] == `help` {
			usage()
			os.Exit(0)
			return
		}

		fmt.Fprintf(os.Stderr, `[jsonfmt] unexpected arguments %q`, args)
		os.Exit(1)
		return
	}

	src, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, `[jsonfmt] failed to read: %v`, err)
		os.Exit(1)
		return
	}

	_, err = os.Stdout.Write(jsonfmt.FormatBytes(conf, src))
	if err != nil {
		fmt.Fprintf(os.Stderr, `[jsonfmt] failed to write: %v`, err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, help)
	flag.PrintDefaults()
}
