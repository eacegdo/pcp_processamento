-- Marca se o Programado veio da version-test ou da live.
-- Rode no SQL Editor se a tabela pcp já existia. Depois rode
-- docs/sql/aplicar_programado.sql para a RPC gravar o campo.

alter table public.pcp
  add column if not exists origem text null;

alter table public.pcp
  drop constraint if exists pcp_origem_check;

alter table public.pcp
  add constraint pcp_origem_check
  check (origem is null or origem in ('version-test', 'live'));
