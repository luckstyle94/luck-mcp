# luck-mcp: codebase memory multi-repo para agentes de IA via MCP

## PROJETO 100% CRIADO UTILIZANDO IA (CODEX 5.3)

## Versoes de documentacao
- Portugues (guia completo): [README.md](./README.md)
- Portugues (quickstart): [QUICKSTART.md](./QUICKSTART.md)
- English (full guide): [README.en.md](./README.en.md)
- English (quickstart): [QUICKSTART.en.md](./QUICKSTART.en.md)

## Cheatsheet Diario (copiar e usar)
Sempre rode comandos no terminal, dentro da pasta do MCP:

```bash
cd /home/$USER/path/to/luck-mcp
```

1. Quando abrir o dia/projeto:
```bash
make up
make migrate
make index PROJECT=meu-projeto ROOT=/caminho/absoluto/do/projeto
```

2. Durante o trabalho (no chat da IA, nao no terminal):
- No Codex CLI, no comeco da sessao, diga: `use codebase memory for this session`
- Inicio de sessao: peça `project_brief` para `meu-projeto`
- Se quiser catalogar descricao e tags do repo: use `repo_register`
- Para saber em quais repos algo aparece: use `search_across_repos`
- Para achar arquivos e modulos: use `repo_find_files`
- Para achar README, ADR e docs: use `repo_find_docs`
- Para busca por tema ou logica parecida: use `repo_search`
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

## Uso automatico no Codex
O MCP fica disponivel para qualquer client MCP configurado, mas o uso mais simples hoje foi preparado para o Codex CLI.

Como isso funciona:
- existe uma skill local do Codex para usar o `luck-mcp` automaticamente em tarefas de codebase
- essa skill ensina o Codex a procurar docs, localizar arquivos, buscar memoria salva e comparar repositorios relacionados
- na pratica, voce nao precisa decorar o nome de todas as tools

Convencao recomendada no inicio da sessao do Codex:
```text
use codebase memory for this session
```

Essa frase ajuda a deixar o comportamento previsivel, mesmo quando a skill ja estiver instalada.

### Como instalar a skill do Codex
Se a skill ainda nao estiver instalada, crie a pasta de skills do Codex e faça um link simbolico:

```bash
mkdir -p ~/.codex/skills
ln -s /home/$USER/path/to/luck-mcp/skills/codebase-memory-mcp ~/.codex/skills/codebase-memory-mcp
```

Se preferir copiar os arquivos em vez de criar link:

```bash
mkdir -p ~/.codex/skills/codebase-memory-mcp
cp -R /home/$USER/path/to/luck-mcp/skills/codebase-memory-mcp/. ~/.codex/skills/codebase-memory-mcp/
```

### O que a skill faz por voce
- procura README, ADR e docs primeiro
- encontra arquivos e modulos importantes
- usa busca cross-repo quando o problema pode afetar mais de um repositorio
- consulta memoria salva antes de mexer em area sensivel
- salva decisoes importantes para reutilizar depois

## Organizacao sugerida dos repositorios
Um layout comum e:

```text
/home/$USER/repos
```

Organizacao sugerida:
- `iac/`: repositorios Terraform; sao os mais importantes para este MCP
- `lambda/`: repositorios Lambda; geralmente Python, mas nao sempre
- `private/`: repositorios pessoais/privados
- outros repos direto em `/home/$USER/repos`: ainda podem ser relevantes e nao devem ser ignorados

Comportamento esperado:
- em repos `iac/`, o MCP deve ser usado cedo e com busca cross-repo com mais frequencia
- em repos `lambda/`, o MCP deve considerar relacao com infraestrutura criada por Terraform
- em tarefas de validacao/review Terraform no Codex, faz sentido usar tambem a skill `vex-tf`

Repos Terraform mais novos que devem servir como referencia de padrao quando relevantes:
- `iac-intelliscan`
- `iac-mkt-diagnostico-maturidade`
- `iac-core-boundary`
- `iac-core-vault`
- `iac-mcp`

Preferencia importante:
- quando um modulo Terraform reutilizavel for indicado, prefira referencia via git source
- evite recomendar referencia por path local como padrao

## Aliases uteis (atalhos no terminal)
Para facilitar uso diario, voce pode criar aliases:

```bash
alias mcp-up='cd /home/$USER/path/to/luck-mcp && make up'
alias mcp-down='cd /home/$USER/path/to/luck-mcp && make down'
alias mcp-index='project_root="$PWD"; project_name="$(basename "$project_root")"; (cd /home/$USER/path/to/luck-mcp && make index PROJECT="$project_name" ROOT="$project_root")'
alias mcp-index-full='project_root="$PWD"; project_name="$(basename "$project_root")"; (cd /home/$USER/path/to/luck-mcp && make index-full PROJECT="$project_name" ROOT="$project_root")'
```

Para salvar definitivamente no bash:

```bash
cat <<'EOF' >> ~/.bashrc
alias mcp-up='cd /home/$USER/path/to/luck-mcp && make up'
alias mcp-down='cd /home/$USER/path/to/luck-mcp && make down'
alias mcp-index='project_root="$PWD"; project_name="$(basename "$project_root")"; (cd /home/$USER/path/to/luck-mcp && make index PROJECT="$project_name" ROOT="$project_root")'
alias mcp-index-full='project_root="$PWD"; project_name="$(basename "$project_root")"; (cd /home/$USER/path/to/luck-mcp && make index-full PROJECT="$project_name" ROOT="$project_root")'
EOF
source ~/.bashrc
```

Depois disso, no terminal:
- `mcp-up` para subir os servicos
- `mcp-down` para parar os servicos
- `mcp-index` para indexar o projeto da pasta atual (`$PWD`)
- `mcp-index-full` para reindexar completo o projeto da pasta atual

Observacao:
- rode `mcp-index` e `mcp-index-full` estando dentro da pasta do projeto que deseja indexar.

## 1) O que este projeto faz (explicacao simples)
Este projeto cria um servidor MCP local para guardar e recuperar contexto de trabalho.

Na pratica, isso permite que seu agent (Cursor, Codex CLI, Claude Code, VSCode com suporte MCP) tenha uma "memoria" persistente entre sessoes e uma camada de pesquisa de codebase multi-repo.

Voce salva:
- decisoes de arquitetura
- gotchas
- resumos de tarefas
- contexto util de codigo

E depois busca:
- por arquivos e docs
- por significado (busca semantica)
- por impacto entre repos
- por padroes relacionados em multiplos repositorios

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
cd /home/$USER/path/to/luck-mcp
```

Regras praticas:
- `make up`, `make down`, `make migrate`, `make index`, `make index-full`: execute na pasta `luck-mcp`.
- `ROOT` do `make index`: e o caminho absoluto do projeto que voce quer indexar (pode ser Go, Python, Terraform, React etc.).
- As tools MCP (`repo_list`, `repo_register`, `search_across_repos`, `repo_search`, `repo_find_files`, `repo_find_docs`, `context_add`, `context_search`, `project_brief`) voce usa no chat do agent (Cursor/Codex/Claude), nao no terminal.
- Nao precisa entrar em container manualmente para uso normal.

### Exemplo real: estou em `/home/$USER/path/to/meu-projeto1`
Se voce esta trabalhando nesse projeto, use sempre o mesmo nome de projeto no MCP, por exemplo: `meu-projeto1`.

No terminal (dentro do repo `luck-mcp`), rode:
```bash
cd /home/$USER/path/to/luck-mcp
make up
make index PROJECT=meu-projeto1 ROOT=/home/$USER/path/to/meu-projeto1
```

Depois, no chat da IA (Cursor/Codex/Claude), use as tools com esse projeto:
```text
Use repo_register com name="meu-projeto1", root_path="/home/$USER/path/to/meu-projeto1", description="Descricao curta do repo", tags=["backend","auth"].
```

```text
Use search_across_repos com query="auth" e k=5 para ver em quais repos isso aparece.
```

```text
Use project_brief no projeto "meu-projeto1" com max_items=20.
```

```text
Use repo_find_files com repos=["meu-projeto1"] query="auth" e k=10.
```

```text
Use repo_find_docs com repos=["meu-projeto1"] query="autenticacao" e k=5.
```

```text
Use repo_search com repos=["meu-projeto1"] query="fluxo de autenticacao" mode="hybrid" e k=8.
```

```text
Use context_add no projeto "meu-projeto1" com kind="summary", importance=5, content="Decisao: ...".
```

Resumo importante:
- Voce pode estar codando em `/home/$USER/path/to/meu-projeto1`.
- Mas os comandos `make ...` sempre sao executados na pasta do MCP (`luck-mcp`).
- As tools sao chamadas no chat do agent e precisam do campo `project` consistente.

## 4) Setup inicial (primeira vez)
Rode exatamente nesta ordem:

```bash
cd /home/$USER/path/to/luck-mcp

docker compose build mcp
docker compose up -d postgres ollama mcp
make migrate
docker compose exec ollama ollama pull nomic-embed-text
make index PROJECT=meu-projeto ROOT=/caminho/absoluto/do/repo
```

O que cada comando faz:
1. `build mcp`: gera a imagem local do servidor MCP.
2. `up -d postgres ollama mcp`: sobe banco, embeddings e container base do MCP.
3. `make migrate`: aplica schema no banco (`0001` ate `0006`).
4. `ollama pull`: baixa o modelo de embedding.
5. `make index`: faz a primeira indexacao automatica do projeto.

## 5) Rotina diaria (uso normal)
### Iniciar ambiente no comeco do dia
```bash
cd /home/$USER/path/to/luck-mcp
docker compose up -d postgres ollama mcp
make index PROJECT=meu-projeto ROOT=/caminho/absoluto/do/repo
```

### Parar ambiente no fim do dia
```bash
cd /home/$USER/path/to/luck-mcp
docker compose down
```

### Quando rodar `make migrate`?
Rode quando:
- for primeira subida do ambiente
- entrar migration nova no repositorio
- quiser garantir schema alinhado

Comando:
```bash
cd /home/$USER/path/to/luck-mcp
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
3. Verificar se tools aparecem: `repo_list`, `repo_register`, `search_across_repos`, `repo_search`, `repo_find_files`, `repo_find_docs`, `context_add`, `context_search`, `project_brief`

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
As tools disponiveis agora sao:
- `repo_list`
- `repo_register`
- `search_across_repos`
- `repo_search`
- `repo_find_files`
- `repo_find_docs`
- `context_add`
- `context_search`
- `project_brief`

### 8.0 Fluxo diario simples (para leigos)
1. No terminal, dentro de `luck-mcp`, rode:
```bash
make up
make migrate
make index PROJECT=meu-projeto ROOT=/caminho/absoluto/do/projeto
```
2. No chat da IA, diga:

```text
use codebase memory for this session
```

3. Em seguida, peça um resumo do projeto:

```text
Use project_brief no projeto "meu-projeto" com max_items=20 e me mostre um resumo simples.
```

4. Se precisar achar algo:
- `repo_find_docs` para documentacao
- `repo_find_files` para arquivos
- `search_across_repos` para descobrir outros repos relacionados
- `repo_search` para buscar logica parecida

5. Antes de mexer em area sensivel:
- use `context_search`

6. Quando decidir algo importante:
- use `context_add` com `kind="summary"`

7. No fim do dia (opcional):
```bash
make down
```

### 8.0.1 Fluxo recomendado no Codex CLI
1. Entrar no repo em que vai trabalhar
2. Rodar `mcp-index` se o repo mudou bastante
3. Comecar a sessao com:
```text
use codebase memory for this session
```
4. Deixar o Codex usar automaticamente:
- `repo_find_docs`
- `repo_find_files`
- `search_across_repos`
- `project_brief`
5. Salvar decisoes importantes com `context_add`

### 8.1 Fluxo recomendado de uso
1. Inicio de sessao:
- usar `search_across_repos` para identificar repos impactados ou relacionados
- usar `repo_find_docs` para achar docs base
- usar `repo_find_files` para achar arquivos/modulos
- usar `repo_search` para buscar implementacao ou tema parecido
- pedir `project_brief` para carregar contexto manual principal

2. Antes de mexer em area critica:
- usar `context_search` com query objetiva

3. Depois de decidir algo importante:
- usar `context_add` com `kind=summary` ou `kind=memory`

4. Fim de tarefa grande:
- salvar resumo final com `importance` alta

### 8.2 Como usar o codebase memory na pratica
Pense assim:
- `project_brief`: "me relembre o que importa"
- `repo_find_docs`: "me mostre a documentacao certa"
- `repo_find_files`: "me mostre onde isso fica"
- `search_across_repos`: "isso existe em outros repositorios?"
- `repo_search`: "me ache algo parecido"
- `context_search`: "ja decidimos algo sobre isso antes?"
- `context_add`: "guarde esta decisao para depois"

### 8.3 Prompts prontos (copiar e colar)
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

### 8.4 Formato de dados recomendado
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
