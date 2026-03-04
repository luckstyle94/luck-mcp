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
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"luck-mpc/internal/domain"
	"luck-mpc/internal/embeddings"
	"luck-mpc/internal/repository"
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
	Path    string
	Hash    string
	Content string
	Tags    []string
}

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

	knownFiles, err := s.repo.ListIndexedFiles(ctx, opts.Project)
	if err != nil {
		return Result{}, fmt.Errorf("list indexed files: %w", err)
	}
	indexedByPath := make(map[string]string, len(knownFiles))
	for _, f := range knownFiles {
		indexedByPath[f.Path] = f.ContentHash
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

		fileHash := hashBytes(contentBytes)
		currentHashes[relPath] = fileHash

		if opts.Mode == "changed" {
			if oldHash, ok := indexedByPath[relPath]; ok && oldHash == fileHash {
				result.UnchangedFiles++
				return nil
			}
		}

		toIndex = append(toIndex, sourceFile{
			Path:    relPath,
			Hash:    fileHash,
			Content: string(contentBytes),
			Tags:    buildAutoTags(relPath),
		})
		return nil
	})
	if err != nil {
		return result, fmt.Errorf("scan project files: %w", err)
	}

	deleted := s.removeMissingFiles(ctx, opts.Project, indexedByPath, currentHashes)
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
				Path:        file.Path,
				ContentHash: file.Hash,
				ChunkCount:  0,
				Status:      "error",
				Error:       &msg,
			})
			if firstErr == nil {
				firstErr = fmt.Errorf("split chunks for %s: %w", file.Path, err)
			}
			continue
		}

		deletedRows, err := s.repo.DeleteAutoChunksByPath(ctx, opts.Project, file.Path)
		if err != nil {
			result.FailedFiles++
			msg := err.Error()
			_ = s.repo.UpsertIndexedFile(ctx, repository.UpsertIndexedFileInput{
				Project:     opts.Project,
				Path:        file.Path,
				ContentHash: file.Hash,
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
			embedding, err := s.embeddings.Embed(ctx, chunk)
			if err != nil {
				result.FailedFiles++
				fileFailed = true
				msg := fmt.Sprintf("embedding failed on chunk %d: %v", idx, err)
				_ = s.repo.UpsertIndexedFile(ctx, repository.UpsertIndexedFileInput{
					Project:     opts.Project,
					Path:        file.Path,
					ContentHash: file.Hash,
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
					Path:        file.Path,
					ContentHash: file.Hash,
					ChunkCount:  insertedChunks,
					Status:      "error",
					Error:       &msg,
				})
				if firstErr == nil {
					firstErr = fmt.Errorf("invalid embedding size for %s", file.Path)
				}
				break
			}

			path := file.Path
			chunkHash := hashString(strings.Join([]string{opts.Project, file.Path, strconv.Itoa(idx), chunk}, "\n"))
			_, err = s.repo.InsertDocumentWithEmbedding(ctx, repository.AddDocumentInput{
				Project:     opts.Project,
				Kind:        domain.KindChunk,
				Path:        &path,
				Tags:        file.Tags,
				Content:     chunk,
				Importance:  1,
				ContentHash: &chunkHash,
				Embedding:   embedding,
			})
			if err != nil {
				result.FailedFiles++
				fileFailed = true
				msg := fmt.Sprintf("insert chunk %d failed: %v", idx, err)
				_ = s.repo.UpsertIndexedFile(ctx, repository.UpsertIndexedFileInput{
					Project:     opts.Project,
					Path:        file.Path,
					ContentHash: file.Hash,
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

		if err := s.repo.UpsertIndexedFile(ctx, repository.UpsertIndexedFileInput{
			Project:     opts.Project,
			Path:        file.Path,
			ContentHash: file.Hash,
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
	}

	if firstErr != nil {
		return result, fmt.Errorf("index finished with failures (%d files): %w", result.FailedFiles, firstErr)
	}

	return result, nil
}

func (s *Service) removeMissingFiles(ctx context.Context, project string, indexedByPath map[string]string, currentHashes map[string]string) int {
	removed := 0
	paths := make([]string, 0)
	for path := range indexedByPath {
		if _, ok := currentHashes[path]; !ok {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)

	for _, path := range paths {
		deletedRows, err := s.repo.DeleteAutoChunksByPath(ctx, project, path)
		if err != nil {
			s.logger.Warn("failed to delete chunks for missing file",
				slog.String("project", project),
				slog.String("path", path),
				slog.String("error", err.Error()),
			)
			continue
		}
		if err := s.repo.DeleteIndexedFile(ctx, project, path); err != nil {
			s.logger.Warn("failed to delete indexed_file row",
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

	if len(content) <= size {
		return []string{content}, nil
	}

	step := size - overlap
	chunks := make([]string, 0, (len(content)/step)+1)
	for start := 0; start < len(content); start += step {
		end := start + size
		if end > len(content) {
			end = len(content)
		}
		chunk := strings.TrimSpace(content[start:end])
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		if end == len(content) {
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

func hashBytes(content []byte) string {
	h := sha256.Sum256(content)
	return hex.EncodeToString(h[:])
}

func hashString(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}
