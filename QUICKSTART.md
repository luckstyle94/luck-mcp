# Quickstart (PT-BR)

Guia rapido para colocar o MCP no ar e usar no Cursor ou Codex em poucos minutos.

## Cheatsheet diario (resumo de 30s)
1. Abrir o dia:
```bash
cd /home/$USER/path/to/luck-mcp
make up
make health
make index PROJECT=meu-projeto ROOT=/caminho/absoluto/do/projeto
```

2. Trabalhar com IA (no chat):
- No Codex CLI, comece com: `use codebase memory for this session`
- `project_brief` no inicio da sessao
- `repo_register` se quiser cadastrar descricao e tags do repo
- `search_across_repos` para descobrir em quais repos o tema aparece
- `repo_find_files` para achar arquivos e modulos
- `repo_find_docs` para achar README/ADR/docs
- `repo_search` para busca por tema ou logica parecida
- `context_search` antes de mexer em partes criticas
- `context_add` depois de decisoes importantes

3. Fechar o dia (opcional):
```bash
cd /home/$USER/path/to/luck-mcp
make down
```

## O que e index, reindex e incremental?
- `make index`: atualiza contexto automaticamente so do que mudou (arquivos novos/alterados/removidos). Esse e o comando do dia a dia e so aplica migrations pendentes antes de indexar.
- `make index-full`: refaz toda a indexacao do projeto do zero. Use quando quiser reconstruir tudo.
- `make health`: confere banco, schema e Ollama/modelo e mostra o proximo passo se algo estiver quebrado.
- `incremental`: significa "somente diferencas". Mais rapido.
- `reindex completo`: significa "todos os arquivos novamente". Mais lento.

Quando usar cada comando:
- Comecou a trabalhar: `make up` + `make index ...`
- Entrou migration nova: `make migrate`
- Mudou muito codigo: `make index ...`
- Quer resetar contexto indexado: `make index-full ...`
- Terminou o dia: `make down` (opcional)

## Uso automatico no Codex
O Codex CLI ja pode usar este MCP de forma quase automatica porque existe uma skill instalada para isso.

Convencao recomendada no inicio da sessao:
```text
use codebase memory for this session
```

O que essa skill faz:
- puxa docs e arquivos relevantes
- usa `search_across_repos` quando houver impacto multi-repo
- usa `project_brief` e `context_search` para memoria
- sugere `mcp-index` quando o indice parecer velho ou incompleto

### Como instalar a skill
Como este repositorio e publico, o ideal e fazer um fork e adaptar a skill antes de instalar.

Edite pelo menos:
- a raiz dos seus repositorios
- os grupos principais que voce usa
- exemplos internos e prioridades de busca

```bash
mkdir -p ~/.codex/skills
ln -s /home/$USER/path/to/luck-mcp/skills/codebase-memory-mcp ~/.codex/skills/codebase-memory-mcp
```

Se preferir copiar:

```bash
mkdir -p ~/.codex/skills/codebase-memory-mcp
cp -R /home/$USER/path/to/luck-mcp/skills/codebase-memory-mcp/. ~/.codex/skills/codebase-memory-mcp/
```

Organizacao sugerida dos repos:
- `/home/$USER/repos/iac`: Terraform, prioridade mais alta
- `/home/$USER/repos/lambda`: Lambdas, geralmente Python
- `/home/$USER/repos/private`: repos pessoais

Para Terraform, o Codex deve seguir padroes dos repos mais novos quando fizer sentido e pode combinar este MCP com a skill `vex-tf`.

## Aliases uteis (mcp-up, mcp-down, mcp-index, mcp-index-full)
Crie os atalhos:

```bash
alias mcp-up='cd /home/$USER/path/to/luck-mcp && make up'
alias mcp-down='cd /home/$USER/path/to/luck-mcp && make down'
alias mcp-index='project_root="$PWD"; project_name="$(basename "$project_root")"; (cd /home/$USER/path/to/luck-mcp && make index PROJECT="$project_name" ROOT="$project_root")'
alias mcp-index-full='project_root="$PWD"; project_name="$(basename "$project_root")"; (cd /home/$USER/path/to/luck-mcp && make index-full PROJECT="$project_name" ROOT="$project_root")'
```

Salvar no bash:

```bash
cat <<'EOF' >> ~/.bashrc
alias mcp-up='cd /home/$USER/path/to/luck-mcp && make up'
alias mcp-down='cd /home/$USER/path/to/luck-mcp && make down'
alias mcp-index='project_root="$PWD"; project_name="$(basename "$project_root")"; (cd /home/$USER/path/to/luck-mcp && make index PROJECT="$project_name" ROOT="$project_root")'
alias mcp-index-full='project_root="$PWD"; project_name="$(basename "$project_root")"; (cd /home/$USER/path/to/luck-mcp && make index-full PROJECT="$project_name" ROOT="$project_root")'
EOF
source ~/.bashrc
```

Uso:
- `mcp-up`
- `mcp-down`
- `mcp-index`
- `mcp-index-full`

Importante:
- `mcp-index` e `mcp-index-full` usam a pasta atual como projeto, entao rode esses comandos dentro do repo que deseja indexar.

## 0) Onde rodar os comandos
Rode todos os comandos abaixo no seu terminal local, dentro da pasta do MCP:

```bash
cd /home/$USER/path/to/luck-mcp
```

Importante:
- `make index` sempre roda aqui no `luck-mcp`.
- O `ROOT` aponta para o projeto que voce quer indexar (ex.: `/home/$USER/repos/private/meu-api`).
- As tools (`repo_list`, `repo_register`, `search_across_repos`, `repo_search`, `repo_find_files`, `repo_find_docs`, `context_add`, `context_search`, `project_brief`) sao usadas no chat do agent.

### Exemplo direto com `/home/$USER/repos/meu-projeto1`
Terminal (sempre no repo MCP):
```bash
cd /home/$USER/path/to/luck-mcp
make up
make index PROJECT=meu-projeto1 ROOT=/home/$USER/repos/meu-projeto1
```

No chat da IA:
```text
Use codebase memory for this session.
Use repo_register com name="meu-projeto1", root_path="/home/$USER/repos/meu-projeto1", description="Descricao curta do repo", tags=["backend","auth"].
Use search_across_repos com query="auth" e k=5.
Use repo_find_files com repos=["meu-projeto1"] query="auth" e k=10.
Use repo_find_docs com repos=["meu-projeto1"] query="arquitetura" e k=5.
Use repo_search com repos=["meu-projeto1"] query="auth" mode="hybrid" e k=8.
Use project_brief no projeto "meu-projeto1" com max_items=20.
Use context_search no projeto "meu-projeto1" com query "auth" e k=8.
Use context_add no projeto "meu-projeto1" com kind="summary", importance=5, content="Decisao: ...".
```

## 1) Primeira configuracao

```bash
cd /home/$USER/path/to/luck-mcp

docker compose build mcp
docker compose up -d postgres ollama mcp
make health
make migrate
docker compose exec ollama ollama pull nomic-embed-text
make index PROJECT=meu-projeto ROOT=/caminho/absoluto/do/repo
```

## 2) Configurar no Cursor

Use esta configuracao MCP:

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

Depois:
1. Salve a configuracao.
2. Reload Window no Cursor.
3. Verifique se as tools aparecem: `repo_list`, `repo_register`, `search_across_repos`, `repo_search`, `repo_find_files`, `repo_find_docs`, `context_add`, `context_search`, `project_brief`.

## 3) Uso diario

### Iniciar no comeco do dia
```bash
cd /home/$USER/path/to/luck-mcp
docker compose up -d postgres ollama mcp
make health
make index PROJECT=meu-projeto ROOT=/caminho/absoluto/do/repo
```

### Parar no fim do dia
```bash
cd /home/$USER/path/to/luck-mcp
docker compose down
```

### Quando rodar migrate
```bash
cd /home/$USER/path/to/luck-mcp
make migrate
```

### Quando rodar index-full
Use quando quiser reconstruir toda a base de contexto do projeto:
```bash
cd /home/$USER/path/to/luck-mcp
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
docker logs --tail=200 luck-mcp-server
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
