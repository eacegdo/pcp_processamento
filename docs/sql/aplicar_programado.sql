-- Programado: identidade Data + INEP.
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
begin
  for r in
    select *
    from jsonb_to_recordset(coalesce(itens, '[]'::jsonb)) as t(
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
end;
$$;

revoke all on function public.aplicar_programado(jsonb) from public;
grant execute on function public.aplicar_programado(jsonb) to service_role;
