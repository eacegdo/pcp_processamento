-- Aplica a Carga de Planejamento.
-- Com CNPJ: chave data + fase + regional + CNPJ.
-- Sem CNPJ: chave data + fase + regional + nome do fornecedor.
-- Chave existente: atualiza quantidade, nome do fornecedor e regional_nome (inclusive para zero).
-- Chave nova e quantidade > 0: insere tipo planejado.
-- Chave nova e quantidade = 0: não insere.
-- Chamada via PostgREST: POST /rest/v1/rpc/aplicar_carga_planejamento

create or replace function public.aplicar_carga_planejamento(itens jsonb)
returns void
language plpgsql
security definer
set search_path = public
as $$
declare
  r record;
  cnpj_chave text;
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

    cnpj_chave := coalesce(r.fornecedor_cnpj, '');

    if cnpj_chave <> '' then
      update public.pcp as p
      set
        quantidade = r.quantidade,
        fornecedor_nome = r.fornecedor_nome,
        regional_nome = r.regional_nome
      where p.tipo = 'planejado'
        and p.data = r.data
        and p.fase = r.fase
        and p.regional = r.regional
        and p.fornecedor_cnpj = cnpj_chave;
    else
      update public.pcp as p
      set
        quantidade = r.quantidade,
        fornecedor_nome = r.fornecedor_nome,
        regional_nome = r.regional_nome
      where p.tipo = 'planejado'
        and p.data = r.data
        and p.fase = r.fase
        and p.regional = r.regional
        and p.fornecedor_cnpj = ''
        and coalesce(p.fornecedor_nome, '') = coalesce(r.fornecedor_nome, '');
    end if;

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
        cnpj_chave,
        r.quantidade
      );
    end if;
  end loop;
end;
$$;

revoke all on function public.aplicar_carga_planejamento(jsonb) from public;
grant execute on function public.aplicar_carga_planejamento(jsonb) to service_role;
