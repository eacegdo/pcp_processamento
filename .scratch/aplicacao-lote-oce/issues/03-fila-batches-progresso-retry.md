# 03 — Fila com batches, progresso, FIFO e falha com retry

**What to build:** O operador vê o Job avançar como fila: aplicação em batches com `processadas`/`restantes` atualizando; um segundo upload entra `queued` e só roda depois do primeiro (FIFO); falha transitória de batch faz retry curto e, se persistir, o Job vai para `failed` com `error_message`, mantendo o que já foi aplicado.

**Blocked by:** 02 — Regras completas do Lote OCE no caminho da API

**Status:** ready-for-agent

- [ ] Job em andamento atualiza `processadas` (e `restantes`) a cada batch até `success` com `processadas == total`
- [ ] Enquanto um Job está `running`, um novo upload cria outro Job `queued` que só inicia depois
- [ ] Ordem FIFO: o Job enfileirado primeiro é processado primeiro
- [ ] Falha transitória de um batch é retentada um número curto de vezes antes de desistir
- [ ] Esgotados os retries, Job fica `failed` com `error_message` e updates já aplicados permanecem no `EscolaStore`
- [ ] Comportamento verificável com fakes (batch size pequeno / store que falha sob comando)
