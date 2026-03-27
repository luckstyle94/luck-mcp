# Quickstart (EN)

Fast guide to get the MCP server running and use it in Cursor or Codex in a few minutes.

## Daily cheatsheet (30s summary)
1. Start day:
```bash
cd /home/$USER/path/to/luck-mcp
make up
make index PROJECT=my-project ROOT=/absolute/path/to/project
```

2. Work with AI (in chat):
- In Codex CLI, start with: `use codebase memory for this session`
- `project_brief` at session start
- `repo_register` if you want to save repo description and tags
- `search_across_repos` to discover which repos contain the topic
- `repo_find_files` to locate files and modules
- `repo_find_docs` to locate README/ADR/docs
- `repo_search` for topic or similar-logic search
- `context_search` before touching critical areas
- `context_add` after important decisions

3. End day (optional):
```bash
cd /home/$USER/path/to/luck-mcp
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

## Automatic usage in Codex
Codex CLI can already use this MCP in a near-automatic way because there is a dedicated skill installed for it.

Recommended session-start convention:
```text
use codebase memory for this session
```

What this skill does:
- pulls relevant docs and files
- uses `search_across_repos` for multi-repo impact/reuse
- uses `project_brief` and `context_search` for memory
- suggests `mcp-index` if the index looks stale

### How to install the skill
Because this repository is public, the best approach is to fork it and adapt the skill before installing it.

Edit at least:
- your repository root
- your main repository groups
- internal examples and search priorities

```bash
mkdir -p ~/.codex/skills
ln -s /home/$USER/path/to/luck-mcp/skills/codebase-memory-mcp ~/.codex/skills/codebase-memory-mcp
```

If you prefer copying:

```bash
mkdir -p ~/.codex/skills/codebase-memory-mcp
cp -R /home/$USER/path/to/luck-mcp/skills/codebase-memory-mcp/. ~/.codex/skills/codebase-memory-mcp/
```

Suggested repo layout:
- `/home/$USER/repos/iac`: Terraform, highest priority
- `/home/$USER/repos/lambda`: Lambdas, usually Python
- `/home/$USER/repos/private`: personal repos

For Terraform, Codex should follow newer repo patterns when relevant and can combine this MCP with the `vex-tf` skill.

## Useful aliases (mcp-up, mcp-down, mcp-index, mcp-index-full)
Create shortcuts:

```bash
alias mcp-up='cd /home/$USER/path/to/luck-mcp && make up'
alias mcp-down='cd /home/$USER/path/to/luck-mcp && make down'
alias mcp-index='project_root="$PWD"; project_name="$(basename "$project_root")"; (cd /home/$USER/path/to/luck-mcp && make index PROJECT="$project_name" ROOT="$project_root")'
alias mcp-index-full='project_root="$PWD"; project_name="$(basename "$project_root")"; (cd /home/$USER/path/to/luck-mcp && make index-full PROJECT="$project_name" ROOT="$project_root")'
```

Persist in bash:

```bash
cat <<'EOF' >> ~/.bashrc
alias mcp-up='cd /home/$USER/path/to/luck-mcp && make up'
alias mcp-down='cd /home/$USER/path/to/luck-mcp && make down'
alias mcp-index='project_root="$PWD"; project_name="$(basename "$project_root")"; (cd /home/$USER/path/to/luck-mcp && make index PROJECT="$project_name" ROOT="$project_root")'
alias mcp-index-full='project_root="$PWD"; project_name="$(basename "$project_root")"; (cd /home/$USER/path/to/luck-mcp && make index-full PROJECT="$project_name" ROOT="$project_root")'
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
cd /home/$USER/path/to/luck-mcp
```

Important:
- `make index` is always executed here in `luck-mcp`.
- `ROOT` points to the project you want to index (example: `/home/$USER/repos/private/my-api`).
- Tools (`repo_list`, `repo_register`, `search_across_repos`, `repo_search`, `repo_find_files`, `repo_find_docs`, `context_add`, `context_search`, `project_brief`) are used in the agent chat.

### Direct example with `/home/$USER/repos/my-project1`
Terminal (always in MCP repo):
```bash
cd /home/$USER/path/to/luck-mcp
make up
make index PROJECT=my-project1 ROOT=/home/$USER/repos/my-project1
```

In AI chat:
```text
Use codebase memory for this session.
Use repo_register with name="my-project1", root_path="/home/$USER/repos/my-project1", description="Short repo description", tags=["backend","auth"].
Use search_across_repos with query="auth" and k=5.
Use repo_find_files with repos=["my-project1"] query="auth" and k=10.
Use repo_find_docs with repos=["my-project1"] query="architecture" and k=5.
Use repo_search with repos=["my-project1"] query="auth" mode="hybrid" and k=8.
Use project_brief for project "my-project1" with max_items=20.
Use context_search for project "my-project1" with query "auth" and k=8.
Use context_add for project "my-project1" with kind="summary", importance=5, content="Decision: ...".
```

## 1) First-time setup

```bash
cd /home/$USER/path/to/luck-mcp

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

Then:
1. Save the configuration.
2. Reload Window in Cursor.
3. Check that tools are available: `repo_list`, `repo_register`, `search_across_repos`, `repo_search`, `repo_find_files`, `repo_find_docs`, `context_add`, `context_search`, `project_brief`.

## 3) Daily usage

### Start at the beginning of the day
```bash
cd /home/$USER/path/to/luck-mcp
docker compose up -d postgres ollama mcp
make index PROJECT=my-project ROOT=/absolute/path/to/repo
```

### Stop at the end of the day
```bash
cd /home/$USER/path/to/luck-mcp
docker compose down
```

### When to run migrate
```bash
cd /home/$USER/path/to/luck-mcp
make migrate
```

### When to run index-full
Use this when you want to rebuild the full project context base:
```bash
cd /home/$USER/path/to/luck-mcp
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
docker logs --tail=200 luck-mcp-server
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
