-- Coleção de Registro PCP (Planejado e Programado).
-- Unique parcial Planejado: com CNPJ, data + fase + regional + CNPJ;
-- sem CNPJ, data + fase + regional + nome do fornecedor.
-- Unique parcial Programado: data + INEP.

create or replace function public.set_updated_at()
returns trigger
language plpgsql
as $$
begin
  new.updated_at = now();
  return new;
end;
$$;

create table public.pcp (
  id uuid primary key default gen_random_uuid(),
  tipo text not null check (tipo in ('planejado', 'programado')),
  data date not null,
  fase text not null,
  regional text not null,
  regional_nome text null,
  uf text null,
  inep text null,
  fornecedor_nome text null,
  fornecedor_cnpj text not null,
  quantidade integer not null check (quantidade >= 0),
  provisoria boolean null,
  origem text null check (origem is null or origem in ('version-test', 'live')),
  created_at timestamp with time zone not null default now(),
  updated_at timestamp with time zone not null default now()
);

create unique index if not exists pcp_planejado_chave_cnpj_idx
  on public.pcp (data, fase, regional, fornecedor_cnpj)
  where tipo = 'planejado' and fornecedor_cnpj <> '';

create unique index if not exists pcp_planejado_chave_nome_idx
  on public.pcp (data, fase, regional, (coalesce(fornecedor_nome, '')))
  where tipo = 'planejado' and fornecedor_cnpj = '';

create unique index if not exists pcp_programado_chave_idx
  on public.pcp (data, inep)
  where tipo = 'programado';

create trigger trg_set_updated_at_pcp
before update on public.pcp
for each row
execute function set_updated_at();
