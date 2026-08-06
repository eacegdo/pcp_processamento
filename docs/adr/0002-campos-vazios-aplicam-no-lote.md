# Campos vazios do Lote OCE são aplicados

Antes, qualquer campo vazio descartava a linha para não limpar Situação OCE por acidente. Na prática isso excluía milhares de Escolas com `oce_tipo_acesso` em branco no CSV. Decidimos: com cabeçalho válido, vazio entra e atualiza (pode limpar); só linha sem INEP é ignorada.
