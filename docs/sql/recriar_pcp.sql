-- Recria a coleção PCP, o Job e a RPC.
-- Apaga dados de public.pcp e public.pcp_job.
-- Não dropa set_updated_at(): outras tabelas do projeto podem usá-la.

drop function if exists public.aplicar_carga_planejamento(jsonb);
drop function if exists public.aplicar_programado(jsonb);

drop table if exists public.pcp cascade;
drop table if exists public.pcp_job cascade;

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
  created_at timestamp with time zone not null default now(),
  updated_at timestamp with time zone not null default now()
);

create unique index pcp_planejado_chave_idx
  on public.pcp (data, fase, regional, fornecedor_cnpj)
  where tipo = 'planejado';

create unique index pcp_programado_chave_idx
  on public.pcp (data, inep)
  where tipo = 'programado';

create trigger trg_set_updated_at_pcp
before update on public.pcp
for each row
execute function set_updated_at();

create table public.pcp_job (
  id uuid primary key default gen_random_uuid(),
  status text not null default 'queued'
    check (status in ('queued', 'running', 'success', 'failed')),
  total integer not null default 0 check (total >= 0),
  processadas integer not null default 0 check (processadas >= 0),
  restantes integer generated always as (total - processadas) stored,
  error_message text null,
  file_name text null,
  tipo text not null default 'planejado'
    check (tipo in ('planejado', 'programado')),
  created_at timestamp with time zone not null default now(),
  updated_at timestamp with time zone not null default now(),
  constraint pcp_job_processadas_lte_total check (processadas <= total)
);

create index pcp_job_status_idx
  on public.pcp_job using btree (status);

create index pcp_job_created_at_idx
  on public.pcp_job using btree (created_at desc);

create index pcp_job_tipo_idx
  on public.pcp_job using btree (tipo);

create trigger trg_set_updated_at_pcp_job
before update on public.pcp_job
for each row
execute function set_updated_at();

create or replace function public.aplicar_carga_planejamento(itens jsonb)
returns void
language plpgsql
security definer
set search_path = public
as $$
declare
  r record;
begin
  for r in
    select *
    from jsonb_to_recordset(coalesce(itens, '[]'::jsonb)) as t(
      data date,
      fase text,
      regional text,
      regional_nome text,
      fornecedor_nome text,
      fornecedor_cnpj text,
      quantidade integer
    )
  loop
    if r.quantidade < 0 then
      continue;
    end if;

    update public.pcp as p
    set
      quantidade = r.quantidade,
      fornecedor_nome = r.fornecedor_nome,
      regional_nome = r.regional_nome
    where p.tipo = 'planejado'
      and p.data = r.data
      and p.fase = r.fase
      and p.regional = r.regional
      and p.fornecedor_cnpj = r.fornecedor_cnpj;

    if found then
      continue;
    end if;

    if r.quantidade > 0 then
      insert into public.pcp (
        tipo,
        data,
        fase,
        regional,
        regional_nome,
        fornecedor_nome,
        fornecedor_cnpj,
        quantidade
      ) values (
        'planejado',
        r.data,
        r.fase,
        r.regional,
        r.regional_nome,
        r.fornecedor_nome,
        r.fornecedor_cnpj,
        r.quantidade
      );
    end if;
  end loop;
end;
$$;

revoke all on function public.aplicar_carga_planejamento(jsonb) from public;
grant execute on function public.aplicar_carga_planejamento(jsonb) to service_role;

create or replace function public.aplicar_programado(itens jsonb)
returns void
language plpgsql
security definer
set search_path = public
as $$
declare
  r record;
  mes_inicio date;
begin
  if itens is null or jsonb_typeof(itens) <> 'array' or jsonb_array_length(itens) = 0 then
    return;
  end if;

  select date_trunc('month', (itens->0->>'data')::date)::date
    into mes_inicio;

  if mes_inicio is null then
    return;
  end if;

  for r in
    select *
    from jsonb_to_recordset(itens) as t(
      data date,
      fase text,
      regional text,
      regional_nome text,
      uf text,
      inep text,
      fornecedor_nome text,
      fornecedor_cnpj text,
      quantidade integer,
      provisoria boolean
    )
  loop
    if r.inep is null or btrim(r.inep) = '' or r.quantidade < 0 then
      continue;
    end if;
    if r.data is null
       or date_trunc('month', r.data)::date is distinct from mes_inicio then
      continue;
    end if;

    update public.pcp as p
    set
      fase = r.fase,
      regional = r.regional,
      regional_nome = r.regional_nome,
      uf = r.uf,
      fornecedor_nome = r.fornecedor_nome,
      fornecedor_cnpj = coalesce(r.fornecedor_cnpj, ''),
      quantidade = r.quantidade,
      provisoria = r.provisoria
    where p.tipo = 'programado'
      and p.data = r.data
      and p.inep = r.inep;

    if found then
      continue;
    end if;

    if r.quantidade > 0 then
      insert into public.pcp (
        tipo,
        data,
        fase,
        regional,
        regional_nome,
        uf,
        inep,
        fornecedor_nome,
        fornecedor_cnpj,
        quantidade,
        provisoria
      ) values (
        'programado',
        r.data,
        r.fase,
        r.regional,
        r.regional_nome,
        r.uf,
        r.inep,
        r.fornecedor_nome,
        coalesce(r.fornecedor_cnpj, ''),
        r.quantidade,
        r.provisoria
      );
    end if;
  end loop;

  delete from public.pcp as p
  where p.tipo = 'programado'
    and p.data >= mes_inicio
    and p.data < (mes_inicio + interval '1 month')::date
    and not exists (
      select 1
      from jsonb_to_recordset(itens) as t(data date, inep text)
      where t.data = p.data
        and t.inep is not null
        and btrim(t.inep) <> ''
        and t.inep = p.inep
    );
end;
$$;

revoke all on function public.aplicar_programado(jsonb) from public;
grant execute on function public.aplicar_programado(jsonb) to service_role;
