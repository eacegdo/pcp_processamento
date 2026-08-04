Status: ready-for-agent

# Spec: Aplicação de Lote OCE

## Problem Statement

Operadores atualizam a Situação OCE de muitas Escolas de uma vez a partir de um CSV. Hoje isso precisa chegar ao Supabase (`public.escola`) sem criar escolas novas, com feedback de progresso no Bubble (fila: quanto falta). Falta um serviço que receba o Lote OCE, aplique updates em massa com segurança e exponha o andamento num Job consultável.

## Solution

Uma API em Go, chamada pelo Bubble com API key, recebe o arquivo CSV do Lote OCE, enfileira um Job de Aplicação (persistido em `oce_job`) e processa em batches (FIFO, um por vez): atualiza só `oce_tipo_acesso`, `oce_status` e `oce_pendencia` na Escola identificada por INEP. O Bubble acompanha o progresso lendo `oce_job` via API Connector. Linhas inválidas ou INEP inexistente são ignoradas; falhas transitórias de batch têm retry curto.

## User Stories

1. As an operador no Bubble, I want to enviar um CSV de Lote OCE para a API, so that a Situação OCE das Escolas seja atualizada em massa no Supabase.
2. As an operador no Bubble, I want receber o `id` do Job de Aplicação na resposta do upload, so that eu possa acompanhar aquele processamento.
3. As an operador no Bubble, I want consultar a tabela `oce_job` pelo `id`, so that eu veja status, total, processadas e restantes como numa fila.
4. As an operador no Bubble, I want ver o Job passar por `queued` → `running` → `success` (ou `failed`), so that eu saiba se terminou.
5. As an operador no Bubble, I want ver `restantes` diminuir ao longo do tempo, so that eu saiba quanto falta.
6. As a sistema chamador, I want autenticar com API key no header, so that só chamadores autorizados disparem Aplicação de Lote.
7. As a sistema chamador, I want ter a requisição rejeitada sem API key válida, so that o endpoint não fique aberto.
8. As an operador, I want o CSV aceitar delimitador `,` ou `;`, so that exports de planilhas brasileiras e internacionais funcionem.
9. As an operador, I want o CSV com colunas `inep`, `oce_tipo_acesso`, `oce_status_final`, `oce_pendencia`, so that o lote espelhe a origem dos dados.
10. As a sistema, I want mapear `oce_status_final` do CSV para `oce_status` na Escola, so that o nome canônico no armazenamento seja respeitado.
11. As a sistema, I want atualizar apenas Escolas cujo INEP já existe, so that a Aplicação de Lote nunca crie Escola nova.
12. As a sistema, I want ignorar linhas cujo INEP não existe em `escola`, so that um INEP errado não derrube o lote.
13. As a sistema, I want ignorar linhas com qualquer dos quatro campos vazio, so that não limpe Situação OCE no banco por acidente.
14. As a sistema, I want que, se o mesmo INEP repetir no CSV, a última ocorrência vença, so that o estado final do lote seja determinístico.
15. As a sistema, I want aplicar updates só nas colunas `oce_tipo_acesso`, `oce_status` e `oce_pendencia`, so that demais dados da Escola permaneçam intactos.
16. As a sistema, I want processar o Job em batches, so that lotes grandes avancem com progresso observável e chamadas ao Supabase sejam eficientes.
17. As a sistema, I want processar apenas um Job por vez em fila FIFO, so that uploads seguintes esperem a vez sem corrida no mesmo INEP.
18. As an operador, I want enfileirar outro upload enquanto um Job roda, so that eu não seja bloqueado no envio do arquivo.
19. As a sistema, I want persistir o Job em `oce_job` no Supabase, so that progresso sobreviva a restart do processo e o Bubble consulte a mesma fonte.
20. As a sistema, I want gravar `file_name` no Job quando disponível, so that o operador reconheça qual arquivo está na fila.
21. As a sistema, I want, em falha transitória de um batch, fazer retry curto, so that blips de rede não falhem o Job inteiro.
22. As a sistema, I want, se o retry esgotar, marcar o Job como `failed` com `error_message`, so that o operador saiba que parou.
23. As a sistema, I want preservar updates já aplicados quando o Job falha no meio, so that não haja rollback frágil; o operador pode reenviar o CSV.
24. As an operador, I want que reenviar o mesmo CSV seja seguro do ponto de vista de estado final (mesmos valores), so that eu recupere de falha parcial.
25. As a sistema, I want responder sucesso na criação do Job mesmo antes da aplicação terminar, so that o Bubble não dependa de request HTTP longo.
26. As a sistema, I want rejeitar upload que não seja CSV parseável / sem header esperado, so that lixo não entre na fila como Job válido enganoso.
27. As a sistema, I want contar em `total` apenas linhas válidas após dedupe (última vence) e filtro de vazios, so that o progresso reflita trabalho real a aplicar.
28. As a sistema, I want que INEPs ignorados por inexistência no banco ainda consumam progresso do batch (contem como processadas no sentido de “já tentadas/avançadas na fila de trabalho”), OR avançar `processadas` pelo tamanho do batch de candidatos — *clarificação na implementação: `processadas` = linhas do trabalho do job já enviadas à etapa de update (incluindo no-ops por INEP inexistente), para a barra chegar a 100%.*
29. As a ops, I want configurar URL Supabase, service role key e API key da API via ambiente, so that segredos não fiquem no código.
30. As a ops, I want um único binário/serviço Go deployável, so that a operação seja simples (sem UI neste repo).
31. As a Bubble app, I want não precisar chamar GET de progresso na API Go, so that o acompanhamento use só o API Connector no Supabase.
32. As a sistema, I want atualizar `updated_at` do Job a cada avanço de progresso, so that o Bubble detecte mudança com freshness.
33. As an agente de implementação, I want portas `EscolaStore` e `JobStore`, so that testes não dependam do Supabase real.
34. As an agente de implementação, I want a costura de teste na API HTTP de ingestão, so that regras de domínio sejam validadas pelo comportamento externo.

## Implementation Decisions

- **Stack:** API e worker em **Go** (sem frontend neste repositório; Bubble é o cliente).
- **Módulos (conceituais):**
  - **HTTP API** — autenticação por API key; `POST` multipart com arquivo CSV; resposta JSON com `id` do Job.
  - **Parser de Lote OCE** — detecta delimitador `,` ou `;`; exige header com as quatro colunas; ignora linhas incompletas; colapsa duplicatas de INEP (última vence); traduz `oce_status_final` → Status OCE (`oce_status`).
  - **JobStore** — persiste e atualiza Job de Aplicação (`oce_job`); suporte a enfileirar e reivindicar o próximo `queued` em ordem FIFO.
  - **EscolaStore** — update da Situação OCE por INEP nas três colunas; não insere; INEP inexistente = no-op.
  - **Worker** — um loop: pega próximo Job FIFO → `running` → aplica em batches → atualiza progresso → `success` / `failed`; retry curto por batch.
- **Schema:** tabela `public.oce_job` conforme SQL já acordado (`docs/sql/oce_job.sql`): `id`, `status` (`queued`|`running`|`success`|`failed`), `total`, `processadas`, `restantes` (generated), `error_message`, `file_name`, `created_at`, `updated_at`. Tabela `public.escola` já existente; colunas alvo `oce_tipo_acesso`, `oce_status`, `oce_pendencia`; chave de match `inep` (único).
- **Contrato API (ingestão):**
  - Método: `POST` (path sugerido `/v1/lotes` ou `/lotes`).
  - Auth: header de API key (ex.: `X-API-Key`).
  - Body: `multipart/form-data` com campo de arquivo.
  - Sucesso: `201`/`200` com `{ "id": "<uuid>" }` (e opcionalmente `status: "queued"`).
  - Falha de auth / CSV inválido: erro HTTP adequado; não cria Job (ou não deixa Job aplicável).
- **Progresso:** Bubble lê `oce_job` via Supabase API Connector (RLS já aberta no projeto do usuário). API Go não precisa expor GET de status.
- **Batch:** tamanho ~200 linhas por chamada de update ao Supabase (ajustável por env).
- **Semântica de `processadas`:** avança conforme o trabalho do Job é consumido em batches (incluindo no-ops), de forma que ao terminar com sucesso `processadas == total` e `restantes == 0`.
- **Integração Supabase:** service role no servidor Go; updates via API PostgREST (ou client equivalente), nunca upsert que crie linha nova.
- **Config:** `SUPABASE_URL`, `SUPABASE_SERVICE_ROLE_KEY`, `API_KEY`, opcionalmente `BATCH_SIZE`, `BATCH_MAX_RETRIES`.
- **Deploy:** processo único (HTTP + worker no mesmo binário), suficiente para o volume esperado.

## Testing Decisions

- Bom teste verifica **comportamento externo** na costura HTTP + portas em memória: não afirma detalhes internos de SQL, nomes de funções privadas ou estrutura de pastas.
- **Costura única:** API HTTP de ingestão do Lote OCE, com `EscolaStore` e `JobStore` falsos/in-memory; worker acionado de forma determinística no teste (ou tick controlado).
- Cobrir no mínimo: auth; parse `,` e `;`; mapeamento `oce_status_final` → `oce_status`; ignore linha vazia; ignore INEP inexistente; última duplicata vence; criação de Job e progresso até `success`; FIFO com dois Jobs; falha de batch após retries → `failed` com progresso parcial preservado no store de Escola.
- Prior art: repositório greenfield — estes serão os primeiros testes; estabelecer o padrão httptest + fakes nas portas.

## Out of Scope

- Persistência do arquivo CSV em Storage (o Lote OCE é parseado na ingestão; o arquivo é temporário).
- UI neste repositório (Bubble já cobre upload e leitura de progresso).
- Criação ou exclusão de Escolas.
- Histórico versionado de Situação OCE (só estado atual na `escola`).
- Validação de enums/valores permitidos além de “não vazio”.
- UNIQUE constraint migration em `escola.inep` (tratado como já único no domínio).
- Autenticação de usuários finais / JWT Supabase Auth na API Go.
- GET de progresso na API Go.
- Rollback compensatório de batches já aplicados.
- Processamento paralelo de múltiplos Jobs.
- RLS/policies novas no Supabase (projeto já consome com RLS aberta).
- Internacionalização de mensagens de erro para o Bubble além de `error_message` textual.

## Further Notes

- Glossário: `CONTEXT.md`.
- SQL do Job: `docs/sql/oce_job.sql` — deve ser aplicado no Supabase antes do go-live.
- Issue tracker deste repo: markdown local em `.scratch/` (`docs/agents/issue-tracker.md`).
- ADRs recomendados na implementação (não bloqueantes do spec): escolha de Go; persistência do Job no Supabase vs memória.
- Próximo passo no fluxo: `/to-tickets` para quebrar em tickets tracer-bullet, depois `/implement` por ticket.
