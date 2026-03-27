package domain

import "time"

type Repo struct {
	ID            int64      `json:"id"`
	Name          string     `json:"name"`
	RootPath      *string    `json:"root_path,omitempty"`
	Description   *string    `json:"description,omitempty"`
	Tags          []string   `json:"tags,omitempty"`
	Active        bool       `json:"active"`
	LastIndexedAt *time.Time `json:"last_indexed_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
