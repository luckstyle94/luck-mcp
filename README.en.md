# luck-mpc: persistent memory for AI agents via MCP

## THIS PROJECT WAS 100% CREATED USING AI (CODEX 5.3)

## Language Versions
- Portuguese (full): [README.md](./README.md)
- Portuguese (quickstart): [QUICKSTART.md](./QUICKSTART.md)
- English (full): [README.en.md](./README.en.md)
- English (quickstart): [QUICKSTART.en.md](./QUICKSTART.en.md)

## 1) What this project does (simple explanation)
This project provides a local MCP server to store and retrieve working context.

In practice, this lets your agent (Cursor, Codex CLI, Claude Code, VSCode with MCP support) keep persistent memory across sessions.

You can save:
- architecture decisions
- gotchas
- task summaries
- useful code context

Then you can search by meaning (semantic search), not only exact text.

## 2) How it works under the hood
Simplified architecture:

```text
Agent (Cursor / Codex / Claude / VSCode)
        |  MCP (STDIO)
        v
MCP Server (Go)
        |
        +--> Postgres + pgvector (persistence)
        |
        +--> Ollama (local embeddings)
```

## 3) Prerequisites
You need:
- Docker
- Docker Compose

You do not need to install Postgres or Ollama manually.

## 4) Initial setup (first time)
Run exactly in this order:

```bash
cd /home/luckstyle/repo/private/luck-mpc

docker compose build mcp
docker compose up -d postgres ollama mcp
make migrate
docker compose exec ollama ollama pull nomic-embed-text
```

What each command does:
1. `build mcp`: builds the local MCP server image.
2. `up -d postgres ollama mcp`: starts database, embeddings service, and base MCP container.
3. `make migrate`: applies DB schema (`0001`, `0002`, `0003`).
4. `ollama pull`: downloads the embedding model.

## 5) Daily routine (normal usage)
### Start environment at the beginning of the day
```bash
cd /home/luckstyle/repo/private/luck-mpc
docker compose up -d postgres ollama mcp
```

### Stop environment at the end of the day
```bash
cd /home/luckstyle/repo/private/luck-mpc
docker compose down
```

### When should you run `make migrate`?
Run it when:
- this is the first environment startup
- new migrations are added to the repository
- you want to ensure schema is aligned

Command:
```bash
cd /home/luckstyle/repo/private/luck-mpc
make migrate
```

## 6) Configure in Cursor (recommended)
Use this MCP configuration in Cursor:

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

Important notes:
- This approach uses `docker exec` and avoids common timeout issues from `docker compose run`.
- Container `luck-mpc-server` must be running (`docker compose up -d ...`).
- If you want a default project without sending `project` every time, add to `args`:
  - `"-e", "MCP_PROJECT_DEFAULT=my-project"`

After saving Cursor configuration:
1. Reload Window
2. Confirm MCP is `ready`
3. Confirm tools are listed: `context_add`, `context_search`, `project_brief`

## 7) Configure in other clients (Codex CLI, Claude Code, VSCode)
General rule: any MCP client that accepts `command + args` can use the same command as Cursor.

Base command:
```bash
docker exec -e LOG_LEVEL=error -i luck-mpc-server /mcp-server
```

Optional for clients that allow env vars:
```bash
docker exec -e LOG_LEVEL=error -e MCP_PROJECT_DEFAULT=my-project -i luck-mpc-server /mcp-server
```

## 8) How to use it day to day with AI
Available tools:
- `context_add`
- `context_search`
- `project_brief`

### 8.1 Recommended workflow
1. Session start:
- call `project_brief` to load core context

2. Before changing critical areas:
- call `context_search` with an objective query

3. After an important decision:
- call `context_add` with `kind=summary` or `kind=memory`

4. End of large task:
- save final summary with high `importance`

### 8.2 Ready-to-use prompts
Session start:

```text
Use project_brief for project "my-project" with max_items 20 and show me an objective summary.
```

Search context before coding:

```text
Use context_search on project "my-project" with query "authentication flow" and k=8.
```

Save architecture decision:

```text
Use context_add to save in project "my-project":
kind="summary", tags=["architecture","auth"], importance=5,
content="Decision: ..."
```

Save debugging gotcha:

```text
Use context_add in project "my-project" with kind="memory",
tags=["gotcha","deploy"], importance=4,
content="Problem: ... Cause: ... Solution: ..."
```

### 8.3 Recommended data format
- `project`: stable project name (example: `nexus-api`)
- `kind`:
  - `summary` for official decisions and final summaries
  - `memory` for learnings/gotchas
  - `note` for quick notes
  - `chunk` for short snippets
- `tags`: 2 to 5 consistent tags
- `importance`:
  - `5`: critical
  - `4`: very important
  - `3`: relevant
  - `0-2`: lightweight context

## 9) Tool JSON examples
### context_add
```json
{
  "project": "my-project",
  "kind": "summary",
  "path": "internal/auth/service.go",
  "tags": ["auth", "architecture"],
  "content": "Decision: refresh token with rotation and 7-day expiration.",
  "importance": 5
}
```

### context_search
```json
{
  "project": "my-project",
  "query": "authentication flow",
  "k": 8,
  "path_prefix": "internal/auth/",
  "tags": ["auth"],
  "kind": "any"
}
```

### project_brief
```json
{
  "project": "my-project",
  "max_items": 20
}
```

## 10) Best practices for useful memory
- Save less, but save better.
- Prefer `summary` for finalized decisions.
- Avoid vague text; write "decision + reason + impact".
- Standardize tags by domain (`auth`, `billing`, `infra`, `db`).
- When an old decision changes, save a new summary explaining the change.

## 11) Troubleshooting (common problems)
### Cursor stuck on "loading tools"
Checklist:
1. Run `docker compose ps` and confirm `luck-mpc-server`, `luck-mpc-postgres`, `luck-mpc-ollama` are `Up`
2. Confirm MCP config is using `docker exec ... /mcp-server`
3. Run `docker compose build mcp` after code changes
4. Reload Window in Cursor

### "model not found" in Ollama
Run:
```bash
docker compose exec ollama ollama pull nomic-embed-text
```

### Database/schema error
Run:
```bash
make migrate
```

### Old database with duplicates
Migration `0003` removes duplicates by `(project, content_hash)` keeping the newest row, then recreates the unique index.

## 12) Quick reference commands
Start environment:
```bash
docker compose up -d postgres ollama mcp
```

Apply migrations:
```bash
make migrate
```

Download model:
```bash
docker compose exec ollama ollama pull nomic-embed-text
```

Check status:
```bash
docker compose ps
```

Check MCP logs:
```bash
docker logs --tail=200 luck-mpc-server
```

Stop everything:
```bash
docker compose down
```

## 13) Project structure
```text
.
├─ cmd/
│  └─ mcp-server/
│     └─ main.go
├─ internal/
│  ├─ config/
│  ├─ db/
│  ├─ domain/
│  ├─ embeddings/
│  ├─ repository/
│  ├─ service/
│  └─ transport/mcp/
├─ migrations/
│  ├─ 0001_init.up.sql
│  ├─ 0001_init.down.sql
│  ├─ 0002_dedupe_index.up.sql
│  ├─ 0002_dedupe_index.down.sql
│  ├─ 0003_dedupe_existing_hashes.up.sql
│  └─ 0003_dedupe_existing_hashes.down.sql
├─ docker-compose.yml
├─ Dockerfile
├─ Makefile
├─ go.mod
├─ go.sum
└─ README.md
```

## 14) Environment variables
- `DATABASE_URL` (required)
- `OLLAMA_URL` (default `http://ollama:11434`)
- `OLLAMA_EMBED_MODEL` (default `nomic-embed-text`)
- `MCP_PROJECT_DEFAULT` (optional)
- `LOG_LEVEL` (default `info`)

## 15) MVP scope and limits
This MCP is explicit memory: it stores and searches what you ask it to.

It does not automatically index your repository in this MVP.
