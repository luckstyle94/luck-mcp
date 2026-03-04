package embeddings

type OllamaEmbeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type OllamaEmbeddingResponse struct {
	Embedding  []float64   `json:"embedding"`
	Embeddings [][]float64 `json:"embeddings"`
}
