package mcp

func toolDefinitions() []toolDefinition {
	return []toolDefinition{
		{
			Name:        "repo_list",
			Description: "Lista os repositorios ativos cadastrados no catalogo local.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
				"required":   []string{},
			},
		},
		{
			Name:        "repo_register",
			Description: "Registra ou atualiza um repositorio no catalogo local com caminho, descricao e tags.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name":        map[string]any{"type": "string"},
					"root_path":   map[string]any{"type": "string"},
					"description": map[string]any{"type": "string"},
					"tags":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"active":      map[string]any{"type": "boolean"},
				},
				"required": []string{"name"},
			},
		},
		{
			Name:        "repo_search",
			Description: "Busca no corpus indexado de um ou mais repositorios usando texto, embeddings ou modo hibrido.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"repos":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"query":       map[string]any{"type": "string"},
					"mode":        map[string]any{"type": "string", "enum": []string{"text", "semantic", "hybrid"}},
					"path_prefix": map[string]any{"type": "string"},
					"file_type":   map[string]any{"type": "string"},
					"language":    map[string]any{"type": "string"},
					"k":           map[string]any{"type": "integer", "minimum": 1, "maximum": 100},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "search_across_repos",
			Description: "Resume em quais repositorios um tema, modulo, contrato ou fluxo aparece, agrupando os melhores caminhos por repo.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"repos":          map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"query":          map[string]any{"type": "string"},
					"mode":           map[string]any{"type": "string", "enum": []string{"text", "semantic", "hybrid"}},
					"path_prefix":    map[string]any{"type": "string"},
					"file_type":      map[string]any{"type": "string"},
					"language":       map[string]any{"type": "string"},
					"k":              map[string]any{"type": "integer", "minimum": 1, "maximum": 100},
					"per_repo_paths": map[string]any{"type": "integer", "minimum": 1, "maximum": 10},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "repo_find_files",
			Description: "Encontra arquivos relevantes por nome, path, conteudo indexado e sinais extraidos.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"repos":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"query":       map[string]any{"type": "string"},
					"path_prefix": map[string]any{"type": "string"},
					"file_type":   map[string]any{"type": "string"},
					"language":    map[string]any{"type": "string"},
					"k":           map[string]any{"type": "integer", "minimum": 1, "maximum": 100},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "repo_find_docs",
			Description: "Encontra READMEs, ADRs e outros arquivos de documentacao relevantes em um ou mais repositorios.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"repos":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"query":       map[string]any{"type": "string"},
					"path_prefix": map[string]any{"type": "string"},
					"language":    map[string]any{"type": "string"},
					"k":           map[string]any{"type": "integer", "minimum": 1, "maximum": 100},
				},
				"required": []string{"query"},
			},
		},
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
				"required": []string{"kind", "content"},
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
				"required": []string{"query"},
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
				"required": []string{},
			},
		},
	}
}
