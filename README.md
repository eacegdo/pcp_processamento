# PCP Processamento

API Go que recebe a Carga de Planejamento (CSV) e o Programado (JSON do Bubble), enfileira um Job de Aplicação e grava na coleção de Registro PCP no Supabase.

## Pré-requisitos

1. Tabela `public.pcp` — rode [`docs/sql/pcp.sql`](docs/sql/pcp.sql).
2. Tabela `public.pcp_job` — rode [`docs/sql/pcp_job.sql`](docs/sql/pcp_job.sql).
3. Função de batch do Planejado — rode [`docs/sql/aplicar_carga_planejamento.sql`](docs/sql/aplicar_carga_planejamento.sql).
4. Função e índice do Programado — rode [`docs/sql/aplicar_programado.sql`](docs/sql/aplicar_programado.sql).
5. Coluna `tipo` em `pcp_job` — rode [`docs/sql/pcp_job_tipo.sql`](docs/sql/pcp_job_tipo.sql) se a tabela já existia.
6. CNPJ opcional no Planejado — rode [`docs/sql/pcp_planejado_cnpj_opcional.sql`](docs/sql/pcp_planejado_cnpj_opcional.sql) e de novo [`docs/sql/aplicar_carga_planejamento.sql`](docs/sql/aplicar_carga_planejamento.sql) se o índice antigo já existia.
7. Origem version-test/live no Programado — rode [`docs/sql/pcp_programado_origem.sql`](docs/sql/pcp_programado_origem.sql) e de novo [`docs/sql/aplicar_programado.sql`](docs/sql/aplicar_programado.sql) se a tabela já existia.
8. Arquivo `.env` local (copie de `.env.example`):

```bash
cp .env.example .env
# preencha SUPABASE_URL, SUPABASE_SERVICE_ROLE_KEY, API_KEY
```

## Subir o serviço

```bash
go run ./cmd/pcp-processamento
```

Sobe HTTP + worker no mesmo processo (padrão `:8080`).

Contrato HTTP para o Bubble: [`docs/api.md`](docs/api.md). Curls de teste no topo desse arquivo.

## Enviar uma Carga de Planejamento

Modelo: [`docs/exemplos/carga_planejamento.csv`](docs/exemplos/carga_planejamento.csv). Copie e ajuste.

```bash
curl -X POST http://localhost:8080/v1/planejamento \
  -H "X-API-Key: $API_KEY" \
  -F "file=@docs/exemplos/carga_planejamento.csv"
```

Resposta: `{"id":"<uuid>","tipo":"planejado"}` do Job de Aplicação. Acompanhe o progresso em `pcp_job`. O Planejado fica em `pcp` (`tipo = planejado`).

CSV (`,` ou `;`): `data,fase,regional,fornecedor,cnpj,quantidade`. Data em `DD/MM/AAAA`. Quantidade inteira. `cnpj` é opcional (como veio, máscara inclusa). Sem CNPJ, a chave usa o nome em `fornecedor`. Regional é a sigla (`NO`, `NE-I`, `NE-II`, `SUSE`, `COSE`); o serviço preenche `regional_nome`.

## Enviar Programado (JSON do Bubble)

Modelo: [`docs/exemplos/programado.json`](docs/exemplos/programado.json). Array de objetos, ou `{"itens":[...]}`. Cerca de 3.000 itens cabem numa requisição (limite 16 MB). A API enfileira um Job e responde na hora; o worker grava em batches.

```bash
curl -X POST http://localhost:8080/v1/programado \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d @docs/exemplos/programado.json
```

Campos: `data` (`DD/MM/AAAA` ou `YYYY-MM-DD`), `fase`, `regional` (sigla ou nome: `NO`/`Norte`, `NE-I`/`Nordeste I`, `NE-II`/`Nordeste II`, `SUSE`/`Sudeste/Centro-Sul`, `COSE`/`Centro-Oeste/Minas`), `uf`, `inep` (texto ou número), `fornecedor_nome`, `fornecedor_cnpj`, `quantidade` (default 1), `provisoria`. Identidade: **data + INEP**. Última ocorrência vence. Objeto sem INEP, data, fase ou regional é ignorado.

O mês do espelho é o da data do primeiro item válido. Depois de gravar, some o Programado daquele mês que não veio. Planejado e outros meses não se mexem. Rode de novo [`docs/sql/aplicar_programado.sql`](docs/sql/aplicar_programado.sql) se a RPC antiga (só upsert) já estiver no banco.

## Puxar Programado do Bubble e gravar no PCP

Com `-env test` (padrão) ou `-env live`. A coluna `origem` em `pcp` fica `version-test` ou `live`. Live usa `BUBBLE_API_TOKEN_LIVE` se existir; senão o mesmo token.

```bash
go run ./cmd/puxar-programado -mes 2026-08 -env test
go run ./cmd/puxar-programado -mes 2026-08 -env live
```

Gera `programado.json` e grava no Supabase. Só o arquivo, sem banco: `-somente-json`.

Com o serviço no ar (`BUBBLE_API_TOKEN` preenchido):

```bash
curl -X POST "http://localhost:8080/v1/programado/puxar?mes=2026-08" \
  -H "X-API-Key: $API_KEY"
```

Resposta: `{"id":"<uuid>","tipo":"programado","itens":N,"skips":N}`. O worker aplica em seguida.

## Testes automatizados

```bash
go test ./...
```

Usam stores em memória e um PostgREST falso — não precisam do Supabase real.

## Colunas tocadas no Supabase

- `pcp_job`: `status`, `tipo` (`planejado` ou `programado`), `total`, `processadas`, `file_name`, `error_message` (e `id` gerado pelo banco)
- `pcp`: Planejado via RPC `aplicar_carga_planejamento`; Programado via RPC `aplicar_programado` (espelho do mês: grava a carga e remove omitidos daquele mês)
