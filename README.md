# PCP Processamento

API Go que recebe um CSV de Carga de Planejamento, enfileira um Job de Aplicação e grava o Planejado na coleção de Registro PCP no Supabase.

## Pré-requisitos

1. Tabela `public.pcp` — rode [`docs/sql/pcp.sql`](docs/sql/pcp.sql).
2. Tabela `public.pcp_job` — rode [`docs/sql/pcp_job.sql`](docs/sql/pcp_job.sql).
3. Função de batch — rode [`docs/sql/aplicar_carga_planejamento.sql`](docs/sql/aplicar_carga_planejamento.sql).
4. Arquivo `.env` local (copie de `.env.example`):

```bash
cp .env.example .env
# preencha SUPABASE_URL, SUPABASE_SERVICE_ROLE_KEY, API_KEY
```

## Subir o serviço

```bash
go run ./cmd/pcp-processamento
```

Sobe HTTP + worker no mesmo processo (padrão `:8080`).

## Enviar uma Carga de Planejamento

```bash
curl -X POST http://localhost:8080/v1/cargas \
  -H "X-API-Key: $API_KEY" \
  -F "file=@carga.csv"
```

Resposta: `{"id":"<uuid>"}` do Job de Aplicação. Acompanhe o progresso em `pcp_job`. O Planejado fica em `pcp` (`tipo = planejado`).

CSV esperado (`,` ou `;`):

```csv
data,fase,regional,fornecedor,cnpj,quantidade
18/08/2026,4.2,NE-I,NUH DIGITAL,12.345.678/0001-99,10
```

Data em `DD/MM/AAAA`. Quantidade inteira. CNPJ como veio. `fornecedor` (nome) é opcional. Regional é a sigla (`NO`, `NE-I`, `NE-II`, `SUSE`, `COSE`); o serviço preenche `regional_nome`.

## Testes automatizados

```bash
go test ./...
```

Usam stores em memória e um PostgREST falso — não precisam do Supabase real.

## Colunas tocadas no Supabase

- `pcp_job`: `status`, `total`, `processadas`, `file_name`, `error_message` (e `id` gerado pelo banco)
- `pcp`: Planejado via RPC `aplicar_carga_planejamento` (upsert pela chave data + fase + regional + CNPJ; chave nova com quantidade 0 não insere)
