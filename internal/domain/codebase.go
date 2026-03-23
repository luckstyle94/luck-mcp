package domain

type SearchMode string

const (
	SearchModeText     SearchMode = "text"
	SearchModeSemantic SearchMode = "semantic"
	SearchModeHybrid   SearchMode = "hybrid"
)

func (m SearchMode) IsValid() bool {
	switch m {
	case SearchModeText, SearchModeSemantic, SearchModeHybrid:
		return true
	default:
		return false
	}
}

type RepoSearchResult struct {
	Repo     string   `json:"repo"`
	Path     string   `json:"path"`
	Score    float64  `json:"score"`
	FileType string   `json:"file_type"`
	Language string   `json:"language"`
	Tags     []string `json:"tags,omitempty"`
	Content  string   `json:"content"`
}

type FileMatch struct {
	Repo      string  `json:"repo"`
	Path      string  `json:"path"`
	Score     float64 `json:"score"`
	FileType  string  `json:"file_type"`
	Language  string  `json:"language"`
	SizeBytes int64   `json:"size_bytes"`
	Snippet   string  `json:"snippet,omitempty"`
}

type CrossRepoMatch struct {
	Repo       string   `json:"repo"`
	Score      float64  `json:"score"`
	MatchCount int      `json:"match_count"`
	TopPaths   []string `json:"top_paths,omitempty"`
	FileTypes  []string `json:"file_types,omitempty"`
	Languages  []string `json:"languages,omitempty"`
}
