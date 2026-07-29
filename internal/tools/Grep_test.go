package tools_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"looporbit/internal/tools"
)

func TestGrepFilename(t *testing.T) {
	allfile, err := tools.Grep("filename", ".", "go")
	if err != nil {
		t.Fatalf("Grep 搜索文件错误: %v", err)
	}
	if len(allfile) == 0 {
		t.Fatal("没有找到文件")
	}
}

func TestGrepContentRegex(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "first.txt"), "alpha 123\nbeta\nalpha 456\n")
	writeTestFile(t, filepath.Join(root, "binary.dat"), []byte("alpha 000\x00data\n"))
	if err := os.Mkdir(filepath.Join(root, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "nested", "second.txt"), "alpha 789\n")

	got, err := tools.Grep("content", root, `alpha \d+`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"first.txt:1:alpha 123",
		"first.txt:3:alpha 456",
		filepath.Join("nested", "second.txt") + ":1:alpha 789",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Grep content = %#v, want %#v", got, want)
	}
}

func TestGrepContentReturnsRegexError(t *testing.T) {
	if _, err := tools.Grep("content", t.TempDir(), "["); err == nil {
		t.Fatal("invalid regular expression did not return an error")
	}
}

func TestGrepContentSearchesSingleFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "single.txt")
	writeTestFile(t, path, "skip\nmatch-42\n")

	got, err := tools.Grep("content", path, `match-\d+`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"single.txt:2:match-42"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Grep content = %#v, want %#v", got, want)
	}
}

func TestGrepRejectsEmptyContent(t *testing.T) {
	for _, searchType := range []string{"filename", "content"} {
		t.Run(searchType, func(t *testing.T) {
			if _, err := tools.Grep(searchType, t.TempDir(), ""); err == nil {
				t.Fatal("empty content did not return an error")
			}
		})
	}
}

func TestCallGrepFuncRejectsMissingContent(t *testing.T) {
	arguments, err := json.Marshal(map[string]string{
		"type": "content",
		"path": t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = tools.CallGrepFunc([]any{map[string]any{
		"arguments": string(arguments),
	}})
	if err == nil {
		t.Fatal("missing content did not return an error")
	}
}

func TestGrepContentKeepsMatchesAndReportsLongLine(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "a-large.txt"), strings.Repeat("x", 1024*1024+1))
	writeTestFile(t, filepath.Join(root, "z-good.txt"), "needle\n")

	got, err := tools.Grep("content", root, "needle")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("Grep content = %#v, want one match and one warning", got)
	}
	if got[0] != "z-good.txt:1:needle" {
		t.Fatalf("first result = %q, want good file match", got[0])
	}
	if !strings.HasPrefix(got[1], "[Warning] a-large.txt:") {
		t.Fatalf("second result = %q, want warning for a-large.txt", got[1])
	}
}

func writeTestFile(t *testing.T, path string, content any) {
	t.Helper()
	var data []byte
	switch value := content.(type) {
	case string:
		data = []byte(value)
	case []byte:
		data = value
	default:
		t.Fatalf("unsupported test content type %T", content)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
