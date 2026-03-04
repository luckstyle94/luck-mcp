# Quickstart (EN)

Fast guide to get the MCP server running and use it in Cursor in a few minutes.

## 1) First-time setup

```bash
cd /home/luckstyle/repo/private/luck-mpc

docker compose build mcp
docker compose up -d postgres ollama mcp
make migrate
docker compose exec ollama ollama pull nomic-embed-text
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
cd /home/luckstyle/repo/private/luck-mpc
docker compose up -d postgres ollama mcp
```

### Stop at the end of the day
```bash
cd /home/luckstyle/repo/private/luck-mpc
docker compose down
```

### When to run migrate
```bash
cd /home/luckstyle/repo/private/luck-mpc
make migrate
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
