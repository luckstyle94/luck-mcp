package indexer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSplitContentIntoChunks(t *testing.T) {
	content := strings.Repeat("a", 4000)
	chunks, err := splitContentIntoChunks(content, 1000, 200)
	if err != nil {
		t.Fatalf("splitContentIntoChunks returned error: %v", err)
	}
	if len(chunks) < 4 {
		t.Fatalf("expected at least 4 chunks, got %d", len(chunks))
	}
	for i, c := range chunks {
		if strings.TrimSpace(c) == "" {
			t.Fatalf("chunk %d should not be empty", i)
		}
		if len(c) > 1000 {
			t.Fatalf("chunk %d too large: %d", i, len(c))
		}
	}
}

func TestSplitContentIntoChunks_UTF8Safe(t *testing.T) {
	content := strings.Repeat("A—Bç", 1200)
	chunks, err := splitContentIntoChunks(content, 200, 50)
	if err != nil {
		t.Fatalf("splitContentIntoChunks returned error: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatalf("expected non-empty chunks")
	}
	for i, c := range chunks {
		if !utf8.ValidString(c) {
			t.Fatalf("chunk %d is not valid utf8", i)
		}
	}
}

func TestShouldSkipFile(t *testing.T) {
	if !shouldSkipFile(".env", 10, 1024) {
		t.Fatalf("expected .env to be skipped")
	}
	if !shouldSkipFile("assets/logo.png", 10, 1024) {
		t.Fatalf("expected png to be skipped")
	}
	if !shouldSkipFile("main.go", 2048, 1024) {
		t.Fatalf("expected file above max size to be skipped")
	}
	if shouldSkipFile("internal/service/auth.py", 200, 1024) {
		t.Fatalf("expected auth.py to be indexed")
	}
}

func TestBuildAutoTags(t *testing.T) {
	tags := buildAutoTags("frontend/src/App.tsx")
	if len(tags) == 0 {
		t.Fatalf("expected tags")
	}
	hasSentinel := false
	hasReact := false
	for _, tag := range tags {
		if tag == "_auto_index" {
			hasSentinel = true
		}
		if tag == "react" {
			hasReact = true
		}
	}
	if !hasSentinel {
		t.Fatalf("expected _auto_index tag in %v", tags)
	}
	if !hasReact {
		t.Fatalf("expected react tag in %v", tags)
	}
}

func TestValidateOptions(t *testing.T) {
	dir := t.TempDir()
	opts := normalizeOptions(Options{Project: "proj", RootPath: dir, Mode: "changed", ChunkSize: 1000, ChunkOverlap: 200, MaxFileBytes: 1024})
	if err := validateOptions(opts); err != nil {
		t.Fatalf("validateOptions returned error: %v", err)
	}

	if err := validateOptions(Options{Project: "", RootPath: dir, Mode: "changed"}); err == nil {
		t.Fatalf("expected error when project is empty")
	}

	missingDir := filepath.Join(dir, "does-not-exist")
	if err := validateOptions(Options{Project: "proj", RootPath: missingDir, Mode: "changed"}); err == nil {
		t.Fatalf("expected error for invalid root path")
	}

	filePath := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}
	if err := validateOptions(Options{Project: "proj", RootPath: filePath, Mode: "changed"}); err == nil {
		t.Fatalf("expected error when root path is file")
	}
}

func TestSanitizeToValidUTF8(t *testing.T) {
	// 0x94 is invalid as standalone UTF-8 byte (common cp1252 artifact).
	raw := []byte{'o', 'k', ' ', 0x94, ' ', 'x'}

	sanitized, invalidCount := sanitizeToValidUTF8(raw)
	if invalidCount == 0 {
		t.Fatalf("expected invalid bytes count > 0")
	}
	if !utf8.ValidString(sanitized) {
		t.Fatalf("expected sanitized string to be valid utf8")
	}
	if strings.ContainsRune(sanitized, rune(0x94)) {
		t.Fatalf("expected invalid byte to be replaced, got %q", sanitized)
	}
}

func TestSanitizeToValidUTF8_AlreadyValid(t *testing.T) {
	raw := []byte("decisao de arquitetura")
	sanitized, invalidCount := sanitizeToValidUTF8(raw)
	if invalidCount != 0 {
		t.Fatalf("expected invalidCount 0, got %d", invalidCount)
	}
	if sanitized != "decisao de arquitetura" {
		t.Fatalf("unexpected sanitized value: %q", sanitized)
	}
}
