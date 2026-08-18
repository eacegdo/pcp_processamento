-- Job de Aplicação da Carga de Planejamento
-- Bubble consulta esta tabela (API Connector) para progresso da fila.

create table public.pcp_job (
  id uuid primary key default gen_random_uuid(),
  status text not null default 'queued'
    check (status in ('queued', 'running', 'success', 'failed')),
  total integer not null default 0 check (total >= 0),
  processadas integer not null default 0 check (processadas >= 0),
  restantes integer generated always as (total - processadas) stored,
  error_message text null,
  file_name text null,
  created_at timestamp with time zone not null default now(),
  updated_at timestamp with time zone not null default now(),
  constraint pcp_job_processadas_lte_total check (processadas <= total)
);

create index if not exists pcp_job_status_idx
  on public.pcp_job using btree (status);

create index if not exists pcp_job_created_at_idx
  on public.pcp_job using btree (created_at desc);

create trigger trg_set_updated_at_pcp_job
before update on public.pcp_job
for each row
execute function set_updated_at();
