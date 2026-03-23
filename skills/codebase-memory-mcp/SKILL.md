---
name: codebase-memory-mcp
description: |
  Default skill for day-to-day work inside repositories under /home/luckstyle/repo.
  Trigger whenever the user is exploring, editing, reviewing, debugging, or
  researching code in a repo, especially Terraform repos under iac/, Lambda
  repos under lambda/, and personal repos under private/. Automatically use the
  local luck-mpc tools for bootstrap, docs/files discovery, cross-repo impact,
  and durable engineering memory. When the repo is Terraform-related, strongly
  bias toward multi-repo search and also follow vex-tf guidance when validating
  Terraform patterns. If MCP results look stale, tell the user to run mcp-index
  in the target repo.
---

# Codebase Memory MCP

Use this skill by default for repository work when the local `luck-mpc` MCP is
configured. The goal is to make MCP usage automatic and predictable without
spamming tools on trivial requests.

## Core Rule

Treat the MCP as the default research layer for repository context inside
`/home/luckstyle/repo`.

Default assumption:

- if the user is working in a repository under `/home/luckstyle/repo`, use
  codebase memory unless the task is trivial or the user explicitly says not to
- at the beginning of the session, act as if the convention is:
  `use codebase memory for this session`

Use the MCP first for:

- local repo discovery
- docs discovery
- cross-repo discovery
- saved engineering memory

Do not wait for the user to remember tool names.

## Repository Topology

The main repository root is:

```text
/home/luckstyle/repo
```

Important groups:

- `iac/`
  Main Terraform repositories. These are the highest-priority repos for this
  skill. Cross-repo relations are common here, but not guaranteed.

- `lambda/`
  Lambda repositories. Usually Python, but not always. These often relate to
  Terraform repos because Terraform provisions Lambda resources.

- `private/`
  Personal/private repositories.

- other repos directly inside `/home/luckstyle/repo`
  Still valid and may be relevant. Do not ignore them only because they are not
  inside `iac/`, `lambda/`, or `private/`.

## Terraform Bias

Terraform repositories are the most common and highest-priority use case.

When the current repo is under `/home/luckstyle/repo/iac/`:

1. default to codebase memory usage early
2. use `search_across_repos` more aggressively for impact and reuse
3. if the task is Terraform validation/review/audit, also follow `vex-tf`
4. prefer module references via git source when the user needs reusable modules
5. never recommend local repo path references as the preferred Terraform module
   source

Newer-pattern Terraform repos that should be followed as closely as possible:

- `iac-intelliscan`
- `iac-mkt-diagnostico-maturidade`
- `iac-core-boundary`
- `iac-core-vault`
- `iac-mcp`

Treat these as strong pattern references when relevant.

## Lambda Bias

When the current repo is under `/home/luckstyle/repo/lambda/`:

1. assume Lambda/Terraform relationships may matter
2. search `iac/` repos when infrastructure, permissions, triggers, queues,
   topics, buckets, or schedules are involved
3. syntax checks and local simulation are acceptable
4. do not frame Lambda validation around local compile/build guarantees if the
   real validation has to happen in AWS

## Session Start

When starting meaningful work in a repository:

1. Infer the current repo name from the current working directory basename.
2. If the repo is not obviously cataloged yet, call `repo_register` with:
   - `name`: repo basename
   - `root_path`: current absolute repo path when known
   - keep description/tags minimal unless the user already gave them
3. Bootstrap context in this order when the task is non-trivial:
   - `repo_find_docs` for README/ADR/docs
   - `repo_find_files` for obvious modules/areas
   - `project_brief` for curated memory
4. If the repo looks like Terraform or the task has impact potential, also run
   `search_across_repos` early.

Skip the full bootstrap only for very narrow requests where repo context is not
needed.

## Tool Selection

Use the tools like this:

- `repo_find_docs`
  Use for setup, onboarding, architecture, ADR, README, runbook, config guide.

- `repo_find_files`
  Use for exact modules, paths, symbols, routes, Terraform resources, React
  components, Ansible roles/modules, or when the user is asking "where is X?".

- `repo_search`
  Use for similar logic, implementation patterns, semantic discovery inside one
  repo or an explicit repo set.

- `search_across_repos`
  Use for:
  - "where else does this exist?"
  - "which repos are impacted?"
  - "who consumes this contract?"
  - "where is similar logic used across repos?"
  - Terraform/Lambda relationships
  - contracts, endpoints, env vars, IAM/resource names, queues, topics, buckets

- `context_search`
  Use before touching sensitive areas when prior decisions/gotchas might exist.

- `context_add`
  Use after durable technical decisions, gotchas, invariants, or recovery notes.

## Automatic Behaviors

### For single-repo implementation work

Default sequence:

1. `repo_find_docs`
2. `repo_find_files`
3. `project_brief`
4. `context_search` only if the area is risky or historical decisions matter

### For cross-repo or impact questions

Default sequence:

1. `search_across_repos`
2. `repo_find_files` or `repo_find_docs` in the top candidate repos
3. `repo_search` for deeper implementation comparison where needed

### For Terraform repositories

Default sequence:

1. `repo_find_docs`
2. `repo_find_files`
3. `search_across_repos`
4. `project_brief`
5. if the task is validation/review/audit, also apply `vex-tf`

### For Lambda repositories

Default sequence:

1. `repo_find_docs`
2. `repo_find_files`
3. `project_brief`
4. `search_across_repos` whenever infrastructure coupling is plausible

### After important decisions

Persist a concise entry with `context_add`.

Prefer:

- `kind="summary"` for decisions and architecture summaries
- `kind="memory"` for gotchas and operational notes

Keep entries short and useful:

- decision
- reason
- impact

## When To Ask For Reindex

If MCP results are sparse, obviously stale, or miss files that should exist,
assume the repo may not be indexed yet.

Tell the user clearly to run, from inside the target repository:

```bash
mcp-index
```

For a full rebuild:

```bash
mcp-index-full
```

Do not invent data when indexing is clearly stale.

## Guardrails

- Do not call every MCP tool on every request.
- Do not use semantic search first when the user is asking for an exact file,
  route, symbol, module, or resource.
- Do not skip cross-repo search for Terraform/Lambda work when infrastructure
  coupling is plausible.
- Do not save noisy or temporary context with `context_add`.
- Do not call `search_across_repos` for strictly local trivial edits.
- If the user explicitly says not to use MCP, obey that.

## Practical Prompts To Internally Follow

- "I need repo docs first."
- "Use codebase memory at the beginning of the session."
- "I need exact files before semantic similarity."
- "If this can affect multiple repos, use cross-repo search."
- "Terraform repos deserve stronger pattern and impact checks."
- "Lambda repos may depend on Terraform repos."
- "If this is a durable decision, save it."
