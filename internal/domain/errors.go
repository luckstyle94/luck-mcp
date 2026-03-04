package domain

import "errors"

var (
	ErrInvalidInput      = errors.New("invalid input")
	ErrEmbeddingFailed   = errors.New("embedding generation failed")
	ErrPersistenceFailed = errors.New("persistence failed")
)
