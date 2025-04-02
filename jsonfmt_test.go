package jsonfmt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const (
	DIR_TESTDATA        = `testdata`
	FMTED_SUFFIX        = `_fmted`
	STD_COMPATIBLE_FILE = `inp_long_pure.json`
)

func Benchmark_json_Indent(b *testing.B) {
	content := readTestFile(b, STD_COMPATIBLE_FILE)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		try(json.Indent(&buf, content, ``, `  `))
	}
}

func BenchmarkFormat(b *testing.B) {
	content := readTestFile(b, STD_COMPATIBLE_FILE)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = FormatBytes(Default, content)
	}
}

func TestMain(m *testing.M) {
	try(deleteTestFiles(`*` + FMTED_SUFFIX + `.*`))

	code := m.Run()
	if code == 0 {
		try(deleteTestFiles(`*` + FMTED_SUFFIX + `.*`))
	}

	os.Exit(code)
}

// Sanity check for the test itself.
func Test_json_Indent(t *testing.T) {
	const src = STD_COMPATIBLE_FILE
	content := readTestFile(t, src)

	var buf bytes.Buffer
	try(json.Indent(&buf, content, ``, `  `))

	eqFile(t, src, `out_long_multi.json`, buf.Bytes())
}

func TestFormat_hybrid(t *testing.T) {
	conf := Default
	conf.TrailingComma = true

	const src = `inp_long_comments.json`
	input := readTestFile(t, src)
	output := FormatBytes(conf, input)
	eqFile(t, src, `out_long_hybrid_commas_comments.json`, output)
}

func TestFormat_hybrid_strip_comments(t *testing.T) {
	conf := Default
	conf.TrailingComma = true
	conf.StripComments = true

	const src = `inp_long_comments.json`
	input := readTestFile(t, src)
	output := FormatBytes(conf, input)
	eqFile(t, src, `out_long_hybrid_commas.json`, output)
}

func TestFormat_insert_punctuation(t *testing.T) {
	conf := Default
	conf.TrailingComma = true

	const src = `inp_short_nopunc.json`
	input := readTestFile(t, src)
	output := FormatBytes(conf, input)
	eqFile(t, src, `out_short_punc.json`, output)
}

func TestFormat_single_line_with_comments(t *testing.T) {
	conf := Default
	conf.Indent = ``
	conf.StripComments = false

	const src = `inp_long_comments.json`
	input := readTestFile(t, src)
	output := FormatBytes(conf, input)
	eqFile(t, src, `out_long_single_comments.json`, output)
}

func TestFormat_single_line_strip_comments(t *testing.T) {
	conf := Default
	conf.Indent = ``
	conf.StripComments = true

	const src = `inp_long_comments.json`
	input := readTestFile(t, src)
	output := FormatBytes(conf, input)
	eqFile(t, src, `out_long_single_stripped.json`, output)
}

// TODO consider desired single-line behavior.
func TestFormat_block_comment_multi_line(t *testing.T) {
	conf := Default

	const src = `inp_comment_block.json`
	input := readTestFile(t, src)
	output := FormatBytes(conf, input)
	eqFile(t, src, `out_comment_block_multi_line.json`, output)
}

// TODO consider desired single-line behavior.
func TestFormat_block_comment_multi_line_nested(t *testing.T) {
	conf := Default
	conf.Width = 0

	const src = `inp_comment_block_nested.json`
	input := readTestFile(t, src)
	output := FormatBytes(conf, input)
	eqFile(t, src, `out_comment_block_nested_multi_line.json`, output)
}

func TestFormat_json_lines(t *testing.T) {
	conf := Default
	conf.StripComments = true

	const src = `inp_lines.json`
	input := readTestFile(t, src)
	output := FormatBytes(conf, input)

	eqFile(t, src, `out_lines.json`, output)
}

// This used to hang forever.
func TestFormat_primitive(t *testing.T) {
	input := []byte(`0`)
	expected := []byte("0\n")
	fmted := FormatBytes(Default, input)

	if bytes.Equal(expected, fmted) {
		return
	}

	t.Fatalf(strings.TrimSpace(`
format mismatch
input:           %q
expected output: %q
actual output:   %q
`), input, expected, fmted)
}

func TestUnmarshal(t *testing.T) {
	type TarGlobal struct {
		CheckForUpdatesOnStartup bool `json:"check_for_updates_on_startup"`
		ShowInMenuBar            bool `json:"show_in_menu_bar"`
		ShowProfileNameInMenuBar bool `json:"show_profile_name_in_menu_bar"`
	}

	type TarProfile struct {
		// Fields elided for simplicity.
	}

	type Tar struct {
		Global   TarGlobal    `json:"global"`
		Profiles []TarProfile `json:"profiles"`
	}

	var tar Tar
	try(Unmarshal(readTestFile(t, `inp_short_nopunc.json`), &tar))

	eq(t, tar, Tar{
		Global:   TarGlobal{CheckForUpdatesOnStartup: true},
		Profiles: []TarProfile{{}},
	})
}

func eq(t testing.TB, exp, act interface{}) {
	if !reflect.DeepEqual(exp, act) {
		t.Fatalf(`
expected (detailed):
	%#[1]v
actual (detailed):
	%#[2]v
expected (simple):
	%[1]v
actual (simple):
	%[2]v
`, exp, act)
	}
}

func eqFile(t testing.TB, pathSrc string, pathExpected string, fmtedContent []byte) {
	expectedContent := readTestFile(t, pathExpected)

	if bytes.Equal(expectedContent, fmtedContent) {
		return
	}

	pathFmted := appendToName(pathExpected, FMTED_SUFFIX)
	writeTestFile(t, pathFmted, fmtedContent)

	t.Fatalf(strings.TrimSpace(`
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
		panic(fmt.Errorf(`failed to find files by pattern %q: %w`, pattern, err))
	}

	for _, path := range matches {
		err := os.Remove(path)
		if err != nil {
			panic(fmt.Errorf(`failed to delete %q: %w`, path, err))
		}
	}

	return nil
}

func readTestFile(t testing.TB, name string) []byte {
	path := testFilePath(name)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf(`failed to read test file at %q: %+v`, path, err)
	}
	return content
}

func writeTestFile(t testing.TB, name string, content []byte) {
	path := testFilePath(name)
	err := os.WriteFile(path, content, os.ModePerm)
	if err != nil {
		t.Fatalf(`failed to write %q: %+v`, path, err)
	}
}

func testFilePath(name string) string {
	return filepath.Join(DIR_TESTDATA, name)
}

func appendToName(path string, suffix string) string {
	dir, base, ext := splitPath(path)
	return dir + base + suffix + ext
}

func splitPath(path string) (string, string, string) {
	dir, file := filepath.Split(path)

	ext := filepath.Ext(file)
	base := strings.TrimSuffix(file, ext)

	if base == `` && ext != `` {
		return dir, ext, ``
	}

	return dir, base, ext
}

func try(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, `%+v`, err)
		os.Exit(1)
	}
}
