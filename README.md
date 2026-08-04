# OCE Processamento

API Go que recebe um CSV de Lote OCE, enfileira um Job de Aplicação e atualiza a Situação OCE das Escolas no Supabase.

## Pré-requisitos

1. Tabela `public.oce_job` no Supabase — rode o SQL em [`docs/sql/oce_job.sql`](docs/sql/oce_job.sql) (SQL Editor).
2. Função de batch — rode também [`docs/sql/aplicar_situacao_oce_lote.sql`](docs/sql/aplicar_situacao_oce_lote.sql).
3. Tabela `public.escola` já existente, com `inep` e as colunas `oce_tipo_acesso`, `oce_status`, `oce_pendencia`.
4. Arquivo `.env` local (copie de `.env.example`):

```bash
cp .env.example .env
# preencha SUPABASE_URL, SUPABASE_SERVICE_ROLE_KEY, API_KEY
```

## Subir o serviço

```bash
go run ./cmd/oce-processamento
```

Sobe HTTP + worker no mesmo processo (padrão `:8080`).

## Enviar um Lote OCE

```bash
curl -X POST http://localhost:8080/v1/lotes \
  -H "X-API-Key: $API_KEY" \
  -F "file=@lote.csv"
```

Resposta: `{"id":"<uuid>"}` do Job de Aplicação. Acompanhe o progresso na tabela `oce_job` (Bubble / Supabase). A Situação OCE aplicada fica em `escola` (só as três colunas OCE, por INEP).

CSV esperado (`,` ou `;`):

```csv
inep,oce_tipo_acesso,oce_status_final,oce_pendencia
12345678,presencial,ativo,nenhuma
```

## Testes automatizados

```bash
go test ./...
```

Usam stores em memória e um PostgREST falso (`httptest`) — não precisam do Supabase real.

## Colunas tocadas no Supabase

- `oce_job`: `status`, `total`, `processadas`, `file_name`, `error_message` (e `id` gerado pelo banco)
- `escola`: apenas `oce_tipo_acesso`, `oce_status`, `oce_pendencia` via RPC `aplicar_situacao_oce_lote` (UPDATE em lote por INEP; sem insert/upsert)
