# 06 — Regressão ponta a ponta do puxar por conexão

**What to build:** a rota de puxar o Programado do mês fica coberta ponta a ponta para o caso novo: um mês cujo único item existe porque a Escola conectou nele, com a Folha de Registro vindo de um mês anterior. O contrato da rota, o JSON do Programado e os testes existentes seguem intactos.

**Blocked by:** 04 — Puxar também pelas conexões do mês.

**Status:** done

- [x] Existe um caso ponta a ponta em que o item do mês só existe pelo caminho de conexão, e ele é gravado com a data da conexão
- [x] Os testes de contrato já existentes da rota continuam passando sem alteração
- [x] O corpo da requisição e a escolha entre test e live seguem se comportando como hoje
