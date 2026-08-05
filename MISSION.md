# Mission: Explicar e operar a carga OCE

## Why
Você precisa explicar ao time, como documentação viva, como o serviço de Aplicação de Lote OCE funciona ponta a ponta — e conseguir completar uma carga com sucesso no Supabase/Bubble.

## Success looks like
- Descrever o fluxo da carga (CSV → Job → batches → `escola` / `oce_job`) sem ler o código na hora
- Rodar o serviço, enviar um Lote OCE e ver o Job chegar a `success` com Situação OCE atualizada
- Responder perguntas do time sobre o que cada peça faz e o que não faz (ex.: não cria Escola, sem pause/resume)

## Constraints
- Português
- Começar por um **mapa geral**; depois aprofundar peça por peça
- Conhecimento prévio superficial: Go, Supabase, filas
- Material deve servir como documento explicativo para o time

## Out of scope
- Reescrever o produto ou mudar a arquitetura neste ensino
- Deep dive em Go avançado / Postgres avançado antes do mapa e da carga funcionar
- Pause/resume de Job (não existe no sistema atual)
