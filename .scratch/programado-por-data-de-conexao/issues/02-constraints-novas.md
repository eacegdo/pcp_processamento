# 02 — Constraints de importação por data de conexão e de folha por INEP

**What to build:** duas novas formas de pedir dados ao Bubble, que os tickets seguintes vão usar, disponíveis como funções puras testáveis isoladas junto das constraints existentes:

1. Importações de escola cuja `data_relatorio` cai dentro de um mês civil, com o mês recortado em America/São_Paulo e serializado no mesmo formato já usado pela constraint de OSPs do mês.
2. Folhas de Registro cujo INEP está numa lista dada. O INEP consultado é o **da folha**, não o da escola — é esse o INEP que vira Registro PCP.

Nada no puxar muda de comportamento neste ticket; é a peça de consulta que o caminho por conexão precisa.

**Blocked by:** None — can start immediately (paralelo a 01).

**Status:** done

- [x] Constraint de faixa de `data_relatorio` cobre o mês civil inteiro, do dia 1 ao último dia, no fuso America/São_Paulo
- [x] O formato serializado é o mesmo já usado pela constraint de OSPs do mês (mesma convenção de limites e de data)
- [x] Constraint de folhas por lista de INEP usa a chave de INEP da folha
- [x] Lista de INEPs vazia não produz uma consulta que devolveria a coleção inteira
- [x] Ambas testadas como funções puras, no estilo dos testes de constraint já existentes
