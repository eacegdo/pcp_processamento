# OCE Processamento

Contexto responsável por ingerir lotes de situação OCE por escola e refletir esse estado no armazenamento persistente. A Aplicação de Lote é acionada por chamador autorizado via API de ingestão; o progresso do Job de Aplicação é observado pelo cliente diretamente no armazenamento persistente do Job.

## Language

**Escola**:
Unidade escolar identificada pelo código INEP.
_Avoid_: Instituição, unidade (salvo em texto livre)

**INEP**:
Identificador oficial, estável e único da Escola neste contexto (uma Escola por INEP).
_Avoid_: id da escola, school_id

**Situação OCE**:
Estado OCE de uma Escola em um momento, composto por tipo de acesso, status e pendência.
_Avoid_: registro genérico, row, linha do CSV

**Status OCE**:
Campo de status da Situação OCE. Nome canônico no armazenamento: `oce_status`. No Lote OCE de entrada pode vir como `oce_status_final` e é traduzido na borda.
_Avoid_: oce_status_final (como nome persistido), status geral, statusEscola (conceitos distintos na Escola)

**Lote OCE**:
Conjunto de Situações OCE recebido de uma vez (arquivo CSV) para ser aplicado. O delimitador do CSV pode ser vírgula ou ponto e vírgula.
_Avoid_: upload, arquivo, import (como conceito de domínio)

**Aplicação de Lote**:
Atualização em massa da Situação OCE de Escolas já existentes, identificadas por INEP; não cria Escola nova. Linha cujo INEP não existe é ignorada; as demais seguem. Se o mesmo INEP repetir no lote, a última ocorrência vence. Com o cabeçalho do Lote OCE correto, campo de Situação OCE vazio no CSV é aplicado como vazio (pode limpar o valor no banco); só linha sem INEP é descartada.
_Avoid_: upsert (quando implicar inserção), insert, sync completo

**Job de Aplicação**:
Execução enfileirada de uma Aplicação de Lote, aplicada em batches contra o armazenamento, com progresso persistido e observável (processadas, total, restantes) até terminar em sucesso ou falha. Novos jobs entram em fila FIFO; um processa por vez. Falha transitória de um batch gera retry curto; se persistir, o job falha no ponto atingido e o que já foi aplicado permanece.
_Avoid_: task genérica, upload em andamento
