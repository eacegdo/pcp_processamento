-- Programado: identidade Data + INEP. Espelho do Mês: grava a carga e
-- remove Programado daquele mês cuja chave não veio.
-- Rode no SQL Editor (não dropa Planejado).

create unique index if not exists pcp_programado_chave_idx
  on public.pcp (data, inep)
  where tipo = 'programado';

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
      provisoria boolean,
      origem text
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
      provisoria = r.provisoria,
      origem = r.origem
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
        provisoria,
        origem
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
        r.provisoria,
        r.origem
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
