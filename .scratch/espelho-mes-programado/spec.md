Status: ready-for-agent

# Spec: Espelho do Mês do Programado

## Problem Statement

O Bubble já envia a Carga de Programado em JSON (~3.000 objetos do mês civil). A API grava por upsert na Chave da Linha de Programado (Data + INEP), mas **não tira** Escola que saiu da busca. Reenviar o mês todo dia deixa Programado morto no dashboard. O operador precisa que a coleção daquele mês seja um **Espelho do Mês**: o que não veio nesta carga deixa de existir naquele mês, sem mexer em Planejado nem em outro mês.

## Solution

Cada `POST /v1/programado` continua enfileirando um Job de Aplicação (`tipo = programado`). O mês do espelho é o da data do **primeiro item válido**. A Aplicação do Programado grava a carga e, em seguida, remove os Programados daquele mês civil que não vieram. O Bubble já recorta dia 1 ao último dia; a API não usa o calendário do servidor. Palco, transação à prova de falha no meio e delta ficam fora do AV1.

## User Stories

1. As an operador no Bubble, I want enviar o JSON completo do mês atual para a API, so that o Programado no Supabase espelhe a busca daquele mês.
2. As an operador no Bubble, I want montar o JSON no Bubble (array ou `itens`) e mandar no body, so that eu não envie arquivo.
3. As an operador no Bubble, I want receber o `id` e o `tipo` do Job na resposta, so that eu acompanhe aquela Aplicação do Programado.
4. As an operador no Bubble, I want o Job gravar `tipo = programado`, so that eu distinga da Aplicação da Carga de Planejamento.
5. As a sistema chamador, I want autenticar com a mesma API key do Planejado, so that só chamadores autorizados disparem a carga.
6. As a sistema chamador, I want requisição sem API key válida ser rejeitada, so that o endpoint não fique aberto.
7. As a sistema, I want responder `201` com o `id` do Job antes da aplicação terminar, so that o Bubble não espere os ~3.000 itens persistirem.
8. As a sistema, I want rejeitar JSON inválido ou sem objetos válidos sem criar Job, so that lixo não entre na fila.
9. As a sistema, I want aceitar cerca de 3.000 objetos numa requisição (limite de body já existente), so that o mês caiba de uma vez.
10. As an operador no Bubble, I want filtrar no Bubble do dia 1 ao último dia daquele mês, so that a carga já nasça recortada a um mês civil.
11. As a sistema, I want inferir o mês do espelho pela data do primeiro item válido, so that eu não dependa do relógio do servidor nem de um campo `mes` extra.
12. As a sistema, I want não recusar um mês só porque não é o calendário de hoje, so that reenviar agosto em setembro ainda substitua só agosto.
13. As a sistema, I want ignorar item cuja data não seja daquele mês civil inferido, so that um objeto stray não grave nem limpe outro mês.
14. As a sistema, I want, se não houver nenhum item válido, não criar Job e não apagar mês nenhum, so that array vazio não limpe o Programado.
15. As a sistema, I want identificar cada Programado por Data + INEP, so that a aplicação seja determinística por Escola e dia.
16. As a sistema, I want, se o mesmo INEP repetir na mesma data no JSON, a última ocorrência vencer, so that o espelho tenha uma linha por chave.
17. As a sistema, I want Fase PCP e Regional PCP viajarem no objeto mas não montarem a chave, so that a mesma Escola no mesmo dia não duplique por fase ou sigla.
18. As a sistema, I want gravar Tipo PCP `programado` em toda linha desta carga, so that não se misture com Planejado.
19. As a sistema, I want persistir UF, nome e CNPJ do Fornecedor PCP, quantidade e indicação de OSP provisória quando vierem, so that o dashboard leia o Programado completo.
20. As a sistema, I want preencher `regional_nome` pelo mesmo de-para do Planejado, so that a sigla ganhe o nome por extenso sem o Bubble calcular.
21. As a sistema, I want INEP aceitar texto ou número, so that o Bubble não quebre se mandar o código sem aspas.
22. As a sistema, I want aceitar data `DD/MM/AAAA` ou `YYYY-MM-DD`, so that o JSON do Bubble não dependa de um único formato.
23. As a sistema, I want objeto sem INEP, data, fase ou regional ser ignorado, so that chave incompleta não entre.
24. As a sistema, I want quantidade omitida virar `1`, so that o Bubble não precise mandar 1 em todo objeto.
25. As a sistema, I want quantidade negativa ser ignorada, so that não nasça Programado inválido.
26. As a sistema, I want, após gravar a carga, remover Programado daquele mês cuja chave não veio, so that Escola que saiu da busca desapareça do dashboard naquele mês.
27. As a sistema, I want não remover Programado de outro mês, so that o histórico mensal anterior permaneça.
28. As a sistema, I want não remover nem alterar Planejado, so that a Carga de Planejamento continue dona da meta.
29. As a sistema, I want chave nova com quantidade `0` não inserir, so that não nasça Programado zerado sem existência prévia — e, se a chave não veio com quantidade positiva, ela não fica no espelho.
30. As a sistema, I want chave existente no mês atualizada pelos campos novos (inclusive quantidade `0` se vier na carga), so that correção de atributo prevaleça; se a chave não veio, ela some na limpeza, não precisa de zero sentinela.
31. As a sistema, I want a remoção dos omitidos acontecer só depois que a carga inteira daquele Job foi gravada, so that um recorte de 200 itens não apague o resto do mês que ainda vai entrar.
32. As a sistema, I want no AV1 não usar tabela palco nem desfazer se o Job falhar no meio, so that a primeira versão fique simples.
33. As a sistema, I want, se o Job falhar no meio, preservar o que já foi aplicado e não garantir espelho consistente, so that o operador reenvie a carga do mês.
34. As a sistema, I want processar um Job por vez em FIFO, so that duas Cargas de Programado (ou uma de Planejado) não corram na mesma coleção.
35. As an operador, I want enfileirar outra carga enquanto um Job roda, so that o envio do Bubble não seja bloqueado.
36. As a sistema, I want persistir o Job no Supabase com `tipo`, so that o Bubble filtre Programado vs Planejado no API Connector.
37. As a Bubble app, I want não precisar de GET de progresso na API Go, so that o acompanhamento use só a tabela de Job.
38. As a ops, I want a RPC de Programado passar a fazer o Espelho do Mês (grava e remove omitidos daquele mês), so that o worker não apague linha a linha na API.
39. As a ops, I want SQL aplicável no SQL Editor sem dropar Planejado, so that o go-live do espelho não recrie a coleção.
40. As an agente de implementação, I want a costura HTTP já usada no Programado passar a observar o Espelho do Mês, so that a regra nova seja o comportamento externo, não um detalhe da RPC.

## Implementation Decisions

- **Contrato HTTP permanece:** `POST /v1/programado`, `Content-Type: application/json`, `X-API-Key`, body array ou `{"itens":[...]}`, limite de body já existente, `201` com `{ "id", "tipo" }`. Sem upload de arquivo.
- **Parser:** infere o mês civil (`YYYY-MM`) da data do primeiro item válido. Itens de outro mês são descartados (não entram no Job). Sem item válido: `400`, não cria Job, não apaga. Dedupe pela Chave da Linha de Programado (última vence). De-para de Regional PCP igual ao Planejado.
- **Job:** `tipo = programado`; `total` = quantidade de chaves válidas após filtro de mês e dedupe. Fila FIFO e um Job por vez não mudam.
- **Aplicação do Programado (AV1):** gravar todos os itens do Job e **só então** remover Programado cujo `data` cai no mês inferido e cuja chave (data, INEP) não está no conjunto da carga. Não apagar a coleção inteira. Não apagar `tipo = planejado`. Não apagar Programado de outro mês.
- **Batches:** o worker hoje recorta ~200. A limpeza dos omitidos **não** pode rodar por batch parcial (apagaria o resto do mês). AV1: a persistência do Programado deste Job trata a carga como um conjunto só para efeito do espelho — ou um único `ApplyBatch` com todos os itens do Job, ou upsert por batch e uma finalização explícita só quando `processadas == total`, passando o conjunto de chaves (e o mês) da carga. Palco e transação que restaura o espelho antigo ficam fora.
- **PcpStore:** o batch de Programado passa a significar Espelho do Mês, não só upsert. A porta em memória e o adapter PostgREST seguem o mesmo contrato. A RPC existente de Programado é reescrita para: upsert/insert das linhas recebidas e delete dos Programados do mês civil da carga cuja chave não veio. O mês na RPC é o das datas dos itens recebidos (o parser já filtrou).
- **Falha no meio (AV1):** retry curto do batch como hoje; se esgotar, Job `failed` com `error_message`; o que já persistiu permanece; omitidos podem não ter sido limpos. Operador reenvia o JSON do mês.
- **Schema:** índice único parcial Programado `(data, inep)` permanece. Sem tabela palco. Sem coluna de geração/versão no AV1.
- **Volume:** ~3.000 objetos numa chamada é esperado e cabe; não há paginação na API Go.

## Testing Decisions

- Bom teste verifica **comportamento externo**: HTTP de ingestão, estado do Job e Registros PCP visíveis na porta em memória. Não afirma SQL interno, nomes de função privada ou árvore de pastas.
- **Seam único (já existente):** `POST /v1/programado` autenticado + `JobStore` em memória + worker tickado no teste + `PcpStore` em memória. Observar por Chave da Linha de Programado (data + INEP) e por ausência: Programado do mês que não veio some; Planejado e Programado de outro mês permanecem. Sem Supabase real nessa costura. Sem seam novo.
- Cobrir no mínimo: carga do mês remove INEP que existia e não veio; INEP que veio é atualizado; primeiro item define o mês; item de outro mês não grava e não limpa o outro mês; Planejado no mesmo dia/mês intocado; Job `tipo = programado` e `success`; JSON inválido / sem válidos não cria Job e não apaga; última chave vence; auth.
- Prior art: costura em `internal/httpapi` (Programado e Planejado); parser em `internal/programado`; adapter PostgREST falso em `internal/supabase` para a RPC — o adapter passa a esperar a RPC de espelho, sem substituir a costura HTTP.

## Out of Scope

- Tabela palco, geração/versão e garantia observável de tudo-ou-nada se o Job falhar no meio.
- Delta / sync incremental (só o que mudou).
- Recusar carga cujo mês não é o calendário de hoje.
- Campo extra `mes` no envelope.
- Apagar a coleção PCP inteira ou qualquer Planejado.
- Realizado.
- GET de progresso na API Go.
- Processamento paralelo de múltiplos Jobs.
- Autenticação JWT / usuários finais na API Go.
- UI neste repositório.
- Paginação do JSON no Bubble para várias chamadas do mesmo mês (AV1 assume uma carga com o mês).
- Mudar o contrato de ingestão do Planejado (CSV / upsert que não apaga omitidos).

## Further Notes

- Glossário: `CONTEXT.md` (Espelho do Mês, Carga de Programado, Aplicação do Programado, Chave da Linha de Programado).
- A busca no Bubble pode ser Folha de Registro; o que este contexto persiste é Programado.
- Issue tracker: markdown em `.scratch/` (`docs/agents/issue-tracker.md`).
- Próximo passo no fluxo: `/to-tickets` para tracer-bullets, depois `/implement` por ticket.
- ADR de palco/tudo-ou-nada foi oferecido e adiado: AV1 aceita espelho inconsistente se o Job falhar no meio; o operador reenvia o mês.
