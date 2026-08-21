-- Esconde Programado com origem = version-test de leitura e de funções
-- SECURITY INVOKER. Rode no SQL Editor depois de pcp_programado_origem.sql.
--
-- O que some para anon, authenticated, Realtime e views/RPCs INVOKER:
--   qualquer linha de pcp com origem = 'version-test'
-- Planejado (origem vazia) e Programado live continuam visíveis.
--
-- O que ainda vê version-test (limitação do Postgres, não dá para ligar RLS nisso):
--   SQL Editor como postgres (superuser)
--   service_role (a API Go / CLI) — precisa gravar o puxar -env test
--   funções SECURITY DEFINER donas de postgres, se fizerem SELECT na tabela pcp
--
-- Por isso existe a view pcp_visivel: o WHERE vale até para postgres e
-- service_role. Funções e o Bubble devem ler pcp_visivel, não pcp.
-- As RPCs aplicar_* devolvem void e continuam na tabela (chave única).

create or replace view public.pcp_visivel
with (security_invoker = true, security_barrier = true)
as
select *
from public.pcp
where origem is distinct from 'version-test';

comment on view public.pcp_visivel is
  'pcp sem origem version-test. Use esta view no Bubble e em funções SQL.';

grant select on public.pcp_visivel to anon, authenticated, service_role;

alter table public.pcp enable row level security;
alter table public.pcp force row level security;

drop policy if exists pcp_permite_acesso on public.pcp;
drop policy if exists pcp_nunca_version_test on public.pcp;

-- Sem uma política PERMISSIVE o Postgres nega tudo. Esta libera;
-- a RESTRICTIVE abaixo corta version-test mesmo se alguém criar
-- outra política "allow all".
create policy pcp_permite_acesso
  on public.pcp
  as permissive
  for all
  to public
  using (true)
  with check (true);

create policy pcp_nunca_version_test
  on public.pcp
  as restrictive
  for all
  to public
  using (origem is distinct from 'version-test')
  with check (origem is distinct from 'version-test');
