# PCP Processamento

Serviço em Go que grava o **Planejado** e o **Programado** na tabela `pcp` do Supabase.

- **Planejado:** meta de escolas (CSV). Não aponta para uma escola específica.
- **Programado:** uma escola (INEP) associada a uma OSP válida. Pode ser enviado em JSON pronto, ou **puxado** da Data API do Bubble.

Cada envio cria um **Job** em `pcp_job`. A API responde na hora; um worker no mesmo processo aplica a carga no banco.

Contrato HTTP detalhado (status, erros, campos): [`docs/api.md`](docs/api.md).

---

## O que você precisa

- Go (versão do `go.mod`)
- Um projeto Supabase
- Arquivo `.env` na raiz (veja abaixo)
- Token da Data API do Bubble, se for **puxar** Programado

---

## 1. Configurar o `.env`

```bash
cp .env.example .env
```

| Variável | Obrigatória? | Para quê |
| --- | --- | --- |
| `SUPABASE_URL` | sim (API e CLI que grava no banco) | URL do projeto, ex. `https://xxxx.supabase.co` |
| `SUPABASE_SERVICE_ROLE_KEY` | sim (idem) | chave **service_role** (não a anon) |
| `API_KEY` | sim para a API HTTP | valor do header `X-API-Key` |
| `BUBBLE_API_TOKEN` | para puxar `-env test` | token da Data API na **version-test** |
| `BUBBLE_API_TOKEN_LIVE` | opcional | token da Data API na **live**. Se vazio, `-env live` usa `BUBBLE_API_TOKEN` |
| `HTTP_ADDR` | não | endereço da API (padrão `:8080`) |
| `BATCH_SIZE` | não | tamanho do lote do Planejado (padrão `200`) |
| `BATCH_MAX_RETRIES` | não | tentativas em falha transitória (padrão `3`) |

A chave `API_KEY` é inventada por você. Quem chama a API precisa enviar o mesmo valor.

---

## 2. Preparar o banco (SQL Editor do Supabase)

**Banco novo, ou pode apagar `pcp` e `pcp_job`:** rode [`docs/sql/recriar_pcp.sql`](docs/sql/recriar_pcp.sql) uma vez. Cria as tabelas, índices e as funções `aplicar_carga_planejamento` e `aplicar_programado`.

**Banco que já tem dados** (não rode o `recriar`): execute, nesta ordem, o que ainda faltar:

1. [`docs/sql/pcp.sql`](docs/sql/pcp.sql) e [`docs/sql/pcp_job.sql`](docs/sql/pcp_job.sql)
2. [`docs/sql/aplicar_carga_planejamento.sql`](docs/sql/aplicar_carga_planejamento.sql)
3. [`docs/sql/aplicar_programado.sql`](docs/sql/aplicar_programado.sql)
4. Se a tabela `pcp_job` já existia sem `tipo`: [`docs/sql/pcp_job_tipo.sql`](docs/sql/pcp_job_tipo.sql)
5. Se o Planejado ainda exige CNPJ: [`docs/sql/pcp_planejado_cnpj_opcional.sql`](docs/sql/pcp_planejado_cnpj_opcional.sql) e de novo o passo 2
6. Se `pcp` ainda não tem `origem`: [`docs/sql/pcp_programado_origem.sql`](docs/sql/pcp_programado_origem.sql) e de novo o passo 3
7. Esconder version-test: [`docs/sql/pcp_rls_esconder_version_test.sql`](docs/sql/pcp_rls_esconder_version_test.sql)

Sem a RPC `aplicar_programado` atualizada, o puxar grava o Job mas a coluna `origem` não entra no banco.

---

## 3. Subir a API

Na raiz do repositório:

```bash
go run ./cmd/pcp-processamento
```

Sobe HTTP + worker no mesmo processo, em `http://localhost:8080`.

Teste se está no ar (não pede chave):

```bash
curl -sS http://localhost:8080/
```

Esperado: `{"status":"ok"}`.

Carregue as variáveis do `.env` no terminal antes dos curls:

```bash
set -a && source .env && set +a
```

Se `BUBBLE_API_TOKEN` (e, se quiser live, `BUBBLE_API_TOKEN_LIVE`) estiver preenchido, a API aceita `POST /v1/programado/puxar` com body `{"mes":"2026-08","env":"test"}` ou `"env":"live"`.

---

## 4. Enviar a Carga de Planejamento (CSV)

Modelo: [`docs/exemplos/carga_planejamento.csv`](docs/exemplos/carga_planejamento.csv).

```bash
curl -X POST http://localhost:8080/v1/planejamento \
  -H "X-API-Key: $API_KEY" \
  -F "file=@docs/exemplos/carga_planejamento.csv"
```

Resposta `201`: `{"id":"<uuid>","tipo":"planejado"}`. Acompanhe o Job em `pcp_job` (seção 7).

Colunas do CSV (`,` ou `;`): `data`, `fase`, `regional`, `fornecedor`, `cnpj`, `quantidade`.

| Campo | Obrigatório | Formato |
| --- | --- | --- |
| `data` | sim | `DD/MM/AAAA` |
| `fase` | sim | texto |
| `regional` | sim | sigla: `NO`, `NE-I`, `NE-II`, `SUSE`, `COSE` |
| `cnpj` | não | como veio (máscara ok) |
| `fornecedor` | se não houver CNPJ | nome; identifica a linha |
| `quantidade` | sim | inteiro ≥ 0 |

O serviço preenche `regional_nome` (`NO` → Norte, `NE-I` → Nordeste I, etc.).

**Chave do Planejado:** data + fase + regional + CNPJ. Sem CNPJ, usa o nome do fornecedor. Última linha com a mesma chave vence.

- Reenviar a mesma chave **atualiza** (10 vira 9; 10 vira 0).
- Chave **nova** com quantidade `0` **não** grava.
- Chave que **não veio** neste CSV **permanece** (não é espelho: não apaga o que faltou).
- Sem nenhuma linha válida: `400`, Job não é criado.
- No Planejado, `inep`, `uf`, `provisoria` e `origem` ficam vazios.

---

## 5. Enviar Programado já montado (JSON)

Use quando o Bubble (ou você) já tem o array. Modelo: [`docs/exemplos/programado.json`](docs/exemplos/programado.json).

Array de objetos, ou `{"itens":[...]}`. Limite 16 MB (~3.000 itens).

```bash
curl -X POST http://localhost:8080/v1/programado \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d @docs/exemplos/programado.json
```

| Campo | Obrigatório | Formato |
| --- | --- | --- |
| `data` | sim | `DD/MM/AAAA` ou `YYYY-MM-DD` |
| `fase` | sim | texto |
| `regional` | sim | sigla ou nome (`NO` / `Norte`, `NE-I` / `Nordeste I`, `NE-II` / `Nordeste II`, `SUSE` / `Sudeste/Centro-Sul`, `COSE` / `Centro-Oeste/Minas`) |
| `inep` | sim | texto ou número |
| `uf` | não | texto |
| `fornecedor_nome` | não | texto |
| `fornecedor_cnpj` | não | como veio |
| `quantidade` | não | inteiro ≥ 0; omitido = `1` |
| `provisoria` | não | `true` / `false` |
| `origem` | não | `version-test` ou `live` (o puxar preenche sozinho) |

Nome vira sigla na gravação (`Norte` → `regional = NO`).

**Chave do Programado:** data + INEP. Última ocorrência no JSON vence.

**Espelho do mês:** o mês é o da data do **primeiro item válido**. Depois de gravar, some o Programado **daquele mês** cuja chave não veio. Planejado e Programado de outros meses não se mexem. Itens de outro mês no mesmo JSON são ignorados.

JSON vazio ou sem itens válidos: `400` e **não apaga** o mês.

---

## 6. Puxar Programado do Bubble (o caminho usual)

O comando lê a Data API, monta o JSON no formato da seção 5, grava `programado.json` na pasta atual e **aplica no Supabase** (mesmo Job / RPC do POST).

### Escolher test ou live

| Flag | URL da Data API | `origem` gravada em `pcp` | Token |
| --- | --- | --- | --- |
| `-env test` (padrão) | `https://eace.org.br/version-test/api/1.1` | `version-test` | `BUBBLE_API_TOKEN` |
| `-env live` | `https://eace.org.br/api/1.1` | `live` | `BUBBLE_API_TOKEN_LIVE`, ou `BUBBLE_API_TOKEN` se o live estiver vazio |

A chave do Programado continua **data + INEP** (não duplica a mesma escola no mês por ser test vs live). A coluna `origem` só marca de onde veio.

```bash
# version-test, mês de agosto de 2026
go run ./cmd/puxar-programado -mes 2026-08 -env test

# app live
go run ./cmd/puxar-programado -mes 2026-08 -env live
```

Sem `-mes`, usa o mês atual em `America/Sao_Paulo`.

### Outras flags

| Flag | Padrão | Efeito |
| --- | --- | --- |
| `-mes YYYY-MM` | mês atual | recorte pela **Data do Programado** (data de conexão quando a Escola está `Conectada`, senão previsão de entrega) |
| `-env test\|live` | `test` | qual app Bubble |
| `-o arquivo.json` | `programado.json` | onde salvar o JSON |
| `-somente-json` | desligado | só gera o arquivo; **não** cria Job nem grava `pcp` |

Exemplo só para inspecionar o que seria gravado:

```bash
go run ./cmd/puxar-programado -mes 2026-08 -env test -somente-json -o /tmp/agosto.json
```

Se não houver nenhuma folha válida, o comando **não** cria Job e **não** apaga o mês.

### O que entra (e o que é skip)

Busca por dois caminhos e une o resultado sem duplicar OSP:

- **por previsão:** OSPs com previsão de entrega no mês, status ≠ `Reprovado`
- **por conexão:** `importação_escola` com `data_relatorio` no mês → INEPs → Folhas de Registro desses INEPs → as OSPs dessas folhas

Em cada Folha de Registro:

1. Contrato de instalação com descrição contendo `kit` e tipo de obra `4-IMPLANTAÇÃO_DE_REDE_INTERNA`
2. Escola com fase e regional
3. Quantidade **1 por folha**
4. **Data do Programado:** se a escola está `Conectada` e `importação_escola.data_relatorio` está preenchida, usa essa data; senão usa a previsão de entrega da OSP
5. **Provisória:** `true` se `OSnum` da OSP está vazio ou `0`
6. Fase, regional, UF, INEP e fornecedor RI vêm da **escola**
7. **Filtro final:** item cuja Data do Programado cai fora do mês pedido vira skip `data do Programado fora do mês` — vale para os dois caminhos

Por isso puxar um mês pode **remover** dele Registros que estavam lá com data de outro mês (o Espelho do Mês substitui o mês civil). Vale puxar em seguida o mês da conexão para reancorar esses Registros. O log e a resposta de `POST /v1/programado/puxar` trazem o resumo por origem (previsão, conexão, após dedupe, itens fora do mês).

MIP não entra. Folhas que não passam nas regras aparecem no log como `skip` (sem INEP, sem kit RI, escola sem fase, etc.) e não vão para o JSON.

Se a live ainda não expuser `/obj/osp`, `-env live` falha com o erro HTTP do Bubble — isso é configuração da Data API no app, não do CLI.

### Puxar pela API HTTP

Com `go run ./cmd/pcp-processamento` no ar e os tokens no `.env`. O body escolhe o mês e o ambiente (padrão `env` = `test`; sem `mes` usa o mês atual):

```bash
curl -X POST http://localhost:8080/v1/programado/puxar \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"mes":"2026-08","env":"test"}'

curl -X POST http://localhost:8080/v1/programado/puxar \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"mes":"2026-08","env":"live"}'
```

Resposta `201`: `{"id":"<uuid>","tipo":"programado","itens":N,"skips":N,"origem":"version-test"}`. O worker aplica em seguida. Live usa `BUBBLE_API_TOKEN_LIVE` se existir; senão o mesmo token da test.

---

## 7. Saber se deu certo

Não há GET de progresso nesta API. Olhe a tabela `pcp_job` no Supabase (Table Editor ou API Connector do Bubble), pelo `id` devolvido no `201`.

| `status` | Significado |
| --- | --- |
| `queued` | na fila |
| `running` | o worker está gravando |
| `success` | carga aplicada |
| `failed` | motivo em `error_message` |

Um Job por vez, na ordem de chegada. Dá para enfileirar outro envio enquanto um corre.

Confira o resultado em `pcp`:

- Planejado: `tipo = planejado`
- Programado: `tipo = programado` (e `origem` se veio do puxar)

Se o Programado falhar no meio, o mês pode ficar inconsistente — rode de novo o puxar (ou o POST do JSON) daquele mês.

---

## 8. Regionais

| Sigla | Nome |
| --- | --- |
| `NO` | Norte |
| `NE-I` | Nordeste I |
| `NE-II` | Nordeste II |
| `SUSE` | Sudeste/Centro-Sul |
| `COSE` | Centro-Oeste/Minas |

No Planejado a regional no CSV é a **sigla**. No Programado aceita sigla ou nome.

---

## 9. Esconder Programado da version-test

Linhas com `origem = version-test` não devem aparecer no Bubble, no Table Editor (chave **anon** / **authenticated**) nem em funções SQL que leem dados.

Rode [`docs/sql/pcp_rls_esconder_version_test.sql`](docs/sql/pcp_rls_esconder_version_test.sql). Isso liga RLS na tabela `pcp` e cria a view **`pcp_visivel`** (sem version-test).

No Bubble e em qualquer função que **devolva** registros, aponte para `pcp_visivel`, não para `pcp`. O filtro da view vale inclusive para `service_role` e para o SQL Editor.

A API Go / CLI (`-env test`) continua gravando em `pcp` com a chave **service_role**. As RPCs `aplicar_carga_planejamento` e `aplicar_programado` não devolvem linha; elas precisam enxergar a tabela inteira por causa da chave única (data + INEP).

Não use a `service_role` no API Connector do Bubble para **ler** `pcp`: essa chave ignora RLS e mostraria version-test.

---

## 10. Testes automatizados

Não precisam do Supabase nem do Bubble reais:

```bash
go test ./...
```

---

## Onde está cada coisa

| Assunto | Arquivo |
| --- | --- |
| Contrato HTTP, curls e erros | [`docs/api.md`](docs/api.md) |
| CSV de exemplo | [`docs/exemplos/carga_planejamento.csv`](docs/exemplos/carga_planejamento.csv) |
| JSON de exemplo | [`docs/exemplos/programado.json`](docs/exemplos/programado.json) |
| Recriar banco do zero | [`docs/sql/recriar_pcp.sql`](docs/sql/recriar_pcp.sql) |
| Coluna `origem` (banco antigo) | [`docs/sql/pcp_programado_origem.sql`](docs/sql/pcp_programado_origem.sql) |
| Esconder version-test (RLS + view) | [`docs/sql/pcp_rls_esconder_version_test.sql`](docs/sql/pcp_rls_esconder_version_test.sql) |
| Variáveis de ambiente | [`.env.example`](.env.example) |
