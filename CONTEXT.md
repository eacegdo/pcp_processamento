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
Escola associada a OSP válida, com INEP preenchido, enviada pelo Bubble como JSON já montado. Grava-se no mesmo Registro PCP. A Carga de Planejamento não traz Programado.
_Avoid_: previsão avulsa, OSP (como se fosse o registro)

**Carga de Planejamento**:
Conjunto de linhas de Planejado recebido de uma vez para ser aplicado. Cada linha traz data (`DD/MM/AAAA`), Fase PCP, Regional PCP (sigla), nome do fornecedor (opcional), CNPJ (obrigatório) e Quantidade Planejada. Não traz Programado, INEP nem UF. Data em outro formato ou vazia invalida a linha.
_Avoid_: upload, arquivo, import, Lote OCE

**Fornecedor PCP**:
Fornecedor da Rede Interna. No Registro PCP o CNPJ entra como veio na Carga de Planejamento (em geral com máscara) e é a identidade; o nome é só rótulo para visualização e pode vir vazio.
_Avoid_: Fornecedor_eace (como identidade nesta coleção), fornecedor da OSP, fornecedor_re, CNPJ só-dígitos como forma canônica

**Chave da Linha de Programado**:
Identidade de um Programado: Data + INEP. Se o mesmo INEP repetir na mesma data no JSON, a última ocorrência vence. Sem INEP ou sem data a linha não entra.
_Avoid_: chave com CNPJ (isso é do Planejado)

**Carga de Programado**:
Conjunto de Programados recebido de uma vez em JSON (lista de objetos). O Bubble monta os objetos; este contexto só persiste.
_Avoid_: arquivo CSV, Carga de Planejamento

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

**Job de Aplicação**:
Execução enfileirada de uma Aplicação da Carga (Planejado) ou de uma Carga de Programado, em batches, com progresso persistido e observável até sucesso ou falha. Um job por vez, FIFO. O Tipo PCP do Job (`planejado` ou `programado`) identifica a origem.
_Avoid_: task genérica, upload em andamento
