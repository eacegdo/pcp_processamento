# 01 — Porta de busca no Bubble e montagem do mês sobre ela

**What to build:** puxar o Programado do mês continua funcionando exatamente como hoje — mesmos itens, mesmos skips, mesmo JSON — mas a montagem do mês deixa de conversar direto com o cliente HTTP e passa a depender de uma porta de busca explícita no pacote `bubble`. Prefactor: nenhuma regra de negócio muda aqui, só fica possível montar o mês contra uma fonte de dados falsa em memória, sem levantar servidor HTTP.

A porta expõe o que a montagem precisa: OSPs do mês por previsão, OSPs por IDs, importações de escola com `data_relatorio` no mês, Folhas de Registro por IDs, Folhas de Registro por INEPs, contratos de instalação por IDs, escolas por IDs, importações de escola por INEPs. Os dois últimos métodos novos (OSPs por IDs, Folhas por INEPs) já nascem paginados e em chunks como os demais, ainda que a montagem só passe a usá-los nos tickets seguintes. O cliente real implementa a porta; `PuxarMes` vira um wrapper fino que passa o próprio cliente. Assinatura e semântica de `PuxarMes` para HTTP API e CLI não mudam.

**Blocked by:** None — can start immediately.

**Status:** done

- [x] A montagem do mês é uma função que recebe a porta de busca e o mês, e não depende de HTTP
- [x] O cliente do Bubble satisfaz a porta, incluindo busca de OSPs por IDs e de Folhas de Registro por INEPs
- [x] `PuxarMes` mantém assinatura, itens, skips e origem gravada idênticos aos de hoje
- [x] Existe uma fonte de busca falsa em memória, usada por testes de comportamento da montagem
- [x] Testes de comportamento cobrem, pela fonte falsa, o mês montado hoje: OSP com previsão no mês entra; Escola Conectada com data de conexão no mês grava a data da conexão; skips de OSP Reprovada, sem INEP, sem kit de Rede Interna, sem Fase e sem Regional seguem saindo com o mesmo motivo
- [x] Os testes existentes de puxar (fake HTTP) e de `POST /v1/programado/puxar` continuam passando sem alteração
