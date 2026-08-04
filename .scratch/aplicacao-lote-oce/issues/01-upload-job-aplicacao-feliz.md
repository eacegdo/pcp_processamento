# 01 — Upload autenticado → Job → Aplicação feliz (fakes)

**What to build:** Um chamador autorizado envia um CSV mínimo de Lote OCE; a API cria um Job de Aplicação, aplica a Situação OCE numa Escola que já existe (stores em memória) e o Job termina em `success`, devolvendo o `id` do Job. Sem API key ou com CSV inválido, a operação falha e não aplica nada. O arquivo CSV é temporário (parse na hora; não vai para Storage).

**Blocked by:** None — can start immediately.

**Status:** done

- [x] `POST` com API key válida e CSV mínimo retorna o `id` de um Job de Aplicação
- [x] Após o processamento, a Situação OCE da Escola existente reflete os valores do CSV (`oce_tipo_acesso`, Status OCE a partir de `oce_status_final`, `oce_pendencia`)
- [x] O Job chega a `success` com progresso coerente (`processadas` / `total`)
- [x] Requisição sem API key válida é rejeitada e não cria/aplica Job
- [x] CSV sem header esperado / não parseável é rejeitado e não aplica Situação OCE
- [x] Comportamento verificado na costura HTTP com `EscolaStore` e `JobStore` em memória (sem Supabase)
