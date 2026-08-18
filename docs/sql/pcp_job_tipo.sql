-- Identifica se o Job é de Planejado ou Programado.
-- Rode no SQL Editor (não dropa jobs existentes).
-- Jobs antigos sem tipo viram planejado.

alter table public.pcp_job
  add column if not exists tipo text;

update public.pcp_job
set tipo = 'planejado'
where tipo is null or btrim(tipo) = '';

alter table public.pcp_job
  alter column tipo set default 'planejado';

alter table public.pcp_job
  alter column tipo set not null;

alter table public.pcp_job
  drop constraint if exists pcp_job_tipo_check;

alter table public.pcp_job
  add constraint pcp_job_tipo_check
  check (tipo in ('planejado', 'programado'));

create index if not exists pcp_job_tipo_idx
  on public.pcp_job using btree (tipo);
