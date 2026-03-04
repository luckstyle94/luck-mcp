package mcp

func toolDefinitions() []toolDefinition {
	return []toolDefinition{
		{
			Name:        "context_add",
			Description: "Armazena um contexto persistente no projeto com embedding semantico.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"project":    map[string]any{"type": "string"},
					"kind":       map[string]any{"type": "string", "enum": []string{"note", "chunk", "summary", "memory"}},
					"path":       map[string]any{"type": "string"},
					"tags":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"content":    map[string]any{"type": "string"},
					"importance": map[string]any{"type": "integer", "minimum": 0, "maximum": 5},
				},
				"required": []string{"project", "kind", "content"},
			},
		},
		{
			Name:        "context_search",
			Description: "Busca contexto por similaridade semantica com filtros opcionais.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"project":     map[string]any{"type": "string"},
					"query":       map[string]any{"type": "string"},
					"k":           map[string]any{"type": "integer", "minimum": 1, "maximum": 100},
					"path_prefix": map[string]any{"type": "string"},
					"tags":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"kind":        map[string]any{"type": "string", "enum": []string{"note", "chunk", "summary", "memory", "any"}},
				},
				"required": []string{"project", "query"},
			},
		},
		{
			Name:        "project_brief",
			Description: "Retorna um brief de bootstrap priorizando summaries e alta importancia.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"project":   map[string]any{"type": "string"},
					"max_items": map[string]any{"type": "integer", "minimum": 1, "maximum": 100},
				},
				"required": []string{"project"},
			},
		},
	}
}
