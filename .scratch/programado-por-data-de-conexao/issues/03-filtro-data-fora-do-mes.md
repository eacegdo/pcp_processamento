# 03 — Item com data fora do mês vira skip

**What to build:** o mês passa a ser definido pela data que de fato vai ser gravada no Registro PCP, e não pela previsão de entrega da OSP. Todo item montado cuja Data do Programado (data de conexão quando a Escola está Conectada e essa data existe, senão previsão de entrega) cai fora do mês pedido deixa de ser gravado e aparece na lista de skips com o motivo de data fora do mês.

Efeito visível já sem o caminho novo: uma OSP com previsão no mês pedido, mas cuja Escola conectou num mês anterior, deixa de ser gravada no mês pedido — hoje ela entra com data de outro mês, sujando o Espelho do Mês. O motivo de skip já declarado e não usado passa a valer, com texto que fala de data do Programado, não de previsão de entrega.

Consequência esperada e desejada: puxar um mês depois desta mudança pode remover dele Registros que hoje estão lá com data de outro mês. Vale puxar em seguida o mês da conexão para reancorar esses Registros.

**Blocked by:** 01 — Porta de busca no Bubble e montagem do mês sobre ela.

**Status:** done

- [x] Item cuja Data do Programado cai fora do mês pedido não entra no resultado
- [x] Esse item aparece nos skips com motivo de data fora do mês, identificando OSP, folha e INEP
- [x] O texto do motivo fala de data do Programado (não de previsão de entrega)
- [x] O filtro vale para qualquer item, independente de como a OSP foi alcançada
- [x] Item cuja Data do Programado cai dentro do mês segue entrando, com a mesma data de hoje
- [x] Teste de comportamento: OSP com previsão no mês e conexão em mês anterior não entra e sai como skip de data fora do mês
