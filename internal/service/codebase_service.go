package service

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"luck-mcp/internal/domain"
	"luck-mcp/internal/embeddings"
	"luck-mcp/internal/repository"
)

type CodebaseService struct {
	store             repository.Store
	embeddings        embeddings.Client
	expectedEmbedding int
	logger            *slog.Logger
}

type RepoSearchInput struct {
	Repos      []string `json:"repos,omitempty"`
	Query      string   `json:"query"`
	Mode       string   `json:"mode,omitempty"`
	PathPrefix *string  `json:"path_prefix,omitempty"`
	FileType   string   `json:"file_type,omitempty"`
	Language   string   `json:"language,omitempty"`
	K          *int     `json:"k,omitempty"`
}

type RepoRegisterInput struct {
	Name        string   `json:"name"`
	RootPath    *string  `json:"root_path,omitempty"`
	Description *string  `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Active      *bool    `json:"active,omitempty"`
}

type FindFilesInput struct {
	Repos      []string `json:"repos,omitempty"`
	Query      string   `json:"query"`
	PathPrefix *string  `json:"path_prefix,omitempty"`
	FileType   string   `json:"file_type,omitempty"`
	Language   string   `json:"language,omitempty"`
	K          *int     `json:"k,omitempty"`
}

type SearchAcrossReposInput struct {
	Repos        []string `json:"repos,omitempty"`
	Query        string   `json:"query"`
	Mode         string   `json:"mode,omitempty"`
	PathPrefix   *string  `json:"path_prefix,omitempty"`
	FileType     string   `json:"file_type,omitempty"`
	Language     string   `json:"language,omitempty"`
	K            *int     `json:"k,omitempty"`
	PerRepoPaths *int     `json:"per_repo_paths,omitempty"`
}

func NewCodebaseService(store repository.Store, embeddingsClient embeddings.Client, expectedEmbedding int, logger *slog.Logger) *CodebaseService {
	if logger == nil {
		logger = slog.Default()
	}
	return &CodebaseService{
		store:             store,
		embeddings:        embeddingsClient,
		expectedEmbedding: expectedEmbedding,
		logger:            logger,
	}
}

func (s *CodebaseService) ListRepos(ctx context.Context) ([]domain.Repo, error) {
	repos, err := s.store.ListRepos(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrPersistenceFailed, err)
	}
	return repos, nil
}

func (s *CodebaseService) RegisterRepo(ctx context.Context, in RepoRegisterInput) (domain.Repo, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return domain.Repo{}, fmt.Errorf("%w: name is required", domain.ErrInvalidInput)
	}

	repo, err := s.store.UpsertRepo(ctx, repository.UpsertRepoInput{
		Name:        name,
		RootPath:    normalizePath(in.RootPath),
		Description: normalizeOptionalText(in.Description),
		Tags:        normalizeTags(in.Tags),
		Active:      in.Active,
	})
	if err != nil {
		return domain.Repo{}, fmt.Errorf("%w: %v", domain.ErrPersistenceFailed, err)
	}

	s.logger.Info("repo registered",
		slog.String("repo", repo.Name),
		slog.String("root_path", valueOrEmpty(repo.RootPath)),
		slog.Int("tags_count", len(repo.Tags)),
		slog.Bool("active", repo.Active),
	)
	return repo, nil
}

func (s *CodebaseService) RepoSearch(ctx context.Context, in RepoSearchInput) ([]domain.RepoSearchResult, error) {
	query := strings.TrimSpace(in.Query)
	if query == "" {
		return nil, fmt.Errorf("%w: query is required", domain.ErrInvalidInput)
	}

	mode := domain.SearchModeHybrid
	if strings.TrimSpace(in.Mode) != "" {
		mode = domain.SearchMode(strings.ToLower(strings.TrimSpace(in.Mode)))
		if !mode.IsValid() {
			return nil, fmt.Errorf("%w: invalid mode %q", domain.ErrInvalidInput, in.Mode)
		}
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

	repos := normalizeTags(in.Repos)
	pathPrefix := normalizePath(in.PathPrefix)
	fileType := strings.ToLower(strings.TrimSpace(in.FileType))
	language := strings.ToLower(strings.TrimSpace(in.Language))

	var embedding []float64
	var err error
	if mode == domain.SearchModeSemantic || mode == domain.SearchModeHybrid {
		embedding, err = s.embeddings.Embed(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", domain.ErrEmbeddingFailed, err)
		}
		if len(embedding) != s.expectedEmbedding {
			return nil, fmt.Errorf("%w: embedding size is %d, expected %d", domain.ErrEmbeddingFailed, len(embedding), s.expectedEmbedding)
		}
	}

	results, err := s.store.SearchIndexedChunks(ctx, repository.SearchIndexedChunksInput{
		RepoNames:      repos,
		Query:          query,
		Mode:           mode,
		PathPrefix:     pathPrefix,
		FileType:       fileType,
		Language:       language,
		K:              k,
		QueryEmbedding: embedding,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrPersistenceFailed, err)
	}

	s.logger.Info("repo search completed",
		slog.String("mode", string(mode)),
		slog.Int("repo_count", len(repos)),
		slog.String("file_type", fileType),
		slog.String("language", language),
		slog.String("path_prefix", valueOrEmpty(pathPrefix)),
		slog.Int("k", k),
		slog.Int("result_count", len(results)),
	)
	return results, nil
}

func (s *CodebaseService) FindFiles(ctx context.Context, in FindFilesInput) ([]domain.FileMatch, error) {
	query := strings.TrimSpace(in.Query)
	if query == "" {
		return nil, fmt.Errorf("%w: query is required", domain.ErrInvalidInput)
	}

	k := 20
	if in.K != nil {
		k = *in.K
	}
	if k <= 0 {
		return nil, fmt.Errorf("%w: k must be positive", domain.ErrInvalidInput)
	}
	if k > 100 {
		k = 100
	}

	results, err := s.store.FindFiles(ctx, repository.FindFilesInput{
		RepoNames:  normalizeTags(in.Repos),
		Query:      query,
		PathPrefix: normalizePath(in.PathPrefix),
		FileType:   strings.ToLower(strings.TrimSpace(in.FileType)),
		Language:   strings.ToLower(strings.TrimSpace(in.Language)),
		K:          k,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrPersistenceFailed, err)
	}

	s.logger.Info("file discovery completed",
		slog.Int("repo_count", len(in.Repos)),
		slog.String("file_type", strings.ToLower(strings.TrimSpace(in.FileType))),
		slog.String("language", strings.ToLower(strings.TrimSpace(in.Language))),
		slog.String("path_prefix", valueOrEmpty(normalizePath(in.PathPrefix))),
		slog.Int("k", k),
		slog.Int("result_count", len(results)),
	)
	return results, nil
}

func (s *CodebaseService) FindDocs(ctx context.Context, in FindFilesInput) ([]domain.FileMatch, error) {
	in.FileType = "doc"
	return s.FindFiles(ctx, in)
}

func (s *CodebaseService) SearchAcrossRepos(ctx context.Context, in SearchAcrossReposInput) ([]domain.CrossRepoMatch, error) {
	query := strings.TrimSpace(in.Query)
	if query == "" {
		return nil, fmt.Errorf("%w: query is required", domain.ErrInvalidInput)
	}

	repoLimit := 8
	if in.K != nil {
		repoLimit = *in.K
	}
	if repoLimit <= 0 {
		return nil, fmt.Errorf("%w: k must be positive", domain.ErrInvalidInput)
	}
	if repoLimit > 100 {
		repoLimit = 100
	}

	perRepoPaths := 3
	if in.PerRepoPaths != nil {
		perRepoPaths = *in.PerRepoPaths
	}
	if perRepoPaths <= 0 {
		return nil, fmt.Errorf("%w: per_repo_paths must be positive", domain.ErrInvalidInput)
	}
	if perRepoPaths > 10 {
		perRepoPaths = 10
	}

	fileLimit := repoLimit * 8
	if fileLimit < 20 {
		fileLimit = 20
	}
	chunkLimit := repoLimit * 8
	if chunkLimit < 20 {
		chunkLimit = 20
	}

	fileMatches, err := s.FindFiles(ctx, FindFilesInput{
		Repos:      in.Repos,
		Query:      query,
		PathPrefix: in.PathPrefix,
		FileType:   in.FileType,
		Language:   in.Language,
		K:          &fileLimit,
	})
	if err != nil {
		return nil, err
	}

	chunkMatches, err := s.RepoSearch(ctx, RepoSearchInput{
		Repos:      in.Repos,
		Query:      query,
		Mode:       in.Mode,
		PathPrefix: in.PathPrefix,
		FileType:   in.FileType,
		Language:   in.Language,
		K:          &chunkLimit,
	})
	if err != nil {
		return nil, err
	}

	type aggregate struct {
		score     float64
		matchSet  map[string]struct{}
		pathSet   map[string]struct{}
		pathScore map[string]float64
		fileTypes map[string]struct{}
		languages map[string]struct{}
	}

	aggs := make(map[string]*aggregate)
	ensure := func(repo string) *aggregate {
		a, ok := aggs[repo]
		if ok {
			return a
		}
		a = &aggregate{
			matchSet:  make(map[string]struct{}),
			pathSet:   make(map[string]struct{}),
			pathScore: make(map[string]float64),
			fileTypes: make(map[string]struct{}),
			languages: make(map[string]struct{}),
		}
		aggs[repo] = a
		return a
	}

	addPath := func(a *aggregate, path string, score float64) {
		a.pathSet[path] = struct{}{}
		if current, ok := a.pathScore[path]; !ok || score > current {
			a.pathScore[path] = score
		}
	}

	for _, item := range fileMatches {
		a := ensure(item.Repo)
		a.score += item.Score * 1.10
		a.matchSet["file:"+item.Path] = struct{}{}
		addPath(a, item.Path, item.Score)
		if item.FileType != "" {
			a.fileTypes[item.FileType] = struct{}{}
		}
		if item.Language != "" {
			a.languages[item.Language] = struct{}{}
		}
	}

	for _, item := range chunkMatches {
		a := ensure(item.Repo)
		a.score += item.Score * 0.90
		a.matchSet["chunk:"+item.Path] = struct{}{}
		addPath(a, item.Path, item.Score)
		if item.FileType != "" {
			a.fileTypes[item.FileType] = struct{}{}
		}
		if item.Language != "" {
			a.languages[item.Language] = struct{}{}
		}
	}

	results := make([]domain.CrossRepoMatch, 0, len(aggs))
	for repo, a := range aggs {
		matchCount := len(a.matchSet)
		if matchCount == 0 {
			continue
		}

		score := a.score / float64(matchCount)
		score += float64(matchCount-1) * 0.08
		if len(a.fileTypes) > 1 {
			score += 0.04
		}

		results = append(results, domain.CrossRepoMatch{
			Repo:       repo,
			Score:      score,
			MatchCount: matchCount,
			TopPaths:   topScoredPaths(a.pathScore, perRepoPaths),
			FileTypes:  sortedKeys(a.fileTypes),
			Languages:  sortedKeys(a.languages),
		})
	}

	sortCrossRepoMatches(results)
	if len(results) > repoLimit {
		results = results[:repoLimit]
	}

	s.logger.Info("cross repo search completed",
		slog.Int("repo_scope_count", len(in.Repos)),
		slog.Int("repo_result_count", len(results)),
		slog.Int("file_result_count", len(fileMatches)),
		slog.Int("chunk_result_count", len(chunkMatches)),
		slog.Int("per_repo_paths", perRepoPaths),
	)
	return results, nil
}

func sortedKeys(values map[string]struct{}) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for key := range values {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func sortCrossRepoMatches(items []domain.CrossRepoMatch) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Score == items[j].Score {
			if items[i].MatchCount == items[j].MatchCount {
				return items[i].Repo < items[j].Repo
			}
			return items[i].MatchCount > items[j].MatchCount
		}
		return items[i].Score > items[j].Score
	})
}

func topScoredPaths(pathScore map[string]float64, limit int) []string {
	if len(pathScore) == 0 || limit <= 0 {
		return nil
	}

	type pathItem struct {
		path  string
		score float64
	}
	items := make([]pathItem, 0, len(pathScore))
	for path, score := range pathScore {
		items = append(items, pathItem{path: path, score: score})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].score == items[j].score {
			return items[i].path < items[j].path
		}
		return items[i].score > items[j].score
	})

	if len(items) > limit {
		items = items[:limit]
	}

	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.path)
	}
	return out
}
