# 04 — Adapters Supabase + serviço rodável

**What to build:** O mesmo comportamento de ingestão e fila funciona contra o Supabase real: Jobs em `oce_job` (lidos pelo Bubble via API Connector) e updates só das colunas de Situação OCE em `escola` por INEP, sem criar Escola. O serviço sobe como um processo (HTTP + worker) configurado por variáveis de ambiente. CSV continua temporário — não há upload para Storage.

**Blocked by:** 03 — Fila com batches, progresso, FIFO e falha com retry

**Status:** ready-for-agent

- [ ] `JobStore` persiste e atualiza Jobs em `oce_job` (status, total, processadas, error_message, file_name)
- [ ] `EscolaStore` atualiza apenas `oce_tipo_acesso`, `oce_status`, `oce_pendencia` por `inep` (sem insert/upsert criador)
- [ ] Serviço inicia com config por env (`SUPABASE_URL`, `SUPABASE_SERVICE_ROLE_KEY`, `API_KEY`, opcionais de batch/retry)
- [ ] Upload autenticado contra o serviço real cria Job visível em `oce_job` para o Bubble consultar por `id`
- [ ] Aplicação de Lote reflete na tabela `escola` existente
- [ ] Documentação mínima de como aplicar o SQL de `oce_job` e subir o serviço
