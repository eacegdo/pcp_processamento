# API PCP Processamento

Serviço HTTP que recebe a **Carga de Planejamento** (CSV) e a **Carga de Programado** (JSON). Cada envio cria um **Job de Aplicação** e responde na hora. O worker grava na coleção `pcp` no Supabase. O progresso **não** tem GET nesta API: o Bubble lê `pcp_job` pelo API Connector.

Base (local): `http://localhost:8080`  
Produção: a URL do serviço no EasyPanel.

## Curls rápidos

Na raiz do repo, com a API no ar (`:8080`) e `.env` preenchido:

```bash
set -a && source .env && set +a
```

**No ar?** — espera `200` e `{"status":"ok"}` (sem API key):

```bash
curl -sS http://localhost:8080/
```

**Planejado** (CSV modelo) — espera `201` e `"tipo":"planejado"`:

```bash
curl -sS -X POST http://localhost:8080/v1/planejamento \
  -H "X-API-Key: $API_KEY" \
  -F "file=@docs/exemplos/carga_planejamento.csv"
```

**Programado** (JSON modelo) — espera `201` e `"tipo":"programado"`:

```bash
curl -sS -X POST http://localhost:8080/v1/programado \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d @docs/exemplos/programado.json
```

**Programado** (um objeto inline):

```bash
curl -sS -X POST http://localhost:8080/v1/programado \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d '[{"data":"18/08/2026","fase":"4.2","regional":"NE-I","uf":"BA","inep":"12345678","fornecedor_nome":"NUH","quantidade":1,"provisoria":false}]'
```

**Espelho do mês** — a segunda chamada deixa só o INEP `12345678` em agosto (some o `87654321` do JSON modelo, se você rodou o curl anterior):

```bash
curl -sS -X POST http://localhost:8080/v1/programado \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d '[{"data":"18/08/2026","fase":"4.2","regional":"NE-I","inep":"12345678","quantidade":1}]'
```

**Sem chave** — espera `401`:

```bash
curl -sS -o /dev/null -w "%{http_code}\n" -X POST http://localhost:8080/v1/programado \
  -H "Content-Type: application/json" \
  -d '[]'
```

**JSON vazio** — espera `400`:

```bash
curl -sS -X POST http://localhost:8080/v1/programado \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d '[]'
```

## Autenticação

Todas as rotas exigem o header:

```
X-API-Key: <API_KEY>
```

A chave é a variável `API_KEY` do serviço. Sem ela, ou com valor diferente: `401` e `{"error":"unauthorized"}`. Nenhum Job é criado.

Não há JWT nem usuário final nesta API.

## Health

`GET /` — sem API key. Responde `200` e `{"status":"ok"}` se o processo HTTP está no ar. Não consulta o Supabase. Use no EasyPanel como health check.

## Resposta de sucesso (os dois POSTs)

`201 Created`

```json
{
  "id": "11111111-1111-1111-1111-111111111111",
  "tipo": "planejado"
}
```

| Campo | Valores | Significado |
| --- | --- | --- |
| `id` | UUID | Identidade do Job em `pcp_job` |
| `tipo` | `planejado` ou `programado` | Origem da carga |

A aplicação ainda pode estar `queued` ou `running`. Consulte `pcp_job` pelo `id`.

## Acompanhar o Job (`pcp_job`)

Tabela no Supabase. O Bubble filtra por `id` (e, se quiser, por `tipo`).

| Coluna | Tipo | Notas |
| --- | --- | --- |
| `id` | uuid | O mesmo da resposta `201` |
| `tipo` | text | `planejado` ou `programado` |
| `status` | text | `queued` → `running` → `success` ou `failed` |
| `total` | int | Itens válidos após parse e dedupe |
| `processadas` | int | Já enviados à persistência |
| `restantes` | int | Gerado: `total - processadas` |
| `file_name` | text | Nome do CSV no Planejado; no Programado fica `programado.json` |
| `error_message` | text | Preenchido só em `failed` |
| `created_at` / `updated_at` | timestamptz | `updated_at` muda a cada avanço |

Um Job por vez (FIFO). Dá para enfileirar outro envio enquanto um corre. Não há pause/resume. Não há GET de progresso no Go.

---

## `POST /v1/planejamento`

Aplica a **Carga de Planejamento**. Multipart com o CSV no campo `file`.

### Headers

```
X-API-Key: <API_KEY>
Content-Type: multipart/form-data
```

### Body

Campo de formulário `file`: arquivo CSV. Modelo: [`docs/exemplos/carga_planejamento.csv`](exemplos/carga_planejamento.csv).

Delimitador `,` ou `;`. BOM UTF-8 ok. Header obrigatório (ordem livre, capitalização irrelevante):

| Coluna | Obrigatório | Formato |
| --- | --- | --- |
| `data` | sim | `DD/MM/AAAA` |
| `fase` | sim | texto (Fase PCP) |
| `regional` | sim | sigla: `NO`, `NE-I`, `NE-II`, `SUSE`, `COSE` |
| `fornecedor` | se `cnpj` vazio | nome; identifica a linha quando não há CNPJ |
| `cnpj` | não | como veio (máscara inclusa); vazio ok |
| `quantidade` | sim | inteiro ≥ 0 |

O serviço preenche `regional_nome`: `NO`→Norte, `NE-I`→Nordeste I, `NE-II`→Nordeste II, `SUSE`→Sudeste/Centro-Sul, `COSE`→Centro-Oeste/Minas. Sigla fora da lista grava a sigla e deixa o nome vazio.

### Identidade e regras

Chave: **data + fase + regional (sigla) + CNPJ**. Sem CNPJ, a chave usa o **nome do fornecedor**. Última ocorrência no arquivo vence.

- Reenviar a mesma chave atualiza (10 vira 9; 10 vira 0).
- Chave **nova** com quantidade `0` **não** grava.
- Chave que **não veio** neste CSV **permanece**.
- Linha sem data, fase, regional, sem CNPJ e sem nome, data em outro formato, ou quantidade não inteira: ignorada.
- Sem nenhuma linha válida: `400`, Job não é criado.
- `inep`, `uf` e `provisoria` ficam vazios no Planejado.

### Exemplo

```bash
curl -X POST http://localhost:8080/v1/planejamento \
  -H "X-API-Key: $API_KEY" \
  -F "file=@docs/exemplos/carga_planejamento.csv"
```

---

## `POST /v1/programado`

Aplica a **Carga de Programado**. JSON no body — **não** é upload de arquivo. O Bubble monta o array (ou o envelope) e POSTa.

Limite de body: **16 MB** (~3.000 objetos cabem folgado).

### Headers

```
X-API-Key: <API_KEY>
Content-Type: application/json
```

### Body

Array de objetos, ou `{"itens":[...]}`. Modelo: [`docs/exemplos/programado.json`](exemplos/programado.json).

```json
[
  {
    "data": "18/08/2026",
    "fase": "4.2",
    "regional": "NE-I",
    "uf": "BA",
    "inep": "12345678",
    "fornecedor_nome": "Fornecedor Nordeste I",
    "fornecedor_cnpj": "22.222.222/0001-22",
    "quantidade": 1,
    "provisoria": false
  }
]
```

| Campo | Obrigatório | Formato |
| --- | --- | --- |
| `data` | sim | `DD/MM/AAAA` ou `YYYY-MM-DD` |
| `fase` | sim | texto (Fase PCP) |
| `regional` | sim | mesma sigla do Planejado |
| `inep` | sim | texto ou número |
| `uf` | não | texto |
| `fornecedor_nome` | não | texto |
| `fornecedor_cnpj` | não | como veio; vazio vira `""` |
| `quantidade` | não | inteiro ≥ 0; omitido = `1` |
| `provisoria` | não | boolean |

`regional_nome` é preenchido pelo mesmo de-para do Planejado.

### Identidade e Espelho do Mês

Chave: **data + INEP**. Última ocorrência no JSON vence. Fase e regional viajam no objeto, mas não montam a chave.

O mês do espelho é o da **data do primeiro item válido**. Itens de outro mês são descartados (não gravam e não limpam o outro mês).

Depois de gravar a carga, **some o Programado daquele mês cuja chave não veio**. Planejado e Programado de outros meses não se mexem.

No Bubble, recorte do dia 1 ao último dia daquele mês. A API não usa o calendário do servidor: reenviar agosto em setembro substitui só agosto.

Objeto sem INEP, data, fase ou regional, ou com quantidade negativa: ignorado. Array vazio / sem objetos válidos: `400`, **não apaga** o mês.

Quantidade `0` em chave nova não insere; em chave que já existe, atualiza para 0 (e, como veio na carga, não é omitido).

### Exemplo

```bash
curl -X POST http://localhost:8080/v1/programado \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d @docs/exemplos/programado.json
```

No Bubble: POST com o JSON no body (não multipart).

---

## Erros

| Status | `error` | Quando |
| --- | --- | --- |
| `401` | `unauthorized` | Sem `X-API-Key` ou chave errada |
| `400` | `carga obrigatória` | Planejado sem campo `file` |
| `400` | `csv inválido` | CSV ilegível, header errado ou sem linhas válidas |
| `400` | `json inválido` | Body ilegível, vazio ou sem objetos válidos |
| `413` | `json grande demais` | Body > 16 MB |
| `500` | `falha ao criar job` | Persistência do Job falhou |

Corpo: `{"error":"..."}` (texto; o `http.Error` do Go pode acrescentar newline).

Job `failed`: a API já tinha respondido `201`. O motivo fica em `pcp_job.error_message`. No AV1, se o Programado falhar no meio, o mês pode ficar inconsistente — reenvie o JSON do mês.

## Fila

Planejado aplica em batches de ~200. Programado aplica a carga do Job de uma vez (espelho do mês). Retry curto em falha transitória. Um Job por vez, na ordem de chegada.

## O que esta API não faz

- GET de Job ou de `pcp` (só `GET /` de health)
- Autenticação de usuário final
- Upload do Programado como arquivo
- Paginação do JSON (um POST = o mês)
- Realizado
