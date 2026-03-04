# luck-mpc: MCP Server de Memoria Persistente para Agents
# Completamente feito por IA ( CODEX 5.3 )

## 1) O que e MCP (Model Context Protocol)
MCP (Model Context Protocol) e um protocolo para conectar agentes de IA a ferramentas externas de forma padronizada.

Neste projeto, o MCP Server expoe ferramentas para salvar e recuperar memoria persistente de projeto. Isso permite que agentes como Cursor, VSCode + extensao compatível, Codex CLI e Claude consultem o mesmo contexto compartilhado.

## 2) O que este MCP resolve
Este MCP foi desenhado para reduzir problemas comuns de contexto em projetos grandes:

- limite de contexto em sessoes longas
- perda de conhecimento entre sessoes
- falta de memoria compartilhada entre diferentes agents

Com isso, voce ganha:

- persistencia de conhecimento em Postgres + pgvector
- busca semantica por similaridade
- bootstrap rapido de sessoes com `project_brief`

## 3) Arquitetura
Diagrama simplificado:

```text
Agent (Cursor / VSCode / Codex CLI / Claude)
        | (MCP STDIO / JSON-RPC)
        v
MCP Server (Go)
        |
        +--> Ollama (embeddings local via HTTP)
        |
        +--> Postgres + pgvector (persistente)
```

Fluxo resumido:

1. Agent chama tool MCP (`context_add`, `context_search`, `project_brief`).
2. MCP Server valida entrada e orquestra a acao.
3. Para `add/search`, gera embedding no Ollama.
4. Persiste/consulta dados no Postgres + pgvector.

## 4) Como subir localmente
### Pre-requisitos
- Docker + Docker Compose

### Subir stack
```bash
make up
```

Servicos do `docker-compose.yml`:

- `postgres`: banco com extensao pgvector
- `ollama`: servico local de embeddings
- `mcp`: servidor MCP em modo STDIO

Migrations:

- o `postgres` aplica automaticamente, no primeiro start do volume, os scripts:
  - `migrations/0001_init.up.sql`
  - `migrations/0002_dedupe_index.up.sql`
  - `migrations/0003_dedupe_existing_hashes.up.sql`
- para reaplicar manualmente no banco ja existente, use `make migrate` (executa todos os `*.up.sql`)
- se precisar reinicializar do zero via init script, remova o volume `pgdata` antes de subir novamente
- em upgrades de bases antigas, a `0003` remove duplicados por `(project, content_hash)` mantendo o registro mais recente

Volumes persistentes:

- `pgdata` para dados do Postgres
- `ollama` para modelos do Ollama

Parar stack:

```bash
make down
```

## 5) Como preparar o modelo no Ollama
O modelo padrao e `nomic-embed-text`.

Depois de subir o `ollama`, baixe o modelo:

```bash
docker compose exec ollama ollama pull nomic-embed-text
```

Sem esse pull, as tools que dependem de embedding podem falhar.

## 6) Como usar no Cursor
O Cursor precisa iniciar este servidor como processo local (STDIO).

Onde configurar:

- Cursor Settings > MCP (ou Integrations/MCP, conforme a versao)
- ou arquivo de configuracao MCP do Cursor (ex.: `~/.cursor/mcp.json`, dependendo da instalacao)

### Exemplo de configuracao MCP (ajuste caminho)
```json
{
  "mcpServers": {
    "luck-mpc": {
      "command": "/home/luckstyle/repo/private/luck-mpc/mcp-server",
      "args": [],
      "env": {
        "DATABASE_URL": "postgres://mcp:mcp@localhost:5432/mcp?sslmode=disable",
        "OLLAMA_URL": "http://localhost:11434",
        "OLLAMA_EMBED_MODEL": "nomic-embed-text",
        "LOG_LEVEL": "info"
      }
    }
  }
}
```

### Build do binario local
```bash
go build -o mcp-server ./cmd/mcp-server
```

### Como verificar se o Cursor chamou as tools
- confira logs do processo MCP (stdout/stderr da integracao)
- busque chamadas para `tools/list` e `tools/call`
- valide inserts no banco (`documents`, `doc_embeddings`)
- o servidor tolera `_meta` e outros campos extras no envelope de `tools/call` para compatibilidade entre clients

## 7) Como usar no VSCode
Cenarios comuns:

- extensao/agent no VSCode com suporte nativo a MCP
- ferramenta externa que inicia MCP local e integra ao agent do VSCode

Onde configurar no VSCode:

- nas configuracoes da extensao/agent MCP que voce estiver usando
- ou em arquivo de config da propria extensao/ferramenta (comando + env do servidor MCP)

Em ambos os casos, o principio e o mesmo:

1. rodar este servidor local em STDIO
2. apontar o cliente MCP para o comando do binario
3. definir variaveis de ambiente (`DATABASE_URL`, `OLLAMA_URL`, etc.)

Exemplo de comando local:

```bash
DATABASE_URL='postgres://mcp:mcp@localhost:5432/mcp?sslmode=disable' \
OLLAMA_URL='http://localhost:11434' \
OLLAMA_EMBED_MODEL='nomic-embed-text' \
./mcp-server
```

## 8) Exemplos praticos das tools
### `context_add`
Entrada:

```json
{
  "project": "nexus-api",
  "kind": "note",
  "path": "internal/auth/service.go",
  "tags": ["auth", "jwt"],
  "content": "Decisao: refresh token expira em 7 dias e e rotacionado a cada login.",
  "importance": 4
}
```

Resposta:

```json
{ "id": 123 }
```

### `context_add` (gotcha)
```json
{
  "project": "nexus-api",
  "kind": "memory",
  "tags": ["gotcha", "postgres"],
  "content": "Nao usar SERIAL novo em tabelas novas; padronizar BIGSERIAL para consistencia.",
  "importance": 3
}
```

### `context_add` (summary)
```json
{
  "project": "nexus-api",
  "kind": "summary",
  "content": "Resumo sprint 12: migracao de auth para middleware unico finalizada.",
  "importance": 5
}
```

### `context_search`
```json
{
  "project": "nexus-api",
  "query": "fluxo de auth",
  "k": 8
}
```

### `context_search` com filtros
```json
{
  "project": "nexus-api",
  "query": "juros compostos",
  "k": 5,
  "path_prefix": "internal/finance/",
  "tags": ["formula", "regra-negocio"],
  "kind": "note"
}
```

### `project_brief`
```json
{
  "project": "nexus-api",
  "max_items": 20
}
```

Resposta:

```json
{
  "brief": "Brief de contexto do projeto nexus-api: ..."
}
```

## 9) Boas praticas de uso (qualquer agent)
O que vale salvar:

- decisoes de arquitetura
- invariantes de dominio
- mapas de fluxo importantes
- gotchas e armadilhas recorrentes
- resumos periodicos (`kind=summary`)

Como evitar virar “lixao”:

- use `tags` de forma consistente
- preencha `kind` corretamente
- use `path` quando houver arquivo relevante
- use `importance` para sinalizar o que e critico
- consolide contexto em summaries de tempos em tempos

## 10) Estrutura de pastas e evolucao
```text
.
├─ cmd/
│  └─ mcp-server/
│     └─ main.go
├─ internal/
│  ├─ config/
│  │  └─ config.go
│  ├─ db/
│  │  └─ db.go
│  ├─ domain/
│  │  ├─ document.go
│  │  └─ errors.go
│  ├─ embeddings/
│  │  ├─ ollama_client.go
│  │  └─ models.go
│  ├─ repository/
│  │  ├─ document_repository.go
│  │  └─ postgres_document_repository.go
│  ├─ service/
│  │  └─ context_service.go
│  └─ transport/
│     └─ mcp/
│        ├─ server.go
│        ├─ tools.go
│        └─ types.go
├─ migrations/
│  ├─ 0001_init.up.sql
│  └─ 0001_init.down.sql
│  ├─ 0002_dedupe_index.up.sql
│  └─ 0002_dedupe_index.down.sql
│  ├─ 0003_dedupe_existing_hashes.up.sql
│  └─ 0003_dedupe_existing_hashes.down.sql
├─ docker-compose.yml
├─ Dockerfile
├─ Makefile
├─ go.mod
├─ go.sum
└─ README.md
```

### Onde evoluir depois
- `internal/transport/mcp`: novas tools
- `internal/service`: regras de negocio
- `internal/repository`: filtros/queries mais avancados
- `migrations`: novas estruturas de persistencia

## Variaveis de ambiente
- `DATABASE_URL` (obrigatorio)
- `OLLAMA_URL` (default `http://ollama:11434`)
- `OLLAMA_EMBED_MODEL` (default `nomic-embed-text`)
- `MCP_PROJECT_DEFAULT` (opcional, usado quando `project` nao for enviado nas tools)
- `LOG_LEVEL` (default `info`)

## Observacao sobre dimensao do vetor
A migration cria `VECTOR(768)` e o servico valida embeddings com 768 dimensoes.

Se trocar para um modelo com outra dimensao, ajuste:

1. `migrations/0001_init.up.sql` (coluna `embedding VECTOR(768)`)
2. `internal/config/config.go` (`defaultEmbeddingDim`)
3. recrie/aplique migration compativel

## Comandos uteis
```bash
make fmt
make test
make migrate
```

## Escopo deste MVP
Este projeto implementa apenas memoria explicita via tools MCP.

Nao ha indexacao automatica de repositorio neste MVP.
