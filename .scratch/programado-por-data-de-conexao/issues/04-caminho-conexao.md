# 04 — Puxar também pelas conexões do mês

**What to build:** puxar o mês passa a enxergar as escolas que **conectaram** nele, não só as OSPs previstas para ele. Uma Folha de Registro de um mês anterior, cuja Escola conectou no mês pedido, aparece no Programado desse mês com a data da conexão — hoje ela simplesmente não aparece, porque a OSP dela nunca é buscada.

A montagem percorre dois caminhos e une o resultado: o de hoje (OSPs com previsão de entrega no mês) e o novo (importações de escola com data de conexão no mês → INEPs distintos → Folhas de Registro desses INEPs → as OSPs dessas folhas, descartando OSP Reprovada). As OSPs são unidas sem duplicar, e daí para frente o fluxo é o de hoje, incluindo o filtro final de data. Nada entra por porta lateral: a folha alcançada por conexão passa pelos mesmos filtros das outras. A busca de folhas por INEP respeita a paginação, os chunks e o paralelismo já usados, com o mesmo padrão de fallback da busca de importações por INEP. Se a busca de importações do mês falhar, o puxar falha com erro claro, em vez de devolver um mês silenciosamente incompleto.

**Blocked by:** 01 — Porta de busca; 02 — Constraints novas; 03 — Filtro de data fora do mês.

**Status:** done

- [x] Folha de Registro de mês anterior cuja Escola conectou no mês pedido entra no mês pedido, com a data da conexão
- [x] Escola Conectada sem data de conexão preenchida continua entrando pela previsão de entrega
- [x] OSP com previsão no mês e Escola não Conectada continua entrando pela previsão de entrega
- [x] OSP alcançada pelos dois caminhos gera um único Programado
- [x] Duas conexões do mesmo INEP na mesma data geram uma linha só
- [x] A importação considerada segue sendo a de data de conexão mais recente
- [x] OSP Reprovada alcançada pelo caminho de conexão não entra e sai como skip
- [x] Folha sem contrato kit de Rede Interna alcançada pelo caminho de conexão sai como skip de sem kit; INEP vazio, Escola sem Fase e Escola sem Regional idem
- [x] Falha na busca de importações do mês propaga erro, sem resultado parcial
- [x] Busca de folhas por INEP é paginada, em chunks e paralela, como as demais buscas em lote
- [x] Puxar em test e em live se comportam igual, e a origem gravada continua correta
- [x] O Espelho do Mês continua substituindo só o mês civil pedido
