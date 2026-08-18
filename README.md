# PCP Processamento

API Go que recebe a Carga de Planejamento (CSV) e o Programado (JSON do Bubble), enfileira um Job de Aplicação e grava na coleção de Registro PCP no Supabase.

## Pré-requisitos

1. Tabela `public.pcp` — rode [`docs/sql/pcp.sql`](docs/sql/pcp.sql).
2. Tabela `public.pcp_job` — rode [`docs/sql/pcp_job.sql`](docs/sql/pcp_job.sql).
3. Função de batch do Planejado — rode [`docs/sql/aplicar_carga_planejamento.sql`](docs/sql/aplicar_carga_planejamento.sql).
4. Função e índice do Programado — rode [`docs/sql/aplicar_programado.sql`](docs/sql/aplicar_programado.sql).
5. Coluna `tipo` em `pcp_job` — rode [`docs/sql/pcp_job_tipo.sql`](docs/sql/pcp_job_tipo.sql) se a tabela já existia.
6. Arquivo `.env` local (copie de `.env.example`):

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

CSV (`,` ou `;`): `data,fase,regional,fornecedor,cnpj,quantidade`. Data em `DD/MM/AAAA`. Quantidade inteira. CNPJ como veio. `fornecedor` (nome) é opcional. Regional é a sigla (`NO`, `NE-I`, `NE-II`, `SUSE`, `COSE`); o serviço preenche `regional_nome`.

## Enviar Programado (JSON do Bubble)

Modelo: [`docs/exemplos/programado.json`](docs/exemplos/programado.json). Array de objetos, ou `{"itens":[...]}`. Cerca de 3.000 itens cabem numa requisição (limite 16 MB). A API enfileira um Job e responde na hora; o worker grava em batches.

```bash
curl -X POST http://localhost:8080/v1/programado \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d @docs/exemplos/programado.json
```

Campos: `data` (`DD/MM/AAAA` ou `YYYY-MM-DD`), `fase`, `regional` (sigla), `uf`, `inep` (texto ou número), `fornecedor_nome`, `fornecedor_cnpj`, `quantidade` (default 1), `provisoria`. Identidade: **data + INEP**. Última ocorrência vence. Objeto sem INEP, data, fase ou regional é ignorado.

O mês do espelho é o da data do primeiro item válido. Depois de gravar, some o Programado daquele mês que não veio. Planejado e outros meses não se mexem. Rode de novo [`docs/sql/aplicar_programado.sql`](docs/sql/aplicar_programado.sql) se a RPC antiga (só upsert) já estiver no banco.

## Testes automatizados

```bash
go test ./...
```

Usam stores em memória e um PostgREST falso — não precisam do Supabase real.

## Colunas tocadas no Supabase

- `pcp_job`: `status`, `tipo` (`planejado` ou `programado`), `total`, `processadas`, `file_name`, `error_message` (e `id` gerado pelo banco)
- `pcp`: Planejado via RPC `aplicar_carga_planejamento`; Programado via RPC `aplicar_programado` (espelho do mês: grava a carga e remove omitidos daquele mês)
