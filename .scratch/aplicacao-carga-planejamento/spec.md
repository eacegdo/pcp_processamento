Status: done

# Spec: Aplicação da Carga de Planejamento

## Problem Statement

Este clone ainda aplica Lote OCE em `escola`. O operador do PCP precisa enviar uma Carga de Planejamento e ver o Planejado gravado no Supabase, numa coleção de Registro PCP que o dashboard consiga ler. Programado usará a mesma coleção depois; Realizado não entra. O remoto e os nomes ainda são OCE.

## Solution

O serviço deixa de tocar Escola e Situação OCE. Um chamador autorizado envia o CSV da Carga de Planejamento; a API cria um Job de Aplicação; o worker aplica o Planejado em batches na coleção de Registro PCP (upsert pela Chave da Linha de Planejamento). O Bubble acompanha o Job e lê o Planejado nessa coleção. A tabela já nasce com colunas do Programado, vazias. Programado não é calculado nesta entrega.

## User Stories

1. As an operador no Bubble, I want enviar um CSV de Carga de Planejamento para a API, so that o Planejado seja gravado no Supabase.
2. As an operador no Bubble, I want receber o `id` do Job de Aplicação na resposta, so that eu possa acompanhar aquele processamento.
3. As an operador no Bubble, I want consultar a tabela de Job pelo `id`, so that eu veja status, total, processadas e restantes.
4. As an operador no Bubble, I want ver o Job passar por `queued` → `running` → `success` (ou `failed`), so that eu saiba se terminou.
5. As an operador no Bubble, I want ver `restantes` diminuir ao longo do tempo, so that eu saiba quanto falta.
6. As a sistema chamador, I want autenticar com API key no header, so that só chamadores autorizados disparem Aplicação da Carga.
7. As a sistema chamador, I want ter a requisição rejeitada sem API key válida, so that o endpoint não fique aberto.
8. As an operador, I want o CSV aceitar delimitador `,` ou `;`, so that exports de planilhas brasileiras e internacionais funcionem.
9. As an operador, I want o CSV aceitar BOM UTF-8 no header, so that arquivo salvo pelo Excel não seja rejeitado.
10. As an operador, I want o CSV com colunas `data`, `fase`, `regional`, `fornecedor`, `cnpj`, `quantidade`, so that a carga traga a Chave da Linha de Planejamento e a Quantidade Planejada.
11. As an operador, I want as colunas serem reconhecidas mesmo fora de ordem, so that a ordem no Excel não quebre a carga.
12. As an operador, I want a data no formato `DD/MM/AAAA`, so that o dia planejado seja o que eu escrevi na planilha.
13. As a sistema, I want rejeitar linha cuja data esteja vazia ou em outro formato, so that a chave não nasça ambígua.
14. As a sistema, I want persistir a data como date, so that filtros de período no dashboard não dependam de texto.
15. As an operador, I want a Quantidade Planejada ser um inteiro de escolas naquele dia, so that não haja rateio decimal da meta mensal.
16. As a sistema, I want rejeitar quantidade não inteira (`4,34`, `4.34`), so that o Planejado não reintroduza o rateio do PCP_Bubble.
17. As a sistema, I want tratar quantidade vazia como linha inválida (não entra), so that célula em branco não seja confundida com zero explícito.
18. As an operador, I want o CNPJ gravado como veio (máscara inclusa), so that eu reconheça o valor da planilha na tabela.
19. As a sistema, I want usar o CNPJ como veio na Chave da Linha de Planejamento, so that não haja forma canônica de dígitos.
20. As a sistema, I want rejeitar linha de Planejado sem CNPJ, so that não exista Planejado sem identidade de Fornecedor PCP.
21. As an operador, I want o nome do fornecedor opcional, so that CNPJ sozinho ainda grave o Planejado.
22. As an operador, I want o nome do fornecedor gravado só para visualização, so that eu identifique a linha quando o CNPJ for difícil de ler.
23. As a sistema, I want que o nome do fornecedor não entre na chave, so that troca de grafia do nome não crie outro Planejado.
24. As an operador, I want a regional da carga ser a sigla (`NO`, `NE-I`, `NE-II`, `SUSE`, `COSE`), so that o arquivo continue igual ao filtro do Bubble.
25. As a sistema, I want usar a sigla como veio na chave, so that `NE-I` e `NEI` não sejam tratados como a mesma Regional PCP.
26. As an operador, I want uma coluna de nome da regional preenchida pelo serviço, so that eu leia Norte, Nordeste I, etc. sem calcular de cabeça.
27. As a sistema, I want o de-para `NO`→Norte, `NE-I`→Nordeste I, `NE-II`→Nordeste II, `SUSE`→Sudeste/Centro-Sul, `COSE`→Centro-Oeste/Minas, so that o nome não venha no CSV.
28. As a sistema, I want, se a sigla não estiver no de-para, gravar a sigla e deixar o nome vazio, so that a carga não caia por regional nova.
29. As a sistema, I want que o nome da regional não entre na chave, so that ajuste de grafia do de-para não duplique Planejado.
30. As a sistema, I want gravar Tipo PCP `planejado` em toda linha desta carga, so that Programado não se misture na ingestão.
31. As a sistema, I want deixar `inep`, `uf` e `provisoria` vazios no Planejado, so that a coleção já comporte Programado depois.
32. As a sistema, I want identificar o Planejado por data + Fase PCP + sigla da Regional PCP + CNPJ, so that a Aplicação da Carga seja determinística.
33. As a sistema, I want, se a mesma chave repetir no arquivo, a última ocorrência vencer, so that o estado final da carga seja único por chave.
34. As an operador, I want, ao reenviar a mesma chave com quantidade diferente, o valor novo prevalecer, so that eu corrija o planejamento (10 vira 9).
35. As an operador, I want, ao reenviar a mesma chave com quantidade `0`, o registro existente ir para zero, so that aquele dia saia da Meta nos gráficos sem eu apagar a linha na mão.
36. As a sistema, I want não gravar chave nova quando a quantidade vier `0`, so that não nasça Registro PCP zerado sem planejamento prévio.
37. As a sistema, I want não apagar chaves que não vieram no arquivo, so that reenviar um recorte da carga não limpe o resto do Planejado.
38. As a sistema, I want rejeitar CSV sem header esperado ou não parseável, so that lixo não entre na fila como Job válido.
39. As a sistema, I want rejeitar CSV sem nenhuma linha válida, so that não se crie Job vazio enganoso.
40. As a sistema, I want contar em `total` só as linhas válidas após dedupe (última vence), so that o progresso reflita o trabalho a aplicar — inclusive chave existente que vai a zero.
41. As a sistema, I want processar o Job em batches, so that cargas grandes avancem com progresso observável.
42. As a sistema, I want processar apenas um Job por vez em fila FIFO, so that cargas seguintes esperem a vez sem corrida na mesma chave.
43. As an operador, I want enfileirar outra carga enquanto um Job roda, so that eu não seja bloqueado no envio.
44. As a sistema, I want persistir o Job no Supabase, so that o progresso sobreviva a restart e o Bubble consulte a mesma fonte.
45. As a sistema, I want gravar `file_name` no Job quando disponível, so that o operador reconheça o arquivo na fila.
46. As a sistema, I want, em falha transitória de um batch, fazer retry curto, so that blip de rede não falhe o Job inteiro.
47. As a sistema, I want, se o retry esgotar, marcar o Job como `failed` com `error_message`, so that o operador saiba que parou.
48. As a sistema, I want preservar Registros PCP já aplicados quando o Job falha no meio, so that não haja rollback; o operador reenvia a carga.
49. As a sistema, I want responder o `id` do Job antes da aplicação terminar, so that o Bubble não dependa de request HTTP longo.
50. As a Bubble app, I want não precisar de GET de progresso na API Go, so that o acompanhamento use só o API Connector no Supabase.
51. As a sistema, I want atualizar `updated_at` do Job a cada avanço, so that o Bubble detecte mudança.
52. As a sistema, I want `processadas` avançar pelo trabalho já enviado à persistência, so that ao terminar com sucesso `processadas == total` e `restantes == 0`.
53. As a ops, I want configurar URL Supabase, service role key e API key via ambiente, so that segredos não fiquem no código.
54. As a ops, I want um único binário Go (HTTP + worker), so that o deploy continue uma réplica, como já decidido.
55. As a sistema, I want parar de atualizar `escola` e de chamar a RPC de Situação OCE, so that este produto não misture PCP com OCE.
56. As an operador, I want consultar os Registros PCP de Tipo PCP planejado no Supabase, so that o dashboard leia a meta da coleção nova.
57. As an agente de implementação, I want uma porta de persistência de Registro PCP no lugar da porta de Escola, so that os testes não dependam do Supabase real.
58. As an agente de implementação, I want a costura de teste na API HTTP de ingestão, so that as regras da Aplicação da Carga sejam validadas pelo comportamento externo.
59. As a ops, I want SQL da coleção de Registro PCP e da tabela de Job aplicável no SQL Editor, so that o go-live não dependa de adivinhar o schema.
60. As a ops, I want o módulo Go e o binário deixarem o nome OCE, so that este clone seja o PCP Processamento.
61. As a sistema, I want linha sem data, fase, regional ou CNPJ ser ignorada, so that a chave incompleta não grave Planejado.
62. As an operador, I want colunas do CSV em qualquer capitalização razoável do header (`Data` vs `data`), so that o Excel não quebre o parse.

## Implementation Decisions

- **Stack:** API e worker em Go; sem frontend neste repositório; Bubble é o cliente. ADR de deploy (uma réplica, Docker/EasyPanel) permanece.
- **Substituição de domínio:** o produto deixa de ser Aplicação de Lote OCE. Portas e vocabulário passam a Carga de Planejamento, Registro PCP, Job de Aplicação. A porta que hoje atualiza Escola some; no lugar, uma porta aplica batch de Planejado.
- **Módulos (conceituais):**
  - **HTTP API** — API key; `POST` multipart com arquivo; JSON `{ "id": "<uuid>" }`.
  - **Parser da Carga de Planejamento** — delimitador `,` ou `;`; BOM UTF-8; header das seis colunas; data `DD/MM/AAAA`; quantidade inteira; ignora linha inválida; colapsa duplicata pela chave (última vence); preenche nome da regional pelo de-para; não exige nome do fornecedor.
  - **JobStore** — mesmo contrato de fila FIFO; persiste numa tabela de Job PCP (não `oce_job`).
  - **PcpStore** — aplica batch de Planejado na coleção de Registro PCP; upsert pela chave; não calcula Programado.
  - **Worker** — um Job por vez; batches; retry curto; `success` / `failed` com progresso parcial preservado.
- **Contrato HTTP:** `POST /v1/planejamento` (deixa de ser `/v1/lotes` e `/v1/cargas`). Auth `X-API-Key`. Campo de arquivo `file`. Sucesso `201` com `{ "id": "..." }`. Sem API key: `401`, não cria Job. CSV inválido / sem linhas válidas: `400`, não cria Job.
- **Progresso:** Bubble lê a tabela de Job via API Connector. A API Go não expõe GET de status.
- **Coleção de Registro PCP** (tabela nova no Supabase), colunas:
  - `tipo` — `planejado` | `programado`
  - `data` — date
  - `fase` — text
  - `regional` — sigla text
  - `regional_nome` — text, pode vazio
  - `uf` — text, vazio no Planejado
  - `inep` — text, vazio no Planejado
  - `fornecedor_nome` — text, pode vazio
  - `fornecedor_cnpj` — text
  - `quantidade` — integer
  - `provisoria` — boolean nulo no Planejado
- **Identidade no banco (Planejado):** índice único parcial em (`data`, `fase`, `regional`, `fornecedor_cnpj`) onde `tipo = 'planejado'`. Programado não usa esse índice.
- **RPC de batch:** uma função que recebe os itens do batch e, para cada um: se a chave de Planejado existe, atualiza quantidade, `fornecedor_nome` e `regional_nome` (inclusive para zero); se não existe e quantidade > 0, insere com `tipo = planejado` e colunas de Programado nulas; se não existe e quantidade = 0, não insere. Service role executa; não há upsert que crie Escola.
- **De-para de Regional PCP** no serviço (não na carga): `NO`→Norte, `NE-I`→Nordeste I, `NE-II`→Nordeste II, `SUSE`→Sudeste/Centro-Sul, `COSE`→Centro-Oeste/Minas.
- **Job:** mesma forma de `oce_job` (`id`, `status`, `total`, `processadas`, `restantes` generated, `error_message`, `file_name`, timestamps), com nome de tabela PCP. Trigger de `updated_at` igual.
- **Batch:** ~200 linhas, `BATCH_SIZE` / `BATCH_MAX_RETRIES` / `HTTP_ADDR` como hoje.
- **Config:** `SUPABASE_URL`, `SUPABASE_SERVICE_ROLE_KEY`, `API_KEY` obrigatórios.
- **Integração:** PostgREST com service role; RPC de Planejado no lugar da RPC `aplicar_situacao_oce_lote`.
- **Nomes do clone:** módulo Go, binário e vocabulário deixam `oce_processamento` / `oce-processamento`. Tabela `escola` e RPC OCE saem do caminho deste serviço.
- **ADR 0002** (campo vazio do Lote OCE aplica e limpa) **não vale** para esta carga: campo obrigatório vazio descarta a linha; zero explícito é outra regra.

## Testing Decisions

- Bom teste verifica **comportamento externo**: HTTP de ingestão, estado do Job e Registros PCP visíveis na porta em memória. Não afirma SQL interno, nomes de função privada ou árvore de pastas.
- **Costura única (seam existente, só a porta de destino muda):** a mesma costura de hoje — `POST` autenticado + `JobStore` em memória + worker tickado no teste — com a porta de Escola trocada por uma porta de Registro PCP em memória (inspecionar por chave: quantidade, tipo, sigla, nome da regional, CNPJ como veio, INEP vazio). Sem Supabase real nessa costura.
- Cobrir no mínimo: auth; `,` e `;`; BOM; colunas fora de ordem; data `DD/MM/AAAA` vs inválida; quantidade inteira vs decimal vs vazia vs `0`; CNPJ obrigatório / nome opcional; última chave vence; 10→9; 10→0 em chave existente; chave nova com `0` não grava; chave omitida no reenvio permanece; de-para de regional e sigla desconhecida (nome vazio); Job até `success`; FIFO; retry; `failed` com o que já foi aplicado preservado; CSV inválido não cria Job.
- Prior art: `internal/httpapi` costura HTTP + fakes; `internal/lote` parse; `internal/supabase` PostgREST falso via `httptest` para o adapter da RPC — o adapter novo segue esse padrão, sem substituir a costura HTTP.

## Out of Scope

- Cálculo e gravação de Programado (OSP, FR_OSP, Kit, previsão, INEP).
- Realizado e qualquer carga com `Mapa_dado` / Plan-Real-Prog do Excel histórico.
- Paridade numérica com `Dados_PCP (1).xlsx`.
- Normalização de CNPJ para 14 dígitos.
- Normalização `NEI` ↔ `NE-I`.
- Criar, atualizar ou ler `escola`.
- Data Type `PCP` nativo do Bubble (este serviço grava no Supabase).
- Persistência do arquivo CSV em Storage.
- UI neste repositório.
- GET de progresso na API Go.
- Processamento paralelo de múltiplos Jobs.
- Rollback compensatório.
- Autenticação JWT / usuários finais na API Go.
- Criar o repositório GitHub remoto (humano: repo novo `pcp_processamento`, sem push no `oce_processamento`).
- Pause/resume de Job.
- Validação de Fase PCP contra lista fechada.
- UF no Planejado.
- Quantidade decimal / rateio mensal.

## Further Notes

- Glossário: `CONTEXT.md`.
- SQL OCE existente (`oce_job`, `aplicar_situacao_oce_lote`, `exportar_escolas`) não é o alvo desta entrega; o agente produz SQL novo da coleção PCP e do Job PCP.
- Issue tracker: markdown em `.scratch/` (`docs/agents/issue-tracker.md`).
- Próximo passo no fluxo: `/to-tickets` para tracer-bullets, depois `/implement` por ticket.
- De-para de `SUSE` conferido com o usuário: Sudeste/Centro-Sul. Nordeste I / II em romano.
