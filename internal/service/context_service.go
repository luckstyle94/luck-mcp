package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"luck-mcp/internal/domain"
	"luck-mcp/internal/embeddings"
	"luck-mcp/internal/repository"
)

type ContextService struct {
	store             repository.Store
	embeddings        embeddings.Client
	defaultProject    string
	expectedEmbedding int
	logger            *slog.Logger
}

type AddContextInput struct {
	Project    string   `json:"project"`
	Kind       string   `json:"kind"`
	Path       *string  `json:"path,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Content    string   `json:"content"`
	Importance *int     `json:"importance,omitempty"`
}

type SearchContextInput struct {
	Project    string   `json:"project"`
	Query      string   `json:"query"`
	K          *int     `json:"k,omitempty"`
	PathPrefix *string  `json:"path_prefix,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Kind       string   `json:"kind,omitempty"`
}

type ProjectBriefInput struct {
	Project  string `json:"project"`
	MaxItems *int   `json:"max_items,omitempty"`
}

func NewContextService(
	store repository.Store,
	embeddingsClient embeddings.Client,
	defaultProject string,
	expectedEmbedding int,
	logger *slog.Logger,
) *ContextService {
	if logger == nil {
		logger = slog.Default()
	}
	return &ContextService{
		store:             store,
		embeddings:        embeddingsClient,
		defaultProject:    strings.TrimSpace(defaultProject),
		expectedEmbedding: expectedEmbedding,
		logger:            logger,
	}
}

func (s *ContextService) AddContext(ctx context.Context, in AddContextInput) (int64, error) {
	repo, err := s.ensureRepo(ctx, in.Project, nil)
	if err != nil {
		return 0, err
	}

	kind := domain.Kind(strings.ToLower(strings.TrimSpace(in.Kind)))
	if !kind.IsValid() {
		return 0, fmt.Errorf("%w: invalid kind %q", domain.ErrInvalidInput, in.Kind)
	}

	content := strings.TrimSpace(in.Content)
	if content == "" {
		return 0, fmt.Errorf("%w: content is required", domain.ErrInvalidInput)
	}

	importance := 0
	if in.Importance != nil {
		importance = *in.Importance
	}
	if importance < 0 || importance > 5 {
		return 0, fmt.Errorf("%w: importance must be between 0 and 5", domain.ErrInvalidInput)
	}

	tags := normalizeTags(in.Tags)
	path := normalizePath(in.Path)
	hash := hashContent(content)

	if existingID, found, err := s.store.FindMemoryByRepoAndContentHash(ctx, repo.ID, hash); err == nil && found {
		s.logger.Info("context deduplicated",
			slog.Int64("entry_id", existingID),
			slog.String("repo", repo.Name),
			slog.String("kind", string(kind)),
			slog.Int("content_size", len(content)),
			slog.String("path", valueOrEmpty(path)),
			slog.Int("tags_count", len(tags)),
		)
		return existingID, nil
	} else if err != nil {
		return 0, fmt.Errorf("%w: %v", domain.ErrPersistenceFailed, err)
	}

	embedding, err := s.embeddings.Embed(ctx, content)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", domain.ErrEmbeddingFailed, err)
	}
	if len(embedding) != s.expectedEmbedding {
		return 0, fmt.Errorf("%w: embedding size is %d, expected %d", domain.ErrEmbeddingFailed, len(embedding), s.expectedEmbedding)
	}

	id, err := s.store.InsertMemoryWithEmbedding(ctx, repository.AddMemoryInput{
		RepoID:      repo.ID,
		Kind:        kind,
		Path:        path,
		Tags:        tags,
		Content:     content,
		Importance:  importance,
		ContentHash: &hash,
		Embedding:   embedding,
	})
	if err != nil {
		return 0, fmt.Errorf("%w: %v", domain.ErrPersistenceFailed, err)
	}

	s.logger.Info("context added",
		slog.Int64("entry_id", id),
		slog.String("repo", repo.Name),
		slog.String("kind", string(kind)),
		slog.Int("content_size", len(content)),
		slog.String("path", valueOrEmpty(path)),
		slog.Int("tags_count", len(tags)),
		slog.Int("importance", importance),
	)
	return id, nil
}

func (s *ContextService) SearchContext(ctx context.Context, in SearchContextInput) ([]domain.SearchResult, error) {
	repo, err := s.resolveExistingRepo(ctx, in.Project)
	if err != nil {
		return nil, err
	}

	query := strings.TrimSpace(in.Query)
	if query == "" {
		return nil, fmt.Errorf("%w: query is required", domain.ErrInvalidInput)
	}

	k := 8
	if in.K != nil {
		k = *in.K
	}
	if k <= 0 {
		return nil, fmt.Errorf("%w: k must be positive", domain.ErrInvalidInput)
	}
	if k > 100 {
		k = 100
	}

	kind := domain.KindAny
	if strings.TrimSpace(in.Kind) != "" {
		kind = domain.Kind(strings.ToLower(strings.TrimSpace(in.Kind)))
		if kind != domain.KindAny && !kind.IsValid() {
			return nil, fmt.Errorf("%w: invalid kind %q", domain.ErrInvalidInput, in.Kind)
		}
	}

	tags := normalizeTags(in.Tags)
	pathPrefix := normalizePath(in.PathPrefix)

	embedding, err := s.embeddings.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrEmbeddingFailed, err)
	}
	if len(embedding) != s.expectedEmbedding {
		return nil, fmt.Errorf("%w: embedding size is %d, expected %d", domain.ErrEmbeddingFailed, len(embedding), s.expectedEmbedding)
	}

	results, err := s.store.SearchMemory(ctx, repository.SearchMemoryInput{
		RepoID:         repo.ID,
		Kind:           kind,
		PathPrefix:     pathPrefix,
		Tags:           tags,
		K:              k,
		QueryEmbedding: embedding,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrPersistenceFailed, err)
	}

	s.logger.Info("context searched",
		slog.String("repo", repo.Name),
		slog.String("kind", string(kind)),
		slog.String("path_prefix", valueOrEmpty(pathPrefix)),
		slog.Int("tags_count", len(tags)),
		slog.Int("query_size", len(query)),
		slog.Int("k", k),
		slog.Int("result_count", len(results)),
	)
	return results, nil
}

func (s *ContextService) ProjectBrief(ctx context.Context, in ProjectBriefInput) (string, error) {
	repo, err := s.resolveExistingRepo(ctx, in.Project)
	if err != nil {
		return "", err
	}

	maxItems := 20
	if in.MaxItems != nil {
		maxItems = *in.MaxItems
	}
	if maxItems <= 0 {
		return "", fmt.Errorf("%w: max_items must be positive", domain.ErrInvalidInput)
	}
	if maxItems > 100 {
		maxItems = 100
	}

	items, err := s.store.ListMemoryBriefItems(ctx, repo.ID, maxItems)
	if err != nil {
		return "", fmt.Errorf("%w: %v", domain.ErrPersistenceFailed, err)
	}
	if len(items) == 0 {
		return s.buildIndexedFallbackBrief(ctx, repo, maxItems)
	}

	sort.SliceStable(items, func(i, j int) bool {
		iSummary := items[i].Kind == domain.KindSummary
		jSummary := items[j].Kind == domain.KindSummary
		if iSummary != jSummary {
			return iSummary
		}
		if items[i].Importance != items[j].Importance {
			return items[i].Importance > items[j].Importance
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})

	var b strings.Builder
	b.WriteString("Brief de contexto do projeto ")
	b.WriteString(repo.Name)
	b.WriteString(":\n")
	for _, item := range items {
		b.WriteString("- [")
		b.WriteString(string(item.Kind))
		b.WriteString("]")
		if item.Path != nil {
			b.WriteString(" (")
			b.WriteString(*item.Path)
			b.WriteString(")")
		}
		if item.Importance > 0 {
			b.WriteString(" {importance=")
			b.WriteString(fmt.Sprintf("%d", item.Importance))
			b.WriteString("}")
		}
		b.WriteString(" ")
		b.WriteString(item.Content)
		b.WriteByte('\n')
	}

	s.logger.Info("project brief generated",
		slog.String("repo", repo.Name),
		slog.Int("max_items", maxItems),
		slog.Int("returned_items", len(items)),
	)
	return strings.TrimSpace(b.String()), nil
}

func (s *ContextService) ensureRepo(ctx context.Context, project string, rootPath *string) (domain.Repo, error) {
	name := strings.TrimSpace(project)
	if name == "" {
		name = s.defaultProject
	}
	if name == "" {
		return domain.Repo{}, fmt.Errorf("%w: project is required", domain.ErrInvalidInput)
	}
	repo, err := s.store.EnsureRepo(ctx, name, rootPath)
	if err != nil {
		return domain.Repo{}, fmt.Errorf("%w: %v", domain.ErrPersistenceFailed, err)
	}
	return repo, nil
}

func (s *ContextService) resolveExistingRepo(ctx context.Context, project string) (domain.Repo, error) {
	name := strings.TrimSpace(project)
	if name == "" {
		name = s.defaultProject
	}
	if name == "" {
		return domain.Repo{}, fmt.Errorf("%w: project is required", domain.ErrInvalidInput)
	}

	repo, found, err := s.store.GetRepoByName(ctx, name)
	if err != nil {
		return domain.Repo{}, fmt.Errorf("%w: %v", domain.ErrPersistenceFailed, err)
	}
	if !found {
		return domain.Repo{}, fmt.Errorf("%w: project %q is not registered or indexed yet; run repo_register or mcp-index first", domain.ErrInvalidInput, name)
	}
	return repo, nil
}

func (s *ContextService) buildIndexedFallbackBrief(ctx context.Context, repo domain.Repo, maxItems int) (string, error) {
	docLimit := maxItems
	if docLimit > 4 {
		docLimit = 4
	}
	if docLimit < 2 {
		docLimit = 2
	}

	docs, err := s.store.FindFiles(ctx, repository.FindFilesInput{
		RepoNames: []string{repo.Name},
		Query:     "readme architecture overview setup quickstart adr guide docs",
		FileType:  "doc",
		K:         docLimit,
	})
	if err != nil {
		return "", fmt.Errorf("%w: %v", domain.ErrPersistenceFailed, err)
	}

	indexedFiles, err := s.store.ListIndexedFiles(ctx, repo.ID)
	if err != nil {
		return "", fmt.Errorf("%w: %v", domain.ErrPersistenceFailed, err)
	}
	highlights := selectHighlightedFiles(indexedFiles, minInt(maxItems, 5))

	if len(docs) == 0 && len(highlights) == 0 {
		return fmt.Sprintf("Projeto %s encontrado, mas ainda nao ha memoria curada nem arquivos indexados suficientes. Rode mcp-index neste repo ou salve contexto com context_add.", repo.Name), nil
	}

	var b strings.Builder
	b.WriteString("Brief de contexto do projeto ")
	b.WriteString(repo.Name)
	b.WriteString(" (fallback do indice; ainda nao ha memoria curada):\n")

	if repo.LastIndexedAt != nil {
		b.WriteString("- ultimo indice atualizado em: ")
		b.WriteString(repo.LastIndexedAt.Format(time.RFC3339))
		b.WriteByte('\n')
	}

	if len(docs) > 0 {
		b.WriteString("- docs recomendados para comecar:\n")
		for _, doc := range docs {
			b.WriteString("  - ")
			b.WriteString(doc.Path)
			b.WriteByte('\n')
		}
	}

	if len(highlights) > 0 {
		b.WriteString("- arquivos-chave indexados:\n")
		for _, file := range highlights {
			b.WriteString("  - ")
			b.WriteString(file.Path)
			b.WriteString(" [")
			b.WriteString(file.FileType)
			if file.Language != "" {
				b.WriteString("/")
				b.WriteString(file.Language)
			}
			b.WriteString("]")
			if file.ChunkCount > 0 {
				b.WriteString(fmt.Sprintf(" chunks=%d", file.ChunkCount))
			}
			b.WriteByte('\n')
		}
	}

	b.WriteString("- proximo passo sugerido: leia os docs acima e depois use repo_find_files ou repo_search para aprofundar um tema.")

	s.logger.Info("project brief fallback generated",
		slog.String("repo", repo.Name),
		slog.Int("doc_count", len(docs)),
		slog.Int("highlight_count", len(highlights)),
	)
	return strings.TrimSpace(b.String()), nil
}

func selectHighlightedFiles(files []repository.IndexedFile, limit int) []repository.IndexedFile {
	if len(files) == 0 || limit <= 0 {
		return nil
	}

	candidates := make([]repository.IndexedFile, 0, len(files))
	for _, file := range files {
		if strings.EqualFold(file.Status, "indexed") {
			candidates = append(candidates, file)
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		left := highlightedFileScore(candidates[i])
		right := highlightedFileScore(candidates[j])
		if left == right {
			return candidates[i].Path < candidates[j].Path
		}
		return left > right
	})

	seen := make(map[string]struct{}, limit)
	selected := make([]repository.IndexedFile, 0, limit)
	for _, file := range candidates {
		if _, ok := seen[file.Path]; ok {
			continue
		}
		seen[file.Path] = struct{}{}
		selected = append(selected, file)
		if len(selected) == limit {
			break
		}
	}
	return selected
}

func highlightedFileScore(file repository.IndexedFile) int {
	score := 0
	switch file.FileType {
	case "infra":
		score += 400
	case "code":
		score += 320
	case "config":
		score += 200
	case "doc":
		score += 80
	case "test":
		score -= 120
	}

	path := strings.ToLower(strings.TrimSpace(file.Path))
	base := filepath.Base(path)

	switch {
	case base == "main.go":
		score += 180
	case base == "main.tf":
		score += 170
	case base == "versions.tf":
		score += 150
	case strings.HasPrefix(base, "readme"):
		score += 140
	}

	if strings.Contains(path, "/cmd/") {
		score += 110
	}
	if strings.Contains(path, "/internal/service/") || strings.Contains(path, "/service/") {
		score += 90
	}
	if strings.Contains(path, "/module") || strings.Contains(path, "/modules/") {
		score += 80
	}
	if strings.Contains(path, "/docs/") || strings.Contains(path, "/adr") {
		score += 30
	}

	score += minInt(file.ChunkCount, 20) * 6
	score += minInt(int(file.SizeBytes/512), 30)
	score -= minInt(len(path), 120) / 3
	return score
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func normalizeTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, raw := range tags {
		t := strings.ToLower(strings.TrimSpace(raw))
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizePath(path *string) *string {
	if path == nil {
		return nil
	}
	p := strings.TrimSpace(*path)
	if p == "" {
		return nil
	}
	return &p
}

func normalizeOptionalText(v *string) *string {
	if v == nil {
		return nil
	}
	text := strings.TrimSpace(*v)
	if text == "" {
		return nil
	}
	return &text
}

func hashContent(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

func valueOrEmpty(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
