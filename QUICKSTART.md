# Quickstart (PT-BR)

Guia rapido para colocar o MCP no ar e usar no Cursor em poucos minutos.

## 1) Primeira configuracao

```bash
cd /home/luckstyle/repo/private/luck-mpc

docker compose build mcp
docker compose up -d postgres ollama mcp
make migrate
docker compose exec ollama ollama pull nomic-embed-text
```

## 2) Configurar no Cursor

Use esta configuracao MCP:

```json
{
  "mcpServers": {
    "luck-mpc": {
      "command": "docker",
      "args": [
        "exec",
        "-e",
        "LOG_LEVEL=error",
        "-i",
        "luck-mpc-server",
        "/mcp-server"
      ]
    }
  }
}
```

Depois:
1. Salve a configuracao.
2. Reload Window no Cursor.
3. Verifique se as tools aparecem: `context_add`, `context_search`, `project_brief`.

## 3) Uso diario

### Iniciar no comeco do dia
```bash
cd /home/luckstyle/repo/private/luck-mpc
docker compose up -d postgres ollama mcp
```

### Parar no fim do dia
```bash
cd /home/luckstyle/repo/private/luck-mpc
docker compose down
```

### Quando rodar migrate
```bash
cd /home/luckstyle/repo/private/luck-mpc
make migrate
```

## 4) Prompts prontos para usar com a IA

Carregar contexto no inicio da sessao:
```text
Use project_brief no projeto "meu-projeto" com max_items=20 e me mostre um resumo objetivo.
```

Buscar antes de codar:
```text
Use context_search no projeto "meu-projeto" com query "fluxo de autenticacao" e k=8.
```

Salvar decisao importante:
```text
Use context_add no projeto "meu-projeto" com kind="summary", tags=["arquitetura"], importance=5,
content="Decisao: ... Motivo: ... Impacto: ...".
```

Salvar gotcha:
```text
Use context_add no projeto "meu-projeto" com kind="memory", tags=["gotcha"], importance=4,
content="Problema: ... Causa: ... Solucao: ...".
```

## 5) Comandos rapidos

Status:
```bash
docker compose ps
```

Logs MCP:
```bash
docker logs --tail=200 luck-mpc-server
```

## 6) Se der problema no Cursor (loading tools)

1. Garanta containers ativos:
```bash
docker compose ps
```

2. Rebuild da imagem MCP:
```bash
docker compose build mcp
```

3. Reabra o Cursor (Reload Window).

---

Documentacao completa:
- [README.md](./README.md)
- [README.en.md](./README.en.md)
- [QUICKSTART.en.md](./QUICKSTART.en.md)
