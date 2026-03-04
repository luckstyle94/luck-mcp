package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"luck-mpc/internal/domain"
	"luck-mpc/internal/embeddings"
	"luck-mpc/internal/repository"
)

type ContextService struct {
	repo              repository.DocumentRepository
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
	repo repository.DocumentRepository,
	embeddingsClient embeddings.Client,
	defaultProject string,
	expectedEmbedding int,
	logger *slog.Logger,
) *ContextService {
	if logger == nil {
		logger = slog.Default()
	}
	return &ContextService{
		repo:              repo,
		embeddings:        embeddingsClient,
		defaultProject:    strings.TrimSpace(defaultProject),
		expectedEmbedding: expectedEmbedding,
		logger:            logger,
	}
}

func (s *ContextService) AddContext(ctx context.Context, in AddContextInput) (int64, error) {
	project := s.resolveProject(in.Project)
	if project == "" {
		return 0, fmt.Errorf("%w: project is required", domain.ErrInvalidInput)
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

	if existingID, found, err := s.repo.FindByProjectAndContentHash(ctx, project, hash); err == nil && found {
		s.logger.Info("context deduplicated",
			slog.Int64("doc_id", existingID),
			slog.String("project", project),
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

	id, err := s.repo.InsertDocumentWithEmbedding(ctx, repository.AddDocumentInput{
		Project:     project,
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
		slog.Int64("doc_id", id),
		slog.String("project", project),
		slog.String("kind", string(kind)),
		slog.Int("content_size", len(content)),
		slog.String("path", valueOrEmpty(path)),
		slog.Int("tags_count", len(tags)),
		slog.Int("importance", importance),
	)
	return id, nil
}

func (s *ContextService) SearchContext(ctx context.Context, in SearchContextInput) ([]domain.SearchResult, error) {
	project := s.resolveProject(in.Project)
	if project == "" {
		return nil, fmt.Errorf("%w: project is required", domain.ErrInvalidInput)
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

	results, err := s.repo.Search(ctx, repository.SearchDocumentsInput{
		Project:        project,
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
		slog.String("project", project),
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
	project := s.resolveProject(in.Project)
	if project == "" {
		return "", fmt.Errorf("%w: project is required", domain.ErrInvalidInput)
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

	items, err := s.repo.ListBriefItems(ctx, project, maxItems)
	if err != nil {
		return "", fmt.Errorf("%w: %v", domain.ErrPersistenceFailed, err)
	}

	if len(items) == 0 {
		return "Nenhum contexto encontrado para o projeto.", nil
	}

	// Reforca prioridade para summaries e importance alta antes de montar o texto final.
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
	b.WriteString(project)
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
		slog.String("project", project),
		slog.Int("max_items", maxItems),
		slog.Int("returned_items", len(items)),
	)

	return strings.TrimSpace(b.String()), nil
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

func hashContent(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

func (s *ContextService) resolveProject(project string) string {
	if p := strings.TrimSpace(project); p != "" {
		return p
	}
	return s.defaultProject
}

func valueOrEmpty(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
