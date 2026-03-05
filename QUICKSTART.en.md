# Quickstart (EN)

Fast guide to get the MCP server running and use it in Cursor in a few minutes.

## Daily cheatsheet (30s summary)
1. Start day:
```bash
cd /home/$USER/repo/private/luck-mpc
make up
make index PROJECT=my-project ROOT=/absolute/path/to/project
```

2. Work with AI (in chat):
- `project_brief` at session start
- `context_search` before touching critical areas
- `context_add` after important decisions

3. End day (optional):
```bash
cd /home/$USER/repo/private/luck-mpc
make down
```

## What are index, reindex and incremental?
- `make index`: updates context only for changes (new/modified/deleted files). This is the daily command.
- `make index-full`: rebuilds indexing for the whole project from zero. Use when you want a full refresh.
- `incremental`: means "only differences". Faster.
- `full reindex`: means "all files again". Slower.

When to use each command:
- Start work: `make up` + `make index ...`
- New migration added: `make migrate`
- Large code changes: `make index ...`
- Need to reset indexed context: `make index-full ...`
- End of day: `make down` (optional)

## Useful aliases (mcp-up, mcp-down, mcp-index, mcp-index-full)
Create shortcuts:

```bash
alias mcp-up='cd /home/$USER/repo/private/luck-mpc && make up'
alias mcp-down='cd /home/$USER/repo/private/luck-mpc && make down'
alias mcp-index='(cd /home/$USER/repo/private/luck-mpc && make index PROJECT="$(basename "$PWD")" ROOT="$PWD")'
alias mcp-index-full='(cd /home/$USER/repo/private/luck-mpc && make index-full PROJECT="$(basename "$PWD")" ROOT="$PWD")'
```

Persist in bash:

```bash
cat <<'EOF' >> ~/.bashrc
alias mcp-up='cd /home/$USER/repo/private/luck-mpc && make up'
alias mcp-down='cd /home/$USER/repo/private/luck-mpc && make down'
alias mcp-index='(cd /home/$USER/repo/private/luck-mpc && make index PROJECT="$(basename "$PWD")" ROOT="$PWD")'
alias mcp-index-full='(cd /home/$USER/repo/private/luck-mpc && make index-full PROJECT="$(basename "$PWD")" ROOT="$PWD")'
EOF
source ~/.bashrc
```

Usage:
- `mcp-up`
- `mcp-down`
- `mcp-index`
- `mcp-index-full`

Important:
- `mcp-index` and `mcp-index-full` use the current folder as project, so run them inside the repo you want to index.

## 0) Where to run commands
Run all commands below in your local terminal, inside the MCP folder:

```bash
cd /home/$USER/repo/private/luck-mpc
```

Important:
- `make index` is always executed here in `luck-mpc`.
- `ROOT` points to the project you want to index (example: `/home/$USER/repo/private/my-api`).
- Tools (`context_add`, `context_search`, `project_brief`) are used in the agent chat.

### Direct example with `/home/my-project1`
Terminal (always in MCP repo):
```bash
cd /home/$USER/repo/private/luck-mpc
make up
make index PROJECT=my-project1 ROOT=/home/my-project1
```

In AI chat:
```text
Use project_brief for project "my-project1" with max_items=20.
Use context_search for project "my-project1" with query "auth" and k=8.
Use context_add for project "my-project1" with kind="summary", importance=5, content="Decision: ...".
```

## 1) First-time setup

```bash
cd /home/$USER/repo/private/luck-mpc

docker compose build mcp
docker compose up -d postgres ollama mcp
make migrate
docker compose exec ollama ollama pull nomic-embed-text
make index PROJECT=my-project ROOT=/absolute/path/to/repo
```

## 2) Configure in Cursor

Use this MCP configuration:

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

Then:
1. Save the configuration.
2. Reload Window in Cursor.
3. Check that tools are available: `context_add`, `context_search`, `project_brief`.

## 3) Daily usage

### Start at the beginning of the day
```bash
cd /home/$USER/repo/private/luck-mpc
docker compose up -d postgres ollama mcp
make index PROJECT=my-project ROOT=/absolute/path/to/repo
```

### Stop at the end of the day
```bash
cd /home/$USER/repo/private/luck-mpc
docker compose down
```

### When to run migrate
```bash
cd /home/$USER/repo/private/luck-mpc
make migrate
```

### When to run index-full
Use this when you want to rebuild the full project context base:
```bash
cd /home/$USER/repo/private/luck-mpc
make index-full PROJECT=my-project ROOT=/absolute/path/to/repo
```

## 4) Ready-to-use prompts for AI

Load context at session start:
```text
Use project_brief on project "my-project" with max_items=20 and show me an objective summary.
```

Search before coding:
```text
Use context_search on project "my-project" with query "authentication flow" and k=8.
```

Save an important decision:
```text
Use context_add on project "my-project" with kind="summary", tags=["architecture"], importance=5,
content="Decision: ... Reason: ... Impact: ...".
```

Save a gotcha:
```text
Use context_add on project "my-project" with kind="memory", tags=["gotcha"], importance=4,
content="Problem: ... Cause: ... Solution: ...".
```

## 5) Quick commands

Status:
```bash
docker compose ps
```

MCP logs:
```bash
docker logs --tail=200 luck-mpc-server
```

## 6) If Cursor has issues (loading tools)

1. Ensure containers are up:
```bash
docker compose ps
```

2. Rebuild MCP image:
```bash
docker compose build mcp
```

3. Reload Cursor window.

---

Full documentation:
- [README.md](./README.md)
- [README.en.md](./README.en.md)
- [QUICKSTART.md](./QUICKSTART.md)
