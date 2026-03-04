# Quickstart (PT-BR)

Guia rapido para colocar o MCP no ar e usar no Cursor em poucos minutos.

## Cheatsheet diario (resumo de 30s)
1. Abrir o dia:
```bash
cd /home/$USER/repo/private/luck-mpc
make up
make index PROJECT=meu-projeto ROOT=/caminho/absoluto/do/projeto
```

2. Trabalhar com IA (no chat):
- `project_brief` no inicio da sessao
- `context_search` antes de mexer em partes criticas
- `context_add` depois de decisoes importantes

3. Fechar o dia (opcional):
```bash
cd /home/$USER/repo/private/luck-mpc
make down
```

## O que e index, reindex e incremental?
- `make index`: atualiza contexto automaticamente so do que mudou (arquivos novos/alterados/removidos). Esse e o comando do dia a dia.
- `make index-full`: refaz toda a indexacao do projeto do zero. Use quando quiser reconstruir tudo.
- `incremental`: significa "somente diferencas". Mais rapido.
- `reindex completo`: significa "todos os arquivos novamente". Mais lento.

Quando usar cada comando:
- Comecou a trabalhar: `make up` + `make index ...`
- Entrou migration nova: `make migrate`
- Mudou muito codigo: `make index ...`
- Quer resetar contexto indexado: `make index-full ...`
- Terminou o dia: `make down` (opcional)

## 0) Onde rodar os comandos
Rode todos os comandos abaixo no seu terminal local, dentro da pasta do MCP:

```bash
cd /home/$USER/repo/private/luck-mpc
```

Importante:
- `make index` sempre roda aqui no `luck-mpc`.
- O `ROOT` aponta para o projeto que voce quer indexar (ex.: `/home/$USER/repo/private/meu-api`).
- As tools (`context_add`, `context_search`, `project_brief`) sao usadas no chat do agent.

## 1) Primeira configuracao

```bash
cd /home/$USER/repo/private/luck-mpc

docker compose build mcp
docker compose up -d postgres ollama mcp
make migrate
docker compose exec ollama ollama pull nomic-embed-text
make index PROJECT=meu-projeto ROOT=/caminho/absoluto/do/repo
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
cd /home/$USER/repo/private/luck-mpc
docker compose up -d postgres ollama mcp
make index PROJECT=meu-projeto ROOT=/caminho/absoluto/do/repo
```

### Parar no fim do dia
```bash
cd /home/$USER/repo/private/luck-mpc
docker compose down
```

### Quando rodar migrate
```bash
cd /home/$USER/repo/private/luck-mpc
make migrate
```

### Quando rodar index-full
Use quando quiser reconstruir toda a base de contexto do projeto:
```bash
cd /home/$USER/repo/private/luck-mpc
make index-full PROJECT=meu-projeto ROOT=/caminho/absoluto/do/repo
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
