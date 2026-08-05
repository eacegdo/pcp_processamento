# OCE Processamento — Resources

## Knowledge

- [Repo: `CONTEXT.md`](CONTEXT.md)
  Glossário de domínio do projeto (Escola, INEP, Situação OCE, Lote OCE, Job). Use for: linguagem ao explicar ao time.
- [Repo: `README.md`](README.md)
  Como subir o serviço, CSV esperado, SQLs e colunas tocadas. Use for: checklist de carga e go-live.
- [Repo: `.scratch/aplicacao-lote-oce/spec.md`](.scratch/aplicacao-lote-oce/spec.md)
  Spec completa (user stories, decisões, fora de escopo). Use for: “por que é assim” e limites do sistema.
- [Go: `net/http` package](https://pkg.go.dev/net/http)
  Documentação oficial do servidor HTTP e `ServeMux`. Use for: endpoint `POST /v1/lotes`.
- [Go Blog: Routing Enhancements (Go 1.22)](https://go.dev/blog/routing-enhancements)
  Rotas com método no pattern (`POST /v1/lotes`). Use for: como o mux escolhe o handler.
- [PostgREST: Functions as RPC](https://docs.postgrest.org/en/v13/references/api/functions.html)
  Como funções Postgres viram `POST /rpc/...`. Use for: `aplicar_situacao_oce_lote`.
- [Supabase: REST API (PostgREST)](https://supabase.com/docs/guides/api)
  Como o projeto fala com tabelas (`oce_job`, `escola`) via REST. Use for: adapters em `internal/supabase`.

## Wisdom (Communities)

- [r/golang](https://www.reddit.com/r/golang/)
  Discussão de padrões HTTP/worker em Go. Use for: dúvidas de desenho além deste repo.
- Time interno (Bubble + Supabase)
  Quem opera a carga no dia a dia. Use for: validar se a explicação “fecha” com o fluxo real do Bubble.

## Gaps

- Ainda não há runbook oficial de incidentes (Job `failed`, reenvio). Pode virar aula/referência depois da carga feliz.
