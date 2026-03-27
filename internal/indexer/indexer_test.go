package indexer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"luck-mpc/internal/repository"
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

func TestExtractSignals(t *testing.T) {
	content := `
import "github.com/acme/auth"
source = "terraform-aws-modules/vpc/aws"
const endpoint = "/api/v1/login"
const url = "https://api.example.com/v1/users"
DATABASE_URL=postgres://local
`
	signals := extractSignals("docs/auth/README.md", content)
	if len(signals) == 0 {
		t.Fatalf("expected signals")
	}

	has := func(signalType, normalized string) bool {
		for _, signal := range signals {
			if signal.SignalType == signalType && signal.NormalizedValue == normalized {
				return true
			}
		}
		return false
	}

	if !has("import_ref", "github.com/acme/auth") {
		t.Fatalf("expected import_ref signal in %+v", signals)
	}
	if !has("terraform_source", "terraform-aws-modules/vpc/aws") {
		t.Fatalf("expected terraform_source signal in %+v", signals)
	}
	if !has("endpoint", "/api/v1/login") {
		t.Fatalf("expected endpoint signal in %+v", signals)
	}
	if !has("url", "https://api.example.com/v1/users") {
		t.Fatalf("expected url signal in %+v", signals)
	}
	if !has("env_var", "database_url") {
		t.Fatalf("expected env_var signal in %+v", signals)
	}
	if !has("path_hint", "docs/auth/readme.md") {
		t.Fatalf("expected path_hint signal in %+v", signals)
	}
}

func TestExtractSignals_StackSpecific(t *testing.T) {
	content := `
package auth
func LoginUser() {}
type AuthService struct{}

class AuthClient:
    pass
def load_user():
    return None

export function createSession() {}

resource "aws_s3_bucket" "logs" {}
module "network" {}

# Authentication Overview
`
	signals := extractSignals("infra/auth/main.tf", content)

	has := func(signalType, normalized string) bool {
		for _, signal := range signals {
			if signal.SignalType == signalType && signal.NormalizedValue == normalized {
				return true
			}
		}
		return false
	}

	if !has("go_package", "auth") {
		t.Fatalf("expected go_package signal in %+v", signals)
	}
	if !has("go_func", "loginuser") {
		t.Fatalf("expected go_func signal in %+v", signals)
	}
	if !has("go_type", "authservice") {
		t.Fatalf("expected go_type signal in %+v", signals)
	}
	if !has("py_class", "authclient") {
		t.Fatalf("expected py_class signal in %+v", signals)
	}
	if !has("py_def", "load_user") {
		t.Fatalf("expected py_def signal in %+v", signals)
	}
	if !has("js_symbol", "createsession") {
		t.Fatalf("expected js_symbol signal in %+v", signals)
	}
	if !has("tf_resource", "aws_s3_bucket:logs") {
		t.Fatalf("expected tf_resource signal in %+v", signals)
	}
	if !has("tf_module", "network") {
		t.Fatalf("expected tf_module signal in %+v", signals)
	}
	if !has("doc_heading", "authentication overview") {
		t.Fatalf("expected doc_heading signal in %+v", signals)
	}
}

func TestExtractSignals_ReactAnsibleAndHTTP(t *testing.T) {
	reactContent := `
import React from "react"
import { Route } from "react-router-dom"

export function AuthPage() {
  const data = fetch("/api/v1/session")
  return <Route path="/auth/login" element={<LoginPage />} />
}

const LoginPage = () => null

function useSession() {}

router.get("/internal/health", handler)
axios.post("/api/v1/login")
`
	reactSignals := extractSignals("frontend/src/AuthPage.tsx", reactContent)

	has := func(signals []repository.FileSignalInput, signalType, normalized string) bool {
		for _, signal := range signals {
			if signal.SignalType == signalType && signal.NormalizedValue == normalized {
				return true
			}
		}
		return false
	}

	if !has(reactSignals, "react_component", "authpage") {
		t.Fatalf("expected react_component authpage in %+v", reactSignals)
	}
	if !has(reactSignals, "react_component", "loginpage") {
		t.Fatalf("expected react_component loginpage in %+v", reactSignals)
	}
	if !has(reactSignals, "react_hook", "usesession") {
		t.Fatalf("expected react_hook usesession in %+v", reactSignals)
	}
	if !has(reactSignals, "route_path", "/auth/login") {
		t.Fatalf("expected route_path in %+v", reactSignals)
	}
	if !has(reactSignals, "http_route", "get /internal/health") {
		t.Fatalf("expected http_route in %+v", reactSignals)
	}
	if !has(reactSignals, "http_client_call", "post /api/v1/login") {
		t.Fatalf("expected http_client_call post in %+v", reactSignals)
	}
	if !has(reactSignals, "http_client_call", "fetch /api/v1/session") {
		t.Fatalf("expected http_client_call fetch in %+v", reactSignals)
	}

	ansibleContent := `
- hosts: app
  tasks:
    - name: Copy config
      ansible.builtin.copy:
        src: app.conf
        dest: /etc/app.conf
    - role: common
`
	ansibleSignals := extractSignals("infra/ansible/playbook.yml", ansibleContent)
	if !has(ansibleSignals, "ansible_hosts", "app") {
		t.Fatalf("expected ansible_hosts in %+v", ansibleSignals)
	}
	if !has(ansibleSignals, "ansible_module", "ansible.builtin.copy") {
		t.Fatalf("expected ansible_module in %+v", ansibleSignals)
	}
	if !has(ansibleSignals, "ansible_role", "common") {
		t.Fatalf("expected ansible_role in %+v", ansibleSignals)
	}

	openAPIContent := `
/v1/users:
  get:
    description: list users
`
	openAPISignals := extractSignals("docs/openapi.yaml", openAPIContent)
	if !has(openAPISignals, "openapi_path", "/v1/users") {
		t.Fatalf("expected openapi_path in %+v", openAPISignals)
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

func TestShouldReindexIndexedFile(t *testing.T) {
	valid := repository.IndexedFile{
		Path:        "lambda_function.py",
		ContentHash: "hash1",
		Language:    "python",
		FileType:    "code",
		SizeBytes:   1234,
		ChunkCount:  4,
		Status:      "indexed",
	}
	if shouldReindexIndexedFile("lambda_function.py", valid) {
		t.Fatalf("expected valid indexed file to be reused")
	}

	stale := valid
	stale.FileType = "unknown"
	stale.Language = "text"
	stale.SizeBytes = 0
	stale.ChunkCount = 0
	if !shouldReindexIndexedFile("lambda_function.py", stale) {
		t.Fatalf("expected stale metadata to force reindex")
	}
}
