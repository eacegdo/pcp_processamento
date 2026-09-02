# PCP Processamento

Contexto responsável por ingerir a Carga de Planejamento e o Programado vindo do Bubble, persistindo ambos na coleção de Registros PCP.

## Language

**Escola**:
Unidade escolar identificada pelo código INEP.
_Avoid_: Instituição, unidade (salvo em texto livre)

**INEP**:
Identificador oficial, estável e único da Escola neste contexto (uma Escola por INEP). No Registro PCP, o INEP existe no Programado e fica vazio no Planejado.
_Avoid_: id da escola, school_id, inep_fr (salvo o nome do campo no Bubble)

**Registro PCP**:
Uma linha da coleção única de PCP, ou Planejado ou Programado. Traz Tipo PCP, data, Fase PCP, Regional PCP (sigla e nome), nome do Fornecedor PCP, CNPJ do Fornecedor PCP e quantidade. UF, INEP e indicação de OSP provisória existem na coleção e ficam vazios no Planejado.
_Avoid_: tabela genérica, fato, snapshot, row

**Tipo PCP**:
Discriminador do Registro PCP. Só há dois valores: Planejado e Programado. Não há Realizado nesta coleção.
_Avoid_: Mapa_dado, Plan/Real/Prog, pcp_tipo (salvo o option set no Bubble)

**Planejado**:
Meta em número de escolas para uma data, Fase PCP, Regional PCP e Fornecedor PCP. Não aponta para uma Escola.
_Avoid_: realizado, programado, meta genérica

**Programado**:
Escola associada a OSP válida, com INEP preenchido, enviada pelo Bubble como JSON já montado. Grava-se no mesmo Registro PCP. A Carga de Planejamento não traz Programado. A busca no Bubble pode partir de Folha de Registro; o que este contexto persiste é o Programado, não a FR.
_Avoid_: previsão avulsa, OSP (como se fosse o registro), FR (como se fosse o Registro PCP)

**Data do Programado**:
Data que vai para o Registro PCP de um Programado: a data de conexão (`data_relatorio` da Importação de Escola) quando a Escola está Conectada e essa data existe; senão a Previsão de entrega da OSP. É ela que decide a qual mês o Programado pertence — não a previsão de entrega.
_Avoid_: previsão de entrega (como se fosse sempre a data), data da OSP, data da folha

**Importação de Escola**:
Registro de conexão de uma Escola, por INEP, com a data de conexão em `data_relatorio`. Vale a de `data_relatorio` mais recente. Puxar o mês parte também dela: quem conectou no mês entra no mês, mesmo com Folha de Registro de mês anterior.
_Avoid_: importação de carga, upload, sync

**Carga de Planejamento**:
Conjunto de linhas de Planejado recebido de uma vez para ser aplicado. Cada linha traz data (`DD/MM/AAAA`), Fase PCP, Regional PCP (sigla), nome do fornecedor (opcional), CNPJ (obrigatório) e Quantidade Planejada. Não traz Programado, INEP nem UF. Data em outro formato ou vazia invalida a linha.
_Avoid_: upload, arquivo, import, Lote OCE

**Fornecedor PCP**:
Fornecedor da Rede Interna. No Registro PCP o CNPJ entra como veio na Carga de Planejamento (em geral com máscara) e é a identidade; o nome é só rótulo para visualização e pode vir vazio.
_Avoid_: Fornecedor_eace (como identidade nesta coleção), fornecedor da OSP, fornecedor_re, CNPJ só-dígitos como forma canônica

**Chave da Linha de Planejamento**:
Identidade de um Planejado: Data + Fase PCP + Regional PCP (sigla como veio) + CNPJ como veio na carga. O nome do fornecedor e o nome da regional não entram na chave. Linha de Planejado sem CNPJ é inválida. Se a mesma chave repetir na Carga de Planejamento, a última ocorrência vence.
_Avoid_: chave com nome do fornecedor, chave com nome da regional, chave com INEP (Planejado não tem Escola), chave com CNPJ normalizado

**Chave da Linha de Programado**:
Identidade de um Programado: Data + INEP. Se o mesmo INEP repetir na mesma data no JSON, a última ocorrência vence. Sem INEP ou sem data a linha não entra.
_Avoid_: chave com CNPJ (isso é do Planejado), chave com Fase PCP ou Regional PCP

**Carga de Programado**:
Conjunto de Programados montado no Bubble e enviado de uma vez em JSON, já recortado a um mês civil (dia 1 ao último dia). O mês do espelho é o da data do primeiro item válido. Este contexto só persiste; não monta o JSON.
_Avoid_: arquivo CSV, Carga de Planejamento, delta, sync incremental

**Espelho do Mês**:
Substituição do Programado daquele mês civil: depois da carga, o que não veio deixa de existir naquele mês. Não mexe em Planejado nem em Programado de outro mês. Palco e carga à prova de falha no meio ficam fora do AV1.
_Avoid_: snapshot (como sinônimo de Registro PCP), apagar a coleção inteira, upsert que deixa omitidos, recusar mês que não é o calendário de hoje

**Fase PCP**:
Fase oficial da Escola, a mesma dimensão usada na Carga de Planejamento.
_Avoid_: fase do arquivo FR/OSP como fonte oficial

**Regional PCP**:
Dimensão de regional do Planejado. A identidade é a **sigla como veio** na Carga de Planejamento; o nome por extenso é rótulo ao lado, preenchido por de-para, e não entra na chave.
Siglas: `NO`, `NE-I`, `NE-II`, `SUSE`, `COSE`.
Nomes: `NO` → Norte; `NE-I` → Nordeste I; `NE-II` → Nordeste II; `SUSE` → Sudeste/Centro-Sul; `COSE` → Centro-Oeste/Minas.
_Avoid_: Gerência (equivalente histórico), regional da OSP, normalizar `NEI` como se fosse `NE-I`

**Quantidade Planejada**:
Número inteiro de escolas planejadas naquele dia, naquela chave. Não há rateio decimal da meta mensal.
_Avoid_: 4,34 / 4.34, quantidade decimal, contagem de INEP

**Aplicação da Carga**:
Persistência em massa do Planejado pela Chave da Linha de Planejamento. O valor novo da Quantidade Planejada prevalece sempre — inclusive zero e correção (de 10 para 9, por exemplo). Chave que ainda não existe e vem com quantidade zero **não é gravada**. Reenviar a carga não apaga chaves que não vieram no arquivo. Linha sem data, Fase PCP, Regional PCP ou CNPJ não entra.
_Avoid_: update em Escola, insert de cadastro escolar, sync completo, média ou soma com o valor anterior

**Aplicação do Programado**:
Persistência do Espelho do Mês: grava a Carga de Programado e em seguida remove os Programados daquele mês que não vieram. Não é a Aplicação da Carga (isso é Planejado). No AV1 não há palco nem desfazer se o Job falhar no meio.
_Avoid_: upsert que preserva omitidos, apagar Planejado, apagar outro mês

**Job de Aplicação**:
Execução enfileirada de uma Aplicação da Carga (Planejado) ou de uma Carga de Programado, em batches, com progresso persistido e observável até sucesso ou falha. Um job por vez, FIFO. O Tipo PCP do Job (`planejado` ou `programado`) identifica a origem.
_Avoid_: task genérica, upload em andamento
