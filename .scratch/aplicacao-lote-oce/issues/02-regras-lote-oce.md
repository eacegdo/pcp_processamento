# 02 — Regras completas do Lote OCE no caminho da API

**What to build:** O mesmo fluxo de upload aplica as regras de domínio do Lote OCE de ponta a ponta: delimitadores `,` e `;`, linhas incompletas ignoradas, duplicata de INEP com última ocorrência vencendo, mapeamento `oce_status_final` → Status OCE, INEP inexistente como no-op, e `total` só com linhas válidas após dedupe.

**Blocked by:** 01 — Upload autenticado → Job → Aplicação feliz (fakes)

**Status:** done

- [x] CSV delimitado por `,` e por `;` produz a mesma Aplicação de Lote correta
- [x] Linha com qualquer dos quatro campos vazio é ignorada (não limpa Situação OCE no store)
- [x] Se o mesmo INEP repetir no CSV, prevalece a última ocorrência
- [x] `oce_status_final` do CSV grava como Status OCE (`oce_status`) na Escola
- [x] INEP inexistente no `EscolaStore` não cria Escola e não falha o Job
- [x] `total` do Job conta apenas linhas válidas após filtro e dedupe
- [x] Cobertura na costura HTTP (fakes), sem Supabase
