package domain

import "time"

type Kind string

const (
	KindNote    Kind = "note"
	KindChunk   Kind = "chunk"
	KindSummary Kind = "summary"
	KindMemory  Kind = "memory"
	KindAny     Kind = "any"
)

func (k Kind) IsValid() bool {
	switch k {
	case KindNote, KindChunk, KindSummary, KindMemory:
		return true
	default:
		return false
	}
}

type Document struct {
	ID          int64
	Project     string
	Kind        Kind
	Path        *string
	Tags        []string
	Content     string
	Importance  int
	ContentHash *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type SearchResult struct {
	ID      int64    `json:"id"`
	Score   float64  `json:"score"`
	Kind    string   `json:"kind"`
	Path    *string  `json:"path"`
	Tags    []string `json:"tags"`
	Content string   `json:"content"`
}

type BriefItem struct {
	Kind       Kind
	Path       *string
	Tags       []string
	Content    string
	Importance int
	UpdatedAt  time.Time
}
