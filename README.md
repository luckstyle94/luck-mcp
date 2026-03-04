# luck-mpc: memoria persistente para agents de IA via MCP

## PROJETO 100% CRIADO UTILIZANDO IA (CODEX 5.3)

## Versoes de documentacao
- Portugues (guia completo): [README.md](./README.md)
- Portugues (quickstart): [QUICKSTART.md](./QUICKSTART.md)
- English (full guide): [README.en.md](./README.en.md)
- English (quickstart): [QUICKSTART.en.md](./QUICKSTART.en.md)

## Cheatsheet Diario (copiar e usar)
Sempre rode comandos no terminal, dentro da pasta do MCP:

```bash
cd /home/$USER/repo/private/luck-mpc
```

1. Quando abrir o dia/projeto:
```bash
make up
make migrate
make index PROJECT=meu-projeto ROOT=/caminho/absoluto/do/projeto
```

2. Durante o trabalho (no chat da IA, nao no terminal):
- Inicio de sessao: peça `project_brief` para `meu-projeto`
- Antes de mudar algo sensivel: peça `context_search`
- Depois de decidir algo importante: peça `context_add` com `kind="summary"` e `importance=5`

3. Se alterou muito codigo e quer atualizar contexto:
```bash
make index PROJECT=meu-projeto ROOT=/caminho/absoluto/do/projeto
```

4. Se quer reconstruir tudo do zero para esse projeto:
```bash
make index-full PROJECT=meu-projeto ROOT=/caminho/absoluto/do/projeto
```

5. Quando encerrar o dia (opcional):
```bash
make down
```

## O que significa cada comando (sem jargao)
- `make up`: liga os containers (Postgres, Ollama, MCP). Use quando for comecar a trabalhar.
- `make migrate`: atualiza estrutura do banco. Use na primeira vez e sempre que entrar migration nova no repo.
- `make index PROJECT=... ROOT=...`: indexacao incremental. Reprocessa so arquivos novos/alterados e remove do banco o que foi apagado no projeto. Use no inicio do dia e depois de mudancas grandes.
- `make index-full PROJECT=... ROOT=...`: reindex completo. Reprocessa todos os arquivos do projeto selecionado. Use quando quiser reconstruir a base de contexto do zero.
- `make down`: desliga os containers. Use no fim do dia (opcional).
- `docker compose build mcp`: recompila imagem do MCP. Use quando voce alterou codigo deste repositorio MCP.
- `docker compose exec ollama ollama pull nomic-embed-text`: baixa/atualiza o modelo de embeddings. Use na primeira vez ou se faltar modelo.

Definicoes rapidas:
- `index incremental`: atualiza so o que mudou (mais rapido para uso diario).
- `reindex completo`: recria toda a memoria indexada daquele projeto (mais lento, usado em manutencao/correcao).

## 1) O que este projeto faz (explicacao simples)
Este projeto cria um servidor MCP local para guardar e recuperar contexto de trabalho.

Na pratica, isso permite que seu agent (Cursor, Codex CLI, Claude Code, VSCode com suporte MCP) tenha uma "memoria" persistente entre sessoes.

Voce salva:
- decisoes de arquitetura
- gotchas
- resumos de tarefas
- contexto util de codigo

E depois busca por significado (busca semantica), nao so por texto exato.

## 2) Como funciona por baixo
Arquitetura simplificada:

```text
Agent (Cursor / Codex / Claude / VSCode)
        |  MCP (STDIO)
        v
MCP Server (Go)
        |
        +--> Postgres + pgvector (persistencia)
        |
        +--> Ollama (embeddings locais)
```

## 3) Pre-requisitos
Voce precisa de:
- Docker
- Docker Compose

Nao precisa instalar Postgres nem Ollama manualmente.

## 3.1) Onde executar cada comando (muito importante)
Todos os comandos de setup e manutencao devem ser executados no seu terminal local, dentro da pasta deste repositiorio MCP:

```bash
cd /home/$USER/repo/private/luck-mpc
```

Regras praticas:
- `make up`, `make down`, `make migrate`, `make index`, `make index-full`: execute na pasta `luck-mpc`.
- `ROOT` do `make index`: e o caminho absoluto do projeto que voce quer indexar (pode ser Go, Python, Terraform, React etc.).
- As tools MCP (`context_add`, `context_search`, `project_brief`) voce usa no chat do agent (Cursor/Codex/Claude), nao no terminal.
- Nao precisa entrar em container manualmente para uso normal.

### Exemplo real: estou em `/home/meu-projeto1`
Se voce esta trabalhando nesse projeto, use sempre o mesmo nome de projeto no MCP, por exemplo: `meu-projeto1`.

No terminal (dentro do repo `luck-mpc`), rode:
```bash
cd /home/$USER/repo/private/luck-mpc
make up
make index PROJECT=meu-projeto1 ROOT=/home/meu-projeto1
```

Depois, no chat da IA (Cursor/Codex/Claude), use as tools com esse projeto:
```text
Use project_brief no projeto "meu-projeto1" com max_items=20.
```

```text
Use context_search no projeto "meu-projeto1" com query "fluxo de autenticacao" e k=8.
```

```text
Use context_add no projeto "meu-projeto1" com kind="summary", importance=5, content="Decisao: ...".
```

Resumo importante:
- Voce pode estar codando em `/home/meu-projeto1`.
- Mas os comandos `make ...` sempre sao executados na pasta do MCP (`luck-mpc`).
- As tools sao chamadas no chat do agent e precisam do campo `project` consistente.

## 4) Setup inicial (primeira vez)
Rode exatamente nesta ordem:

```bash
cd /home/$USER/repo/private/luck-mpc

docker compose build mcp
docker compose up -d postgres ollama mcp
make migrate
docker compose exec ollama ollama pull nomic-embed-text
make index PROJECT=meu-projeto ROOT=/caminho/absoluto/do/repo
```

O que cada comando faz:
1. `build mcp`: gera a imagem local do servidor MCP.
2. `up -d postgres ollama mcp`: sobe banco, embeddings e container base do MCP.
3. `make migrate`: aplica schema no banco (`0001`, `0002`, `0003`, `0004`).
4. `ollama pull`: baixa o modelo de embedding.
5. `make index`: faz a primeira indexacao automatica do projeto.

## 5) Rotina diaria (uso normal)
### Iniciar ambiente no comeco do dia
```bash
cd /home/$USER/repo/private/luck-mpc
docker compose up -d postgres ollama mcp
make index PROJECT=meu-projeto ROOT=/caminho/absoluto/do/repo
```

### Parar ambiente no fim do dia
```bash
cd /home/$USER/repo/private/luck-mpc
docker compose down
```

### Quando rodar `make migrate`?
Rode quando:
- for primeira subida do ambiente
- entrar migration nova no repositorio
- quiser garantir schema alinhado

Comando:
```bash
cd /home/$USER/repo/private/luck-mpc
make migrate
```

### Como funciona a indexacao automatica
O comando `make index` varre arquivos de texto do projeto (Go, Python, Terraform, Ansible, React, Markdown, SQL etc.), gera embeddings e salva chunks com `kind=chunk`.

Regras principais:
- indexa por `project` (cada projeto fica isolado no banco)
- modo padrao `changed`: indexa so arquivos novos/alterados
- remove automaticamente chunks de arquivos deletados
- ignora arquivos binarios, segredos (`.env*`, chaves) e arquivos grandes (> 1MB)

Comando diario recomendado:
```bash
make index PROJECT=meu-projeto ROOT=/caminho/absoluto/do/repo
```

Quando quiser reindexar tudo:
```bash
make index-full PROJECT=meu-projeto ROOT=/caminho/absoluto/do/repo
```

## 6) Configurar no Cursor (recomendado)
Use esta configuracao MCP no Cursor:

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

Observacoes importantes:
- Essa estrategia usa `docker exec` e evita problemas de timeout comuns de `docker compose run`.
- O container `luck-mpc-server` precisa estar ativo (`docker compose up -d ...`).
- Se quiser projeto padrao sem enviar `project` toda hora, adicione no `args`:
  - `"-e", "MCP_PROJECT_DEFAULT=meu-projeto"`

Depois de salvar configuracao no Cursor:
1. Reload Window
2. Verificar se o MCP ficou `ready`
3. Verificar se tools aparecem: `context_add`, `context_search`, `project_brief`

## 7) Configurar em outros clients (Codex CLI, Claude Code, VSCode)
Regra geral: qualquer cliente MCP que aceite `command + args` pode usar o mesmo comando do Cursor.

Comando base:
```bash
docker exec -e LOG_LEVEL=error -i luck-mpc-server /mcp-server
```

Para clients que aceitam env no comando, opcional:
```bash
docker exec -e LOG_LEVEL=error -e MCP_PROJECT_DEFAULT=meu-projeto -i luck-mpc-server /mcp-server
```

## 8) Como usar no dia a dia com a IA
As 3 tools disponiveis sao:
- `context_add`
- `context_search`
- `project_brief`

### 8.0 Fluxo diario simples (para leigos)
1. No terminal, dentro de `luck-mpc`, rode:
```bash
make up
make migrate
make index PROJECT=meu-projeto ROOT=/caminho/absoluto/do/projeto
```
2. No Cursor (ou outro agent), inicie a sessao pedindo:
- `project_brief` para `meu-projeto`
3. Antes de codar em area sensivel:
- rode `context_search` com uma query objetiva
4. Quando tomar decisao importante:
- rode `context_add` com `kind=summary` e `importance` alta
5. No fim do dia (opcional):
```bash
make down
```

### 8.1 Fluxo recomendado de uso
1. Inicio de sessao:
- pedir `project_brief` para carregar contexto principal

2. Antes de mexer em area critica:
- usar `context_search` com query objetiva

3. Depois de decidir algo importante:
- usar `context_add` com `kind=summary` ou `kind=memory`

4. Fim de tarefa grande:
- salvar resumo final com `importance` alta

### 8.2 Prompts prontos (copiar e colar)
Inicio da sessao:

```text
Use a tool project_brief para o projeto "meu-projeto" com max_items 20 e me mostre um resumo objetivo.
```

Buscar contexto antes de codar:

```text
Use context_search no projeto "meu-projeto" com query "fluxo de autenticacao" e k=8.
```

Salvar decisao de arquitetura:

```text
Use context_add para salvar no projeto "meu-projeto":
kind="summary", tags=["arquitetura","auth"], importance=5,
content="Decisao: ..."
```

Salvar gotcha de debugging:

```text
Use context_add no projeto "meu-projeto" com kind="memory",
tags=["gotcha","deploy"], importance=4,
content="Problema: ... Causa: ... Solucao: ..."
```

### 8.3 Formato de dados recomendado
- `project`: nome estavel do projeto (ex: `nexus-api`)
- `kind`:
  - `summary` para decisoes e resumos oficiais
  - `memory` para aprendizado/gotcha
  - `note` para anotacao rapida
  - `chunk` para trechos curtos
- `tags`: 2 a 5 tags consistentes
- `importance`:
  - `5`: critico
  - `4`: muito importante
  - `3`: relevante
  - `0-2`: contexto leve

## 9) Exemplos JSON das tools
### context_add
```json
{
  "project": "meu-projeto",
  "kind": "summary",
  "path": "internal/auth/service.go",
  "tags": ["auth", "arquitetura"],
  "content": "Decisao: refresh token com rotacao e expiracao de 7 dias.",
  "importance": 5
}
```

### context_search
```json
{
  "project": "meu-projeto",
  "query": "fluxo de autenticacao",
  "k": 8,
  "path_prefix": "internal/auth/",
  "tags": ["auth"],
  "kind": "any"
}
```

### project_brief
```json
{
  "project": "meu-projeto",
  "max_items": 20
}
```

## 10) Boas praticas para memoria realmente util
- Salve menos, mas salve melhor.
- Prefira `summary` para decisoes fechadas.
- Evite texto vago; escreva "decisao + motivo + impacto".
- Padronize tags por dominio (`auth`, `billing`, `infra`, `db`).
- Sempre que mudar decisao antiga, grave novo summary explicando a mudanca.

## 11) Troubleshooting (problemas comuns)
### Cursor fica em "loading tools"
Checklist:
1. `docker compose ps` e confirmar `luck-mpc-server`, `luck-mpc-postgres`, `luck-mpc-ollama` como `Up`
2. conferir config MCP usando `docker exec ... /mcp-server`
3. rodar `docker compose build mcp` apos mudancas de codigo
4. Reload Window no Cursor

### "model not found" no Ollama
Rode:
```bash
docker compose exec ollama ollama pull nomic-embed-text
```

### Erro de banco/schema
Rode:
```bash
make migrate
```

### Base antiga com duplicados
A migration `0003` limpa duplicados por `(project, content_hash)` mantendo o registro mais recente, depois recria o indice unico.

## 12) Comandos de referencia rapida
Subir ambiente:
```bash
docker compose up -d postgres ollama mcp
```

Aplicar migrations:
```bash
make migrate
```

Indexar projeto (incremental):
```bash
make index PROJECT=meu-projeto ROOT=/caminho/absoluto/do/repo
```

Reindexar completo:
```bash
make index-full PROJECT=meu-projeto ROOT=/caminho/absoluto/do/repo
```

Baixar modelo:
```bash
docker compose exec ollama ollama pull nomic-embed-text
```

Ver status:
```bash
docker compose ps
```

Ver logs do MCP:
```bash
docker logs --tail=200 luck-mpc-server
```

Parar tudo:
```bash
docker compose down
```

## 13) Estrutura do projeto
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

## 14) Variaveis de ambiente
- `DATABASE_URL` (obrigatorio)
- `OLLAMA_URL` (default `http://ollama:11434`)
- `OLLAMA_EMBED_MODEL` (default `nomic-embed-text`)
- `MCP_PROJECT_DEFAULT` (opcional)
- `LOG_LEVEL` (default `info`)

## 15) Limite e escopo do MVP
Este MCP e memoria explicita: ele salva e busca o que voce pedir.

Nao faz indexacao automatica do repositorio neste MVP.
