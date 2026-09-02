# Programado do mês por data de conexão

Status: done

## Problem Statement

Quando o PCP puxa o Programado de um mês, ele só enxerga as OSPs cuja **Previsão de entrega** cai naquele mês. Mas a data que vale para o Registro PCP nem sempre é a previsão: quando a Escola está **Conectada**, a data do Programado é a data de conexão (`data_relatorio` da importação_escola).

Consequência prática: uma Folha de Registro de julho, cuja Escola só conectou em agosto, tem data de Programado em agosto e mesmo assim **não aparece** no puxar de agosto — a OSP dela nunca é buscada, porque a previsão de entrega é de julho. O usuário vê o mês atual incompleto e precisa explicar à mão por que uma conexão do mês não está no PCP.

O espelho do mês também é assimétrico no outro sentido: uma OSP com previsão em agosto que conectou em julho é puxada em agosto e gravada com data de julho — Registro com data fora do mês que o espelho diz representar.

## Solution

O recorte do mês passa a ser **a data final do Registro PCP**, não a previsão de entrega da OSP.

Puxar o mês M devolve todo Programado cuja data (a **Data do Programado**: `data_relatorio` quando a Escola está Conectada e essa data existe, senão Previsão de entrega da OSP) cai dentro de M. Para isso o puxar passa a percorrer dois caminhos e unir o resultado:

1. **Por previsão** (o de hoje): OSPs com Previsão de entrega em M.
2. **Por conexão** (novo): importações de escola com `data_relatorio` em M → INEPs → Folhas de Registro desses INEPs → as OSPs dessas folhas.

Depois da junção, todo item cuja Data do Programado cair fora de M é descartado como skip — inclusive itens vindos do caminho 1, o que corrige a assimetria acima. Regra única, um só critério de pertencimento ao mês.

## User Stories

1. Como analista de PCP, quero que uma Folha de Registro de um mês anterior cuja Escola conectou no mês atual apareça no Programado do mês atual, para que o mês reflita as conexões que de fato aconteceram nele.
2. Como analista de PCP, quero que a data gravada nesse Registro seja a data de conexão, para que o PCP conte a escola no dia em que ela conectou.
3. Como analista de PCP, quero que uma OSP com previsão no mês atual mas conexão em mês anterior **não** seja gravada no mês atual, para que o espelho do mês não contenha Registro com data de outro mês.
4. Como analista de PCP, quero que essa mesma OSP apareça quando eu puxar o mês em que ela conectou, para que ela não desapareça do PCP.
5. Como analista de PCP, quero que OSP com previsão no mês e Escola ainda não Conectada continue entrando pela previsão de entrega, para que o Programado siga mostrando o que está previsto.
6. Como analista de PCP, quero que uma Escola Conectada sem `data_relatorio` preenchido continue caindo na previsão de entrega, para que a falta do dado não apague o Programado.
7. Como analista de PCP, quero que uma Folha alcançada pelo caminho de conexão passe pelos mesmos filtros das demais (OSP não Reprovada, INEP preenchido, contrato kit de Rede Interna, Escola com Fase e Regional), para que não entre lixo por uma porta lateral.
8. Como analista de PCP, quero que uma OSP alcançada pelos dois caminhos gere um único Programado, para que a contagem do mês não duplique.
9. Como analista de PCP, quero que duas conexões do mesmo INEP na mesma data gerem uma linha só, para que a Chave da Linha de Programado siga valendo.
10. Como analista de PCP, quero que a importação considerada seja a de `data_relatorio` mais recente, como já é hoje, para que a regra de escolha da importação não mude.
11. Como analista de PCP, quero ver no resumo do puxar quantos itens vieram por previsão e quantos por conexão, para que eu saiba o que a mudança trouxe.
12. Como analista de PCP, quero que o item descartado por data fora do mês apareça na lista de skips com esse motivo, para que eu consiga explicar uma ausência sem abrir o Bubble.
13. Como analista de PCP, quero que o espelho do mês continue substituindo só aquele mês civil, para que Planejado e outros meses não sejam tocados.
14. Como analista de PCP, quero puxar um mês passado e obter o mesmo recorte por data final, para que reprocessar um mês antigo seja consistente.
15. Como operador, quero que o puxar em `test` e em `live` se comportem igual e que a origem gravada continue certa, para que o ambiente não mude a regra.
16. Como operador, quero que o caminho novo respeite o paralelismo e a paginação já usados, para que puxar o mês não fique lento nem estoure a Data API.
17. Como operador, quero que uma falha na busca de importações do mês faça o puxar falhar de forma clara, e não devolver um mês silenciosamente incompleto.
18. Como desenvolvedor, quero uma porta de busca explícita para o Bubble, para que eu teste a montagem do mês sem levantar servidor HTTP falso.

## Implementation Decisions

**Porta de busca (seam nova, decisão do dev).** A montagem do Programado do mês deixa de chamar o `Client` direto e passa a depender de uma interface de busca no pacote `bubble`, com os métodos que a montagem precisa:

- OSPs do mês (por Previsão de entrega, status ≠ Reprovado)
- OSPs por IDs
- Importações de escola com `data_relatorio` no mês
- Folhas de Registro por IDs
- Folhas de Registro por INEPs
- Contratos de instalação por IDs
- Escolas por IDs
- Importações de escola por INEPs

`Client` implementa essa interface (os métodos privados de hoje viram exportados ou ganham wrappers); `PuxarMes` vira um wrapper fino que chama a montagem passando o próprio `Client`. Assinatura e semântica de `PuxarMes` para os chamadores (HTTP API, CLI) não mudam.

**Algoritmo da montagem:**

1. Caminho previsão: OSPs do mês (constraint atual, inalterado).
2. Caminho conexão: importações com `data_relatorio` dentro do mês → INEPs distintos → Folhas por INEP → IDs de OSP dessas folhas → OSPs por ID, descartando Reprovada.
3. União das OSPs por `_id` (dedupe).
4. Daí para frente o fluxo é o de hoje: folhas por ID, contratos, escolas, importações por INEP, `ProgramadoDaFolha`.
5. Filtro final: item cuja `Data` não pertence ao mês vira skip com motivo "data fora do mês" (`SkipForaDoMes`, hoje declarado e não usado). O filtro vale para os dois caminhos.

**Constraints novas na Data API:**

- importação_escola por faixa de `data_relatorio`: `greater than` / `less than` no mês civil em America/São_Paulo, no mesmo formato já usado por `ConstraintsOSPMes`.
- fr_osp por lista de INEP: `in` sobre a chave `INEP` da folha (não a da escola — o INEP do Registro é o da folha, conforme `ProgramadoDaFolha`).

Ambas em funções puras junto das constraints existentes, para poderem ser testadas isoladas.

**Busca de folhas por INEP** é paginada e em chunks, como as demais (`DefaultPageSize`, `puxarWorkers`), com o mesmo padrão de fallback já usado em `importacoesPorINEPs`.

**Observabilidade:** o log do puxar passa a informar as duas origens (quantas OSPs por previsão, quantas por conexão, quantas após dedupe) e a contagem de itens descartados por data fora do mês.

**Não muda:** `DataProgramado`, a escolha da importação mais recente, a Chave da Linha de Programado (Data + INEP), o Espelho do Mês, o contrato de `POST /v1/programado/puxar`, o JSON do Programado.

## Testing Decisions

Bom teste aqui é teste de comportamento externo: dado um conjunto de dados do Bubble, quais Programados e quais skips o mês produz. Nada de asserção sobre ordem de chamadas ou sobre funções internas de junção.

**Módulos testados:**

- **Montagem do mês** (a nova função sobre a porta de busca) — o grosso dos casos, com uma fonte falsa em memória (sem HTTP):
  - FR de julho, escola Conectada com `data_relatorio` em agosto → entra em agosto, com a data da conexão.
  - OSP com previsão em agosto, conexão em julho → não entra em agosto.
  - OSP com previsão em agosto, escola não Conectada → entra em agosto pela previsão.
  - Escola Conectada sem `data_relatorio` → entra pela previsão de entrega.
  - Mesma OSP alcançada pelos dois caminhos → um item só.
  - OSP Reprovada alcançada pelo caminho de conexão → skip.
  - Folha sem kit de Rede Interna alcançada pelo caminho de conexão → skip "sem kit".
  - Item descartado pelo filtro final → skip com motivo de data fora do mês.
  - Erro na busca de importações do mês → erro propagado, sem resultado parcial.
- **Constraints** (funções puras) — faixa de `data_relatorio` no mês e lista de INEPs de folha, em `map_test.go`/`client_test.go`, no estilo dos testes de `ConstraintsOSPMes`.
- **`PuxarMes` sobre o `Client`** — manter um teste de fiação com o fake HTTP existente em `internal/bubble/puxar_test.go`, provando que o `Client` satisfaz a porta e que os caminhos batem nos endpoints certos. Não replicar ali a matriz de casos acima.
- **Ponta a ponta** — os testes de `POST /v1/programado/puxar` em `internal/httpapi/puxar_test.go` seguem como estão (regressão de contrato); acrescentar um caso em que o item do mês só existe por conexão.

Prior art: `internal/bubble/puxar_test.go` (fake Bubble via `httptest`), `internal/bubble/map_test.go` (funções puras de mapeamento e constraint), `internal/httpapi/puxar_test.go` (job enfileirado e aplicado com stores em memória).

## Out of Scope

- Mudar `DataProgramado` ou a definição de Escola Conectada.
- Puxar automático de vários meses numa chamada, ou reprocessar o mês anterior junto.
- Corrigir retroativamente meses já gravados (o mês antigo se corrige ao ser puxado de novo).
- Palco, transação ou desfazer do Job — segue fora do AV1.
- Qualquer mudança no Planejado ou na Carga de Planejamento.

## Further Notes

- Uma consequência esperada: puxar agosto depois da mudança pode **remover** de agosto Registros que hoje estão lá com data de julho, porque o Espelho do Mês substitui o mês. É o comportamento desejado (histórias 3 e 4), mas vale puxar em seguida o mês anterior para reancorar esses Registros.
- Volume do caminho novo é limitado às conexões do mês; se ele crescer, o gargalo é a busca de folhas por INEP, não a das importações.
