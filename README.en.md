# luck-mcp: multi-repo codebase memory for AI agents via MCP

## THIS PROJECT WAS 100% CREATED USING AI (CODEX 5.3)

## Language Versions
- Portuguese (full): [README.md](./README.md)
- Portuguese (quickstart): [QUICKSTART.md](./QUICKSTART.md)
- English (full): [README.en.md](./README.en.md)
- English (quickstart): [QUICKSTART.en.md](./QUICKSTART.en.md)

## Daily Cheatsheet (copy and use)
Always run commands in terminal, inside the MCP folder:

```bash
cd /home/$USER/path/to/luck-mcp
```

1. When you start your day/project:
```bash
make up
make health
make index PROJECT=my-project ROOT=/absolute/path/to/project
```

2. During work (in AI chat, not in terminal):
- In Codex CLI, start the session with: `use codebase memory for this session`
- Session start: ask `project_brief` for `my-project`
- If you want to catalog repo description and tags: use `repo_register`
- To see which repos contain the same topic/module/contract: use `search_across_repos`
- To find files and modules: use `repo_find_files`
- To find README, ADR, and docs: use `repo_find_docs`
- For topic/similar-logic search: use `repo_search`
- Before sensitive changes: ask `context_search`
- After important decisions: ask `context_add` with `kind="summary"` and `importance=5`

3. If you changed a lot of code and want fresh context:
```bash
make index PROJECT=my-project ROOT=/absolute/path/to/project
```

4. If you want a full rebuild for this project:
```bash
make index-full PROJECT=my-project ROOT=/absolute/path/to/project
```

5. End of day (optional):
```bash
make down
```

## What each command means (plain language)
- `make up`: starts containers (Postgres, Ollama, MCP). Use when starting work.
- `make health`: checks database, schema, and Ollama/model readiness and shows the next step if something is missing.
- `make migrate`: applies only pending migrations and records versions/checksums. Use it on first setup, when new migrations are added, or when you want a manual schema sync.
- `make index PROJECT=... ROOT=...`: incremental indexing. Reprocesses only new/changed files and removes indexed data for deleted files. Before indexing, it ensures schema is aligned and applies only pending migrations.
- `make index-full PROJECT=... ROOT=...`: full reindex. Reprocesses all files for the selected project. Use when you want to rebuild context from scratch.
- `make down`: stops containers. Use at end of day (optional).
- `docker compose build mcp`: rebuilds MCP image. Use when you changed code in this MCP repository.
- `docker compose exec ollama ollama pull nomic-embed-text`: downloads/updates embedding model. Use first time or when model is missing.

Quick definitions:
- `incremental index`: updates only what changed (faster for daily use).
- `full reindex`: rebuilds all indexed project memory (slower, maintenance/reset use).

## Automatic usage in Codex
The MCP is available to any configured MCP client, but the most automatic flow is currently prepared for Codex CLI.

How it works:
- there is a local Codex skill for `luck-mcp`
- it tells Codex to use docs discovery, file discovery, cross-repo search, and saved memory automatically for codebase work
- in practice, you do not need to remember every MCP tool name

Recommended convention at session start:
```text
use codebase memory for this session
```

This makes behavior more predictable even when the skill is already installed.

### How to install the Codex skill
Because this repository is public, the recommended path is:
1. fork this repository
2. edit the skill so it matches your own repository layout, priorities, and conventions
3. only then install the skill in Codex

What you will usually edit in the skill:
- your main repository root
- your main groups such as `iac/`, `lambda/`, `private/`, or equivalents
- which repository types should trigger earlier cross-repo search
- internal examples and references that should match your environment

After that, create the Codex skills folder and link this project's skill:

```bash
mkdir -p ~/.codex/skills
ln -s /home/$USER/path/to/luck-mcp/skills/codebase-memory-mcp ~/.codex/skills/codebase-memory-mcp
```

If you prefer copying instead of symlink:

```bash
mkdir -p ~/.codex/skills/codebase-memory-mcp
cp -R /home/$USER/path/to/luck-mcp/skills/codebase-memory-mcp/. ~/.codex/skills/codebase-memory-mcp/
```

## Suggested repository topology
A common layout is:

```text
/home/$USER/repos
```

Suggested organization:
- `iac/`: Terraform repositories; highest-priority group for this MCP
- `lambda/`: Lambda repositories; usually Python, but not always
- `private/`: personal/private repositories
- other repos directly under `/home/$USER/repos`: still relevant and should not be ignored

Expected behavior:
- for `iac/` repos, use MCP early and bias more strongly toward cross-repo search
- for `lambda/` repos, consider relationships with Terraform-managed infrastructure
- for Terraform validation/review in Codex, it makes sense to also use the `vex-tf` skill

Example Terraform repos that can be treated as stronger pattern references when relevant:
- `iac-example-app`
- `iac-example-platform`
- `iac-example-security`
- `iac-example-network`
- `iac-example-mcp`

Important preference:
- when suggesting reusable Terraform modules, prefer git source references
- avoid recommending local path references as the default pattern

## Useful aliases (terminal shortcuts)
To simplify daily usage, you can create aliases:

```bash
alias mcp-up='cd /home/$USER/path/to/luck-mcp && make up'
alias mcp-down='cd /home/$USER/path/to/luck-mcp && make down'
alias mcp-index='project_root="$PWD"; project_name="$(basename "$project_root")"; (cd /home/$USER/path/to/luck-mcp && make index PROJECT="$project_name" ROOT="$project_root")'
alias mcp-index-full='project_root="$PWD"; project_name="$(basename "$project_root")"; (cd /home/$USER/path/to/luck-mcp && make index-full PROJECT="$project_name" ROOT="$project_root")'
```

To persist in bash:

```bash
cat <<'EOF' >> ~/.bashrc
alias mcp-up='cd /home/$USER/path/to/luck-mcp && make up'
alias mcp-down='cd /home/$USER/path/to/luck-mcp && make down'
alias mcp-index='project_root="$PWD"; project_name="$(basename "$project_root")"; (cd /home/$USER/path/to/luck-mcp && make index PROJECT="$project_name" ROOT="$project_root")'
alias mcp-index-full='project_root="$PWD"; project_name="$(basename "$project_root")"; (cd /home/$USER/path/to/luck-mcp && make index-full PROJECT="$project_name" ROOT="$project_root")'
EOF
source ~/.bashrc
```

Then use:
- `mcp-up` to start services
- `mcp-down` to stop services
- `mcp-index` to index the current folder project (`$PWD`)
- `mcp-index-full` to fully reindex the current folder project

Note:
- run `mcp-index` and `mcp-index-full` from inside the repository you want to index.

## 1) What this project does (simple explanation)
This project provides a local MCP server to store and retrieve working context.

In practice, this lets your agent (Cursor, Codex CLI, Claude Code, VSCode with MCP support) keep persistent memory across sessions and use a multi-repo codebase research layer.

You can save:
- architecture decisions
- gotchas
- task summaries
- useful code context

Then you can search:
- by files and docs
- by meaning (semantic search)
- by cross-repo impact
- by related patterns across multiple repositories

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

## 3.1) Where to run each command (very important)
Run setup and maintenance commands in your local terminal, inside this MCP repository:

```bash
cd /home/$USER/path/to/luck-mcp
```

Practical rules:
- `make up`, `make down`, `make migrate`, `make index`, `make index-full`: run from `luck-mcp`.
- `ROOT` in `make index`: absolute path of the project you want to index (Go, Python, Terraform, React, etc.).
- MCP tools (`repo_list`, `repo_register`, `search_across_repos`, `repo_search`, `repo_find_files`, `repo_find_docs`, `context_add`, `context_search`, `project_brief`) are used in your agent chat (Cursor/Codex/Claude), not in terminal.
- You do not need to manually enter containers for normal usage.

### Real example: I am working in `/home/$USER/repos/my-project1`
If you are working on this project, keep the same MCP project name, for example: `my-project1`.

In terminal (inside `luck-mcp` repo), run:
```bash
cd /home/$USER/path/to/luck-mcp
make up
make index PROJECT=my-project1 ROOT=/home/$USER/repos/my-project1
```

Then in AI chat (Cursor/Codex/Claude), call tools with that same project:
```text
Use repo_register with name="my-project1", root_path="/home/$USER/repos/my-project1", description="Short repo description", tags=["backend","auth"].
```

```text
Use search_across_repos with query="auth" and k=5 to see which repos contain that topic.
```

```text
Use project_brief for project "my-project1" with max_items=20.
```

```text
Use repo_find_files with repos=["my-project1"] query="auth" and k=10.
```

```text
Use repo_find_docs with repos=["my-project1"] query="authentication" and k=5.
```

```text
Use repo_search with repos=["my-project1"] query="authentication flow" mode="hybrid" and k=8.
```

```text
Use context_add for project "my-project1" with kind="summary", importance=5, content="Decision: ...".
```

Important summary:
- You can be coding inside `/home/$USER/repos/my-project1`.
- But `make ...` commands are always executed in MCP folder (`luck-mcp`).
- Tools are called in agent chat and should use consistent `project` value.

## 4) Initial setup (first time)
Run exactly in this order:

```bash
cd /home/$USER/path/to/luck-mcp

docker compose build mcp
docker compose up -d postgres ollama mcp
make migrate
docker compose exec ollama ollama pull nomic-embed-text
make index PROJECT=my-project ROOT=/absolute/path/to/repo
```

What each command does:
1. `build mcp`: builds the local MCP server image.
2. `up -d postgres ollama mcp`: starts database, embeddings service, and base MCP container.
3. `make migrate`: applies only pending DB migrations.
4. `ollama pull`: downloads the embedding model.
5. `make index`: runs the first automatic indexing for the project.

## 5) Daily routine (normal usage)
### Start environment at the beginning of the day
```bash
cd /home/$USER/path/to/luck-mcp
docker compose up -d postgres ollama mcp
make index PROJECT=my-project ROOT=/absolute/path/to/repo
```

### Stop environment at the end of the day
```bash
cd /home/$USER/path/to/luck-mcp
docker compose down
```

### When should you run `make migrate`?
Run it when:
- this is the first environment startup
- new migrations are added to the repository
- you want a manual schema sync

For daily work, `make index` already checks schema and applies only pending migrations.

Command:
```bash
cd /home/$USER/path/to/luck-mcp
make migrate
```

### How automatic indexing works
`make index` scans text files from your project (Go, Python, Terraform, Ansible, React, Markdown, SQL, etc.), creates embeddings, and stores chunks as `kind=chunk`.

Main behavior:
- indexes by `project` (each project is isolated in the database)
- default mode is `changed`: only new/changed files are reindexed
- automatically removes chunks for deleted files
- ignores binaries, secrets (`.env*`, keys), and large files (>1MB)

Recommended daily command:
```bash
make index PROJECT=my-project ROOT=/absolute/path/to/repo
```

When you need a full rebuild:
```bash
make index-full PROJECT=my-project ROOT=/absolute/path/to/repo
```

## 6) Configure in Cursor (recommended)
Use this MCP configuration in Cursor:

```json
{
  "mcpServers": {
    "luck-mcp": {
      "command": "docker",
      "args": [
        "exec",
        "-e",
        "LOG_LEVEL=error",
        "-i",
        "luck-mcp-server",
        "/mcp-server"
      ]
    }
  }
}
```

Important notes:
- This approach uses `docker exec` and avoids common timeout issues from `docker compose run`.
- Container `luck-mcp-server` must be running (`docker compose up -d ...`).
- If you want a default project without sending `project` every time, add to `args`:
  - `"-e", "MCP_PROJECT_DEFAULT=my-project"`

After saving Cursor configuration:
1. Reload Window
2. Confirm MCP is `ready`
3. Confirm tools are listed: `repo_list`, `repo_register`, `search_across_repos`, `repo_search`, `repo_find_files`, `repo_find_docs`, `context_add`, `context_search`, `project_brief`

## 7) Configure in other clients (Codex CLI, Claude Code, VSCode)
General rule: any MCP client that accepts `command + args` can use the same command as Cursor.

Base command:
```bash
docker exec -e LOG_LEVEL=error -i luck-mcp-server /mcp-server
```

Optional for clients that allow env vars:
```bash
docker exec -e LOG_LEVEL=error -e MCP_PROJECT_DEFAULT=my-project -i luck-mcp-server /mcp-server
```

## 8) How to use it day to day with AI
Available tools:
- `repo_list`
- `repo_register`
- `search_across_repos`
- `repo_search`
- `repo_find_files`
- `repo_find_docs`
- `context_add`
- `context_search`
- `project_brief`

### 8.0 Simple daily flow (beginner-friendly)
1. In terminal, inside `luck-mcp`, run:
```bash
make up
make index PROJECT=my-project ROOT=/absolute/path/to/project
```
2. In Cursor (or another agent), start by locating concrete context:
- `search_across_repos` to identify related or impacted repos
- `repo_find_docs` for docs
- `repo_find_files` for files/modules
- `repo_search` for topic/similar logic
3. Then load manual memory:
- `project_brief` for `my-project`
4. Before coding in sensitive areas:
- run `context_search` if you need saved manual memory
5. When you make an important decision:
- run `context_add` with `kind=summary` and high `importance`
5. End of day (optional):
```bash
make down
```

### 8.0.1 Recommended flow in Codex CLI
1. Enter the repo you will work on
2. Run `mcp-index` if the repo changed a lot
3. Start the session with:
```text
use codebase memory for this session
```
4. Let Codex automatically use:
- `repo_find_docs`
- `repo_find_files`
- `search_across_repos`
- `project_brief`
5. Save important decisions with `context_add`

### 8.1 Recommended workflow
1. Session start:
- use `search_across_repos` to identify related/impacted repos
- use `repo_find_docs` to locate base docs
- use `repo_find_files` to locate files/modules
- use `repo_search` to locate similar implementation or topic
- call `project_brief` to load core manual context

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
1. Run `docker compose ps` and confirm `luck-mcp-server`, `luck-mcp-postgres`, `luck-mcp-ollama` are `Up`
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

Index project (incremental):
```bash
make index PROJECT=my-project ROOT=/absolute/path/to/repo
```

Full reindex:
```bash
make index-full PROJECT=my-project ROOT=/absolute/path/to/repo
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
docker logs --tail=200 luck-mcp-server
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
│  ├─ indexer/
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
│  ├─ 0004_indexed_files.up.sql
│  └─ 0004_indexed_files.down.sql
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
