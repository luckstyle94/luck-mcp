package indexer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"luck-mcp/internal/embeddings"
	"luck-mcp/internal/repository"
)

var (
	errInvalidOptions = errors.New("invalid index options")

	skipDirs = map[string]struct{}{
		".git":          {},
		".svn":          {},
		".hg":           {},
		"node_modules":  {},
		"vendor":        {},
		"dist":          {},
		"build":         {},
		"out":           {},
		"target":        {},
		".next":         {},
		".nuxt":         {},
		".cache":        {},
		".terraform":    {},
		".terragrunt":   {},
		".venv":         {},
		"venv":          {},
		"__pycache__":   {},
		".mypy_cache":   {},
		".pytest_cache": {},
		".idea":         {},
		".vscode":       {},
	}

	skipExt = map[string]struct{}{
		".png":   {},
		".jpg":   {},
		".jpeg":  {},
		".gif":   {},
		".webp":  {},
		".svg":   {},
		".ico":   {},
		".pdf":   {},
		".zip":   {},
		".tar":   {},
		".gz":    {},
		".tgz":   {},
		".7z":    {},
		".rar":   {},
		".mp3":   {},
		".mp4":   {},
		".mov":   {},
		".avi":   {},
		".mkv":   {},
		".wasm":  {},
		".exe":   {},
		".dll":   {},
		".so":    {},
		".dylib": {},
		".ttf":   {},
		".woff":  {},
		".woff2": {},
		".eot":   {},
		".otf":   {},
		".lock":  {},
	}
)

type Options struct {
	Project      string
	RootPath     string
	Mode         string
	ChunkSize    int
	ChunkOverlap int
	MaxFileBytes int64
}

type Result struct {
	ScannedFiles   int
	SkippedFiles   int
	IndexedFiles   int
	UnchangedFiles int
	DeletedFiles   int
	ChunksAdded    int
	FailedFiles    int
}

type Service struct {
	repo              repository.DocumentRepository
	embeddings        embeddings.Client
	expectedEmbedding int
	logger            *slog.Logger
}

type sourceFile struct {
	Path     string
	Hash     string
	Content  string
	Tags     []string
	Signals  []repository.FileSignalInput
	Warnings []string
}

var (
	urlPattern             = regexp.MustCompile(`https?://[^\s"'` + "`" + `<>]+`)
	endpointPattern        = regexp.MustCompile(`(?:/api/[A-Za-z0-9_./:-]+|/v[0-9]+/[A-Za-z0-9_./:-]+)`)
	importQuotedPattern    = regexp.MustCompile(`(?m)(?:^|\s)(?:import|from|require|include)\s*(?:\(|)?\s*["']([^"']+)["']`)
	terraformSourcePattern = regexp.MustCompile(`(?m)source\s*=\s*"([^"]+)"`)
	envVarPattern          = regexp.MustCompile(`\b[A-Z][A-Z0-9_]{2,}\b`)
	goPackagePattern       = regexp.MustCompile(`(?m)^package\s+([A-Za-z_][A-Za-z0-9_]*)`)
	goFuncPattern          = regexp.MustCompile(`(?m)^func\s+(?:\([^)]+\)\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	goTypePattern          = regexp.MustCompile(`(?m)^type\s+([A-Za-z_][A-Za-z0-9_]*)\s+(?:struct|interface|map|\[\]|chan|func|\*)`)
	pythonDefPattern       = regexp.MustCompile(`(?m)^def\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	pythonClassPattern     = regexp.MustCompile(`(?m)^class\s+([A-Za-z_][A-Za-z0-9_]*)`)
	jsSymbolPattern        = regexp.MustCompile(`(?m)^(?:export\s+)?(?:async\s+)?(?:function|class|const|let|var)\s+([A-Za-z_$][A-Za-z0-9_$]*)`)
	reactFunctionPattern   = regexp.MustCompile(`(?m)^(?:export\s+(?:default\s+)?)?function\s+([A-Z][A-Za-z0-9_]*)\s*\(`)
	reactConstPattern      = regexp.MustCompile(`(?m)^(?:export\s+(?:default\s+)?)?(?:const|let|var)\s+([A-Z][A-Za-z0-9_]*)\s*=\s*(?:\([^=]*\)\s*)?=>`)
	reactHookPattern       = regexp.MustCompile(`\b(use[A-Z][A-Za-z0-9_]*)\b`)
	reactRoutePattern      = regexp.MustCompile(`(?i)<Route[^>]+path=["']([^"']+)["']`)
	routerMethodPattern    = regexp.MustCompile(`(?i)\b(?:router|app)\.(get|post|put|patch|delete|options|head)\(\s*["']([^"']+)["']`)
	pythonRoutePattern     = regexp.MustCompile(`(?m)@\w+\.(get|post|put|patch|delete)\(\s*["']([^"']+)["']`)
	axiosMethodPattern     = regexp.MustCompile(`(?i)\baxios\.(get|post|put|patch|delete|options|head)\(\s*["']([^"']+)["']`)
	fetchPattern           = regexp.MustCompile(`(?i)\bfetch\(\s*["']([^"']+)["']`)
	openAPIPathPattern     = regexp.MustCompile(`(?m)^\s{0,4}(/[^:\s]+):\s*$`)
	ansibleRolePattern     = regexp.MustCompile(`(?m)^\s*-\s*(?:role|import_role|include_role):\s*([A-Za-z0-9_.-]+)\s*$`)
	ansibleModulePattern   = regexp.MustCompile(`(?m)^\s{2,}([a-z_][a-z0-9_.]+):\s*$`)
	ansibleHostsPattern    = regexp.MustCompile(`(?m)^\s*-\s*hosts:\s*([A-Za-z0-9_.,:-]+)\s*$`)
	terraformBlockPattern  = regexp.MustCompile(`(?m)^(resource|data|module|variable|output|provider)\s+"([^"]+)"(?:\s+"([^"]+)")?`)
	markdownHeadingPattern = regexp.MustCompile(`(?m)^#{1,6}\s+(.+)$`)
)

func NewService(repo repository.DocumentRepository, emb embeddings.Client, expectedEmbedding int, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		repo:              repo,
		embeddings:        emb,
		expectedEmbedding: expectedEmbedding,
		logger:            logger,
	}
}

func (s *Service) IndexProject(ctx context.Context, opts Options) (Result, error) {
	opts = normalizeOptions(opts)
	if err := validateOptions(opts); err != nil {
		return Result{}, err
	}

	rootPath := opts.RootPath
	repoRecord, err := s.repo.EnsureRepo(ctx, opts.Project, &rootPath)
	if err != nil {
		return Result{}, fmt.Errorf("ensure repo: %w", err)
	}

	knownFiles, err := s.repo.ListIndexedFiles(ctx, repoRecord.ID)
	if err != nil {
		return Result{}, fmt.Errorf("list indexed files: %w", err)
	}
	indexedByPath := make(map[string]repository.IndexedFile, len(knownFiles))
	for _, f := range knownFiles {
		indexedByPath[f.Path] = f
	}

	currentHashes := make(map[string]string)
	toIndex := make([]sourceFile, 0)
	result := Result{}

	err = filepath.WalkDir(opts.RootPath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.Type()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if shouldSkipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		result.ScannedFiles++
		relPath, err := filepath.Rel(opts.RootPath, path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)

		fileInfo, err := d.Info()
		if err != nil {
			return err
		}
		if shouldSkipFile(relPath, fileInfo.Size(), opts.MaxFileBytes) {
			result.SkippedFiles++
			return nil
		}

		contentBytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !isLikelyText(contentBytes) {
			result.SkippedFiles++
			return nil
		}

		safeContent, replacedInvalid := sanitizeToValidUTF8(contentBytes)
		if strings.TrimSpace(safeContent) == "" {
			result.SkippedFiles++
			s.logger.Warn("skipping file with empty content after utf8 sanitization",
				slog.String("project", opts.Project),
				slog.String("path", relPath),
				slog.Int("size", len(contentBytes)),
			)
			return nil
		}

		fileHash := hashBytes(contentBytes)
		currentHashes[relPath] = fileHash

		if opts.Mode == "changed" {
			if oldFile, ok := indexedByPath[relPath]; ok && oldFile.ContentHash == fileHash && !shouldReindexIndexedFile(relPath, oldFile) {
				result.UnchangedFiles++
				return nil
			}
		}

		warnings := make([]string, 0, 1)
		if replacedInvalid > 0 {
			warnings = append(warnings, fmt.Sprintf("sanitized_utf8_invalid_bytes=%d", replacedInvalid))
		}

		toIndex = append(toIndex, sourceFile{
			Path:     relPath,
			Hash:     fileHash,
			Content:  safeContent,
			Tags:     buildAutoTags(relPath),
			Signals:  extractSignals(relPath, safeContent),
			Warnings: warnings,
		})
		return nil
	})
	if err != nil {
		return result, fmt.Errorf("scan project files: %w", err)
	}

	deleted := s.removeMissingFiles(ctx, repoRecord.ID, opts.Project, indexedByPath, currentHashes)
	result.DeletedFiles = deleted

	sort.Slice(toIndex, func(i, j int) bool {
		return toIndex[i].Path < toIndex[j].Path
	})

	var firstErr error
	for _, file := range toIndex {
		chunks, err := splitContentIntoChunks(file.Content, opts.ChunkSize, opts.ChunkOverlap)
		if err != nil {
			result.FailedFiles++
			msg := err.Error()
			_ = s.repo.UpsertIndexedFile(ctx, repository.UpsertIndexedFileInput{
				Project:     opts.Project,
				RepoID:      repoRecord.ID,
				Path:        file.Path,
				ContentHash: file.Hash,
				Language:    languageFromExt(strings.ToLower(filepath.Ext(file.Path))),
				FileType:    fileTypeFromPath(file.Path),
				SizeBytes:   int64(len(file.Content)),
				ChunkCount:  0,
				Status:      "error",
				Error:       &msg,
			})
			if firstErr == nil {
				firstErr = fmt.Errorf("split chunks for %s: %w", file.Path, err)
			}
			continue
		}

		deletedRows, err := s.repo.DeleteIndexedChunksByPath(ctx, repoRecord.ID, file.Path)
		if err != nil {
			result.FailedFiles++
			msg := err.Error()
			_ = s.repo.UpsertIndexedFile(ctx, repository.UpsertIndexedFileInput{
				Project:     opts.Project,
				RepoID:      repoRecord.ID,
				Path:        file.Path,
				ContentHash: file.Hash,
				Language:    languageFromExt(strings.ToLower(filepath.Ext(file.Path))),
				FileType:    fileTypeFromPath(file.Path),
				SizeBytes:   int64(len(file.Content)),
				ChunkCount:  0,
				Status:      "error",
				Error:       &msg,
			})
			if firstErr == nil {
				firstErr = fmt.Errorf("delete previous chunks for %s: %w", file.Path, err)
			}
			continue
		}

		insertedChunks := 0
		fileFailed := false
		for idx, chunk := range chunks {
			if !utf8.ValidString(chunk) {
				chunk = strings.ToValidUTF8(chunk, " ")
				file.Warnings = append(file.Warnings, fmt.Sprintf("chunk_%d_sanitized_invalid_utf8", idx))
			}
			chunk = strings.TrimSpace(chunk)
			if chunk == "" {
				file.Warnings = append(file.Warnings, fmt.Sprintf("chunk_%d_dropped_empty_after_sanitize", idx))
				continue
			}

			embedding, err := s.embeddings.Embed(ctx, chunk)
			if err != nil {
				result.FailedFiles++
				fileFailed = true
				msg := fmt.Sprintf("embedding failed on chunk %d: %v", idx, err)
				_ = s.repo.UpsertIndexedFile(ctx, repository.UpsertIndexedFileInput{
					Project:     opts.Project,
					RepoID:      repoRecord.ID,
					Path:        file.Path,
					ContentHash: file.Hash,
					Language:    languageFromExt(strings.ToLower(filepath.Ext(file.Path))),
					FileType:    fileTypeFromPath(file.Path),
					SizeBytes:   int64(len(file.Content)),
					ChunkCount:  insertedChunks,
					Status:      "error",
					Error:       &msg,
				})
				if firstErr == nil {
					firstErr = fmt.Errorf("embed chunk for %s: %w", file.Path, err)
				}
				break
			}
			if len(embedding) != s.expectedEmbedding {
				result.FailedFiles++
				fileFailed = true
				msg := fmt.Sprintf("invalid embedding size %d (expected %d)", len(embedding), s.expectedEmbedding)
				_ = s.repo.UpsertIndexedFile(ctx, repository.UpsertIndexedFileInput{
					Project:     opts.Project,
					RepoID:      repoRecord.ID,
					Path:        file.Path,
					ContentHash: file.Hash,
					Language:    languageFromExt(strings.ToLower(filepath.Ext(file.Path))),
					FileType:    fileTypeFromPath(file.Path),
					SizeBytes:   int64(len(file.Content)),
					ChunkCount:  insertedChunks,
					Status:      "error",
					Error:       &msg,
				})
				if firstErr == nil {
					firstErr = fmt.Errorf("invalid embedding size for %s", file.Path)
				}
				break
			}

			chunkHash := hashString(strings.Join([]string{opts.Project, file.Path, strconv.Itoa(idx), chunk}, "\n"))
			_, err = s.repo.InsertIndexedChunkWithEmbedding(ctx, repository.AddIndexedChunkInput{
				RepoID:      repoRecord.ID,
				Path:        file.Path,
				ChunkIndex:  idx,
				Tags:        file.Tags,
				Content:     chunk,
				ContentHash: chunkHash,
				Embedding:   embedding,
			})
			if err != nil {
				result.FailedFiles++
				fileFailed = true
				msg := fmt.Sprintf("insert chunk %d failed: %v", idx, err)
				_ = s.repo.UpsertIndexedFile(ctx, repository.UpsertIndexedFileInput{
					Project:     opts.Project,
					RepoID:      repoRecord.ID,
					Path:        file.Path,
					ContentHash: file.Hash,
					Language:    languageFromExt(strings.ToLower(filepath.Ext(file.Path))),
					FileType:    fileTypeFromPath(file.Path),
					SizeBytes:   int64(len(file.Content)),
					ChunkCount:  insertedChunks,
					Status:      "error",
					Error:       &msg,
				})
				if firstErr == nil {
					firstErr = fmt.Errorf("insert chunk for %s: %w", file.Path, err)
				}
				break
			}
			insertedChunks++
		}

		if fileFailed {
			continue
		}

		if err := s.repo.ReplaceFileSignals(ctx, repoRecord.ID, file.Path, file.Signals); err != nil {
			result.FailedFiles++
			msg := fmt.Sprintf("replace file signals failed: %v", err)
			_ = s.repo.UpsertIndexedFile(ctx, repository.UpsertIndexedFileInput{
				Project:     opts.Project,
				RepoID:      repoRecord.ID,
				Path:        file.Path,
				ContentHash: file.Hash,
				Language:    languageFromExt(strings.ToLower(filepath.Ext(file.Path))),
				FileType:    fileTypeFromPath(file.Path),
				SizeBytes:   int64(len(file.Content)),
				ChunkCount:  insertedChunks,
				Status:      "error",
				Error:       &msg,
			})
			if firstErr == nil {
				firstErr = fmt.Errorf("replace signals for %s: %w", file.Path, err)
			}
			continue
		}

		if err := s.repo.UpsertIndexedFile(ctx, repository.UpsertIndexedFileInput{
			Project:     opts.Project,
			RepoID:      repoRecord.ID,
			Path:        file.Path,
			ContentHash: file.Hash,
			Language:    languageFromExt(strings.ToLower(filepath.Ext(file.Path))),
			FileType:    fileTypeFromPath(file.Path),
			SizeBytes:   int64(len(file.Content)),
			ChunkCount:  insertedChunks,
			Status:      "indexed",
			Error:       nil,
		}); err != nil {
			result.FailedFiles++
			if firstErr == nil {
				firstErr = fmt.Errorf("upsert indexed file %s: %w", file.Path, err)
			}
			continue
		}

		result.IndexedFiles++
		result.ChunksAdded += insertedChunks
		s.logger.Info("file indexed",
			slog.String("project", opts.Project),
			slog.String("path", file.Path),
			slog.Int("chunks", insertedChunks),
			slog.Int64("removed_chunks", deletedRows),
			slog.Int("tags_count", len(file.Tags)),
			slog.Int("size", len(file.Content)),
		)
		for _, warn := range file.Warnings {
			s.logger.Warn("file indexed with warning",
				slog.String("project", opts.Project),
				slog.String("path", file.Path),
				slog.String("warning", warn),
			)
		}
	}

	if firstErr != nil {
		return result, fmt.Errorf("index finished with failures (%d files): %w", result.FailedFiles, firstErr)
	}

	return result, nil
}

func (s *Service) removeMissingFiles(ctx context.Context, repoID int64, project string, indexedByPath map[string]repository.IndexedFile, currentHashes map[string]string) int {
	removed := 0
	paths := make([]string, 0)
	for path := range indexedByPath {
		if _, ok := currentHashes[path]; !ok {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)

	for _, path := range paths {
		deletedRows, err := s.repo.DeleteIndexedChunksByPath(ctx, repoID, path)
		if err != nil {
			s.logger.Warn("failed to delete chunks for missing file",
				slog.String("project", project),
				slog.String("path", path),
				slog.String("error", err.Error()),
			)
			continue
		}
		if err := s.repo.DeleteIndexedFile(ctx, repoID, path); err != nil {
			s.logger.Warn("failed to delete indexed_file row",
				slog.String("project", project),
				slog.String("path", path),
				slog.String("error", err.Error()),
			)
			continue
		}
		if err := s.repo.DeleteFileSignalsByPath(ctx, repoID, path); err != nil {
			s.logger.Warn("failed to delete file signals row",
				slog.String("project", project),
				slog.String("path", path),
				slog.String("error", err.Error()),
			)
			continue
		}
		removed++
		s.logger.Info("removed stale indexed file",
			slog.String("project", project),
			slog.String("path", path),
			slog.Int64("removed_chunks", deletedRows),
		)
	}
	return removed
}

func shouldReindexIndexedFile(path string, existing repository.IndexedFile) bool {
	if existing.Status != "indexed" {
		return true
	}
	if strings.TrimSpace(existing.ContentHash) == "" {
		return true
	}
	if existing.ChunkCount <= 0 {
		return true
	}
	if existing.SizeBytes <= 0 {
		return true
	}

	expectedType := fileTypeFromPath(path)
	if strings.TrimSpace(existing.FileType) == "" || existing.FileType == "unknown" || existing.FileType != expectedType {
		return true
	}

	expectedLanguage := languageFromExt(strings.ToLower(filepath.Ext(path)))
	if strings.TrimSpace(existing.Language) == "" {
		return true
	}
	if expectedLanguage != "text" && existing.Language != expectedLanguage {
		return true
	}

	return false
}

func normalizeOptions(opts Options) Options {
	opts.Project = strings.TrimSpace(opts.Project)
	opts.RootPath = strings.TrimSpace(opts.RootPath)
	opts.Mode = strings.ToLower(strings.TrimSpace(opts.Mode))
	if opts.Mode == "" {
		opts.Mode = "changed"
	}
	if opts.RootPath == "" {
		opts.RootPath = "."
	}
	if opts.ChunkSize <= 0 {
		opts.ChunkSize = 1600
	}
	if opts.ChunkOverlap < 0 {
		opts.ChunkOverlap = 0
	}
	if opts.ChunkOverlap >= opts.ChunkSize {
		opts.ChunkOverlap = opts.ChunkSize / 4
	}
	if opts.MaxFileBytes <= 0 {
		opts.MaxFileBytes = 1 << 20
	}
	return opts
}

func validateOptions(opts Options) error {
	if opts.Project == "" {
		return fmt.Errorf("%w: project is required", errInvalidOptions)
	}
	if opts.Mode != "changed" && opts.Mode != "full" {
		return fmt.Errorf("%w: mode must be changed or full", errInvalidOptions)
	}
	st, err := os.Stat(opts.RootPath)
	if err != nil {
		return fmt.Errorf("%w: root path is invalid: %v", errInvalidOptions, err)
	}
	if !st.IsDir() {
		return fmt.Errorf("%w: root path must be a directory", errInvalidOptions)
	}
	return nil
}

func shouldSkipDir(name string) bool {
	_, ok := skipDirs[strings.ToLower(strings.TrimSpace(name))]
	return ok
}

func shouldSkipFile(path string, size int64, maxFileBytes int64) bool {
	if size == 0 {
		return true
	}
	fileName := strings.ToLower(filepath.Base(path))
	if strings.HasPrefix(fileName, ".env") {
		return true
	}
	if strings.HasSuffix(fileName, ".pem") || strings.HasSuffix(fileName, ".key") || strings.HasSuffix(fileName, ".crt") || strings.HasSuffix(fileName, ".p12") || strings.HasSuffix(fileName, ".pfx") {
		return true
	}
	if strings.HasSuffix(fileName, ".min.js") || strings.HasSuffix(fileName, ".min.css") || strings.HasSuffix(fileName, ".map") {
		return true
	}
	ext := strings.ToLower(filepath.Ext(fileName))
	if _, blocked := skipExt[ext]; blocked {
		return true
	}
	return size > maxFileBytes
}

func isLikelyText(content []byte) bool {
	if len(content) == 0 {
		return false
	}
	if bytes.IndexByte(content, 0) >= 0 {
		return false
	}

	sample := content
	if len(sample) > 4096 {
		sample = sample[:4096]
	}
	invalid := 0
	total := 0
	for len(sample) > 0 {
		r, size := utf8.DecodeRune(sample)
		total++
		if r == utf8.RuneError && size == 1 {
			invalid++
		}
		sample = sample[size:]
	}
	if total == 0 {
		return false
	}
	return float64(invalid)/float64(total) < 0.1
}

func sanitizeToValidUTF8(content []byte) (string, int) {
	if len(content) == 0 {
		return "", 0
	}
	invalidBytes := countInvalidUTF8Bytes(content)
	if invalidBytes == 0 {
		return string(content), 0
	}
	return strings.ToValidUTF8(string(content), " "), invalidBytes
}

func countInvalidUTF8Bytes(content []byte) int {
	invalid := 0
	for len(content) > 0 {
		_, size := utf8.DecodeRune(content)
		if size == 1 && content[0] >= 0x80 && !utf8.Valid(content[:1]) {
			invalid++
		}
		content = content[size:]
	}
	return invalid
}

func splitContentIntoChunks(content string, size, overlap int) ([]string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, nil
	}
	if size <= 0 {
		return nil, errors.New("chunk size must be positive")
	}
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= size {
		return nil, errors.New("chunk overlap must be smaller than chunk size")
	}

	runes := []rune(content)
	if len(runes) <= size {
		return []string{content}, nil
	}

	step := size - overlap
	chunks := make([]string, 0, (len(runes)/step)+1)
	for start := 0; start < len(runes); start += step {
		end := start + size
		if end > len(runes) {
			end = len(runes)
		}
		chunk := strings.TrimSpace(string(runes[start:end]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		if end == len(runes) {
			break
		}
	}

	return chunks, nil
}

func buildAutoTags(path string) []string {
	ext := strings.ToLower(filepath.Ext(path))
	lang := languageFromExt(ext)
	seen := map[string]struct{}{}
	tags := make([]string, 0, 4)
	for _, tag := range []string{repository.AutoIndexTag, "auto", "index", lang} {
		tag = strings.TrimSpace(strings.ToLower(tag))
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags
}

func languageFromExt(ext string) string {
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".tf", ".tfvars", ".hcl":
		return "terraform"
	case ".yaml", ".yml":
		return "yaml"
	case ".ansible":
		return "ansible"
	case ".js", ".cjs", ".mjs":
		return "javascript"
	case ".ts", ".cts", ".mts":
		return "typescript"
	case ".jsx", ".tsx":
		return "react"
	case ".md", ".mdx":
		return "markdown"
	case ".sql":
		return "sql"
	case ".sh", ".bash", ".zsh":
		return "shell"
	case ".json":
		return "json"
	case ".xml":
		return "xml"
	case ".toml":
		return "toml"
	case ".ini", ".cfg", ".conf":
		return "config"
	default:
		return "text"
	}
}

func fileTypeFromPath(path string) string {
	base := strings.ToLower(filepath.Base(path))
	ext := strings.ToLower(filepath.Ext(path))

	switch {
	case strings.HasPrefix(path, "docs/"), strings.Contains(path, "/docs/"), strings.HasPrefix(base, "readme"), strings.Contains(base, "adr"):
		return "doc"
	case strings.HasSuffix(base, "_test.go"), strings.HasSuffix(base, "_test.py"), strings.HasSuffix(base, ".spec.ts"), strings.HasSuffix(base, ".spec.tsx"), strings.HasSuffix(base, ".test.ts"), strings.HasSuffix(base, ".test.tsx"), strings.HasSuffix(base, ".test.js"), strings.HasSuffix(base, ".test.jsx"):
		return "test"
	case ext == ".tf" || ext == ".tfvars" || ext == ".hcl" || base == "terragrunt.hcl" || strings.Contains(path, "/terraform/") || strings.Contains(path, "/ansible/"):
		return "infra"
	case ext == ".yaml" || ext == ".yml" || ext == ".json" || ext == ".toml" || ext == ".ini" || ext == ".cfg" || ext == ".conf":
		return "config"
	case ext == ".md" || ext == ".mdx":
		return "doc"
	default:
		return "code"
	}
}

func extractSignals(path, content string) []repository.FileSignalInput {
	if strings.TrimSpace(content) == "" {
		return nil
	}

	seen := make(map[string]struct{})
	out := make([]repository.FileSignalInput, 0, 24)
	add := func(signalType, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		normalized := strings.ToLower(value)
		key := signalType + "\x00" + normalized
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, repository.FileSignalInput{
			SignalType:      signalType,
			Value:           value,
			NormalizedValue: normalized,
		})
	}

	for _, match := range urlPattern.FindAllString(content, 12) {
		add("url", match)
	}
	for _, match := range endpointPattern.FindAllString(content, 12) {
		add("endpoint", match)
	}
	for _, match := range importQuotedPattern.FindAllStringSubmatch(content, 24) {
		if len(match) > 1 {
			add("import_ref", match[1])
		}
	}
	for _, match := range terraformSourcePattern.FindAllStringSubmatch(content, 12) {
		if len(match) > 1 {
			add("terraform_source", match[1])
		}
	}
	for _, match := range envVarPattern.FindAllString(content, 20) {
		add("env_var", match)
	}
	for _, match := range goPackagePattern.FindAllStringSubmatch(content, 4) {
		if len(match) > 1 {
			add("go_package", match[1])
		}
	}
	for _, match := range goFuncPattern.FindAllStringSubmatch(content, 16) {
		if len(match) > 1 {
			add("go_func", match[1])
		}
	}
	for _, match := range goTypePattern.FindAllStringSubmatch(content, 16) {
		if len(match) > 1 {
			add("go_type", match[1])
		}
	}
	for _, match := range pythonDefPattern.FindAllStringSubmatch(content, 16) {
		if len(match) > 1 {
			add("py_def", match[1])
		}
	}
	for _, match := range pythonClassPattern.FindAllStringSubmatch(content, 16) {
		if len(match) > 1 {
			add("py_class", match[1])
		}
	}
	for _, match := range jsSymbolPattern.FindAllStringSubmatch(content, 20) {
		if len(match) > 1 {
			add("js_symbol", match[1])
		}
	}
	if looksLikeReactFile(path, content) {
		for _, match := range reactFunctionPattern.FindAllStringSubmatch(content, 20) {
			if len(match) > 1 {
				add("react_component", match[1])
			}
		}
		for _, match := range reactConstPattern.FindAllStringSubmatch(content, 20) {
			if len(match) > 1 {
				add("react_component", match[1])
			}
		}
		for _, match := range reactHookPattern.FindAllStringSubmatch(content, 24) {
			if len(match) > 1 {
				add("react_hook", match[1])
			}
		}
		for _, match := range reactRoutePattern.FindAllStringSubmatch(content, 20) {
			if len(match) > 1 {
				add("route_path", match[1])
				add("endpoint", match[1])
			}
		}
	}
	for _, match := range routerMethodPattern.FindAllStringSubmatch(content, 24) {
		if len(match) > 2 {
			method := strings.ToUpper(match[1])
			pathValue := match[2]
			add("http_route", method+" "+pathValue)
			add("endpoint", pathValue)
		}
	}
	for _, match := range pythonRoutePattern.FindAllStringSubmatch(content, 24) {
		if len(match) > 2 {
			method := strings.ToUpper(match[1])
			pathValue := match[2]
			add("http_route", method+" "+pathValue)
			add("endpoint", pathValue)
		}
	}
	for _, match := range axiosMethodPattern.FindAllStringSubmatch(content, 24) {
		if len(match) > 2 {
			method := strings.ToUpper(match[1])
			pathValue := match[2]
			add("http_client_call", method+" "+pathValue)
			add("endpoint", pathValue)
		}
	}
	for _, match := range fetchPattern.FindAllStringSubmatch(content, 24) {
		if len(match) > 1 {
			add("http_client_call", "FETCH "+match[1])
			add("endpoint", match[1])
		}
	}
	for _, match := range openAPIPathPattern.FindAllStringSubmatch(content, 24) {
		if len(match) > 1 {
			add("openapi_path", match[1])
			add("endpoint", match[1])
		}
	}
	if looksLikeAnsibleFile(path, content) {
		for _, match := range ansibleRolePattern.FindAllStringSubmatch(content, 20) {
			if len(match) > 1 {
				add("ansible_role", match[1])
			}
		}
		for _, match := range ansibleHostsPattern.FindAllStringSubmatch(content, 12) {
			if len(match) > 1 {
				add("ansible_hosts", match[1])
			}
		}
		for _, match := range ansibleModulePattern.FindAllStringSubmatch(content, 32) {
			if len(match) > 1 && !isIgnoredAnsibleKey(match[1]) {
				add("ansible_module", match[1])
			}
		}
	}
	for _, match := range terraformBlockPattern.FindAllStringSubmatch(content, 16) {
		if len(match) > 2 {
			switch match[1] {
			case "resource":
				add("tf_resource", match[2]+":"+match[3])
			case "data":
				add("tf_data", match[2]+":"+match[3])
			case "module":
				add("tf_module", match[2])
			case "variable":
				add("tf_variable", match[2])
			case "output":
				add("tf_output", match[2])
			case "provider":
				add("tf_provider", match[2])
			}
		}
	}
	for _, match := range markdownHeadingPattern.FindAllStringSubmatch(content, 16) {
		if len(match) > 1 {
			add("doc_heading", match[1])
		}
	}

	add("path_hint", path)

	if len(out) == 0 {
		return nil
	}
	if len(out) > 80 {
		out = out[:80]
	}
	return out
}

func looksLikeReactFile(path, content string) bool {
	lowerPath := strings.ToLower(path)
	if strings.HasSuffix(lowerPath, ".tsx") || strings.HasSuffix(lowerPath, ".jsx") {
		return true
	}
	return strings.Contains(content, "react") || strings.Contains(content, "<Route")
}

func looksLikeAnsibleFile(path, content string) bool {
	lowerPath := strings.ToLower(path)
	if !(strings.HasSuffix(lowerPath, ".yml") || strings.HasSuffix(lowerPath, ".yaml")) {
		return false
	}
	if strings.Contains(lowerPath, "/ansible/") || strings.Contains(lowerPath, "/roles/") || strings.Contains(lowerPath, "playbook") {
		return true
	}
	return strings.Contains(content, "\nhosts:") || strings.Contains(content, "ansible.builtin.") || strings.Contains(content, "\n  tasks:")
}

func isIgnoredAnsibleKey(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "name", "hosts", "vars", "tasks", "handlers", "when", "tags", "block", "rescue", "always", "notify", "register", "environment", "become", "delegate_to", "loop", "with_items":
		return true
	default:
		return false
	}
}

func hashBytes(content []byte) string {
	h := sha256.Sum256(content)
	return hex.EncodeToString(h[:])
}

func hashString(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}
