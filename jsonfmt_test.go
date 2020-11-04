package jsonfmt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	DIR_TEST_FILES      = "testfiles"
	FMTED_SUFFIX        = "_fmted"
	STD_COMPATIBLE_FILE = "in_long_pure.json"
)

func BenchmarkStdlibIndent(b *testing.B) {
	content := readTestFile(b, STD_COMPATIBLE_FILE)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		must(json.Indent(&buf, content, "", "  "))
	}
}

func BenchmarkFmt(b *testing.B) {
	content := readTestFile(b, STD_COMPATIBLE_FILE)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = Fmt(content, Default)
	}
}

func TestMain(m *testing.M) {
	must(deleteTestFiles("*" + FMTED_SUFFIX + ".*"))

	code := m.Run()
	if code == 0 {
		must(deleteTestFiles("*" + FMTED_SUFFIX + ".*"))
	}

	os.Exit(code)
}

// Sanity check for the test itself.
func TestStdlibIndent(t *testing.T) {
	const src = STD_COMPATIBLE_FILE
	content := readTestFile(t, src)

	var buf bytes.Buffer
	must(json.Indent(&buf, content, "", "  "))

	eqFile(t, src, "out_long_multi.json", buf.Bytes())
}

func TestFmtHybrid(t *testing.T) {
	conf := Default
	conf.TrailingComma = true

	const src = "in_long_comments.json"
	input := readTestFile(t, src)
	output := Fmt(input, conf)
	eqFile(t, src, "out_long_hybrid_commas_comments.json", output)
}

func TestFmtHybridStripComments(t *testing.T) {
	conf := Default
	conf.TrailingComma = true
	conf.StripComments = true

	const src = "in_long_comments.json"
	input := readTestFile(t, src)
	output := Fmt(input, conf)
	eqFile(t, src, "out_long_hybrid_commas.json", output)
}

func TestFmtInsertPunctuation(t *testing.T) {
	conf := Default
	conf.TrailingComma = true

	const src = "in_short_nopunc.json"
	input := readTestFile(t, src)
	output := Fmt(input, conf)
	eqFile(t, src, "out_short_punc.json", output)
}

func TestFmtSingleLineWithComments(t *testing.T) {
	conf := Default
	conf.Indent = ""
	conf.StripComments = false

	const src = "in_long_comments.json"
	input := readTestFile(t, src)
	output := Fmt(input, conf)
	eqFile(t, src, "out_long_single_comments.json", output)
}

func TestFmtSingleLineStripComments(t *testing.T) {
	conf := Default
	conf.Indent = ""
	conf.StripComments = true

	const src = "in_long_comments.json"
	input := readTestFile(t, src)
	output := Fmt(input, conf)
	eqFile(t, src, "out_long_single_stripped.json", output)
}

func eqFile(tb testing.TB, pathSrc string, pathExpected string, fmtedContent []byte) {
	expectedContent := readTestFile(tb, pathExpected)

	if bytes.Equal(expectedContent, fmtedContent) {
		return
	}

	pathFmted := appendToName(pathExpected, FMTED_SUFFIX)
	writeTestFile(tb, pathFmted, fmtedContent)

	tb.Fatalf(strings.TrimSpace(`
format mismatch
source:          %q
expected output: %q
actual output:   %q
`),
		testFilePath(pathSrc),
		testFilePath(pathExpected),
		testFilePath(pathFmted))
}

func deleteTestFiles(pattern string) error {
	matches, err := filepath.Glob(testFilePath(pattern))
	if err != nil {
		panic(fmt.Errorf("failed to find files by pattern %q: %w", pattern, err))
	}

	for _, path := range matches {
		err := os.Remove(path)
		if err != nil {
			panic(fmt.Errorf("failed to delete %q: %w", path, err))
		}
	}

	return nil
}

func readTestFile(tb testing.TB, name string) []byte {
	path := testFilePath(name)
	content, err := ioutil.ReadFile(path)
	if err != nil {
		tb.Fatalf("failed to read test file at %q: %+v", path, err)
	}
	return content
}

func writeTestFile(tb testing.TB, name string, content []byte) {
	path := testFilePath(name)
	err := ioutil.WriteFile(path, content, os.ModePerm)
	if err != nil {
		tb.Fatalf("failed to write %q: %+v", path, err)
	}
}

func testFilePath(name string) string {
	return filepath.Join(DIR_TEST_FILES, name)
}

func appendToName(path string, suffix string) string {
	dir, base, ext := splitPath(path)
	return dir + base + suffix + ext
}

func splitPath(path string) (string, string, string) {
	dir, file := filepath.Split(path)

	ext := filepath.Ext(file)
	base := strings.TrimSuffix(file, ext)

	if base == "" && ext != "" {
		return dir, ext, ""
	}

	return dir, base, ext
}
