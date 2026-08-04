-- Aplica Situação OCE em lote: UPDATE só nas Escolas cujo INEP já existe.
-- Não insere linhas novas. Chamada via PostgREST: POST /rest/v1/rpc/aplicar_situacao_oce_lote

create or replace function public.aplicar_situacao_oce_lote(itens jsonb)
returns void
language sql
security definer
set search_path = public
as $$
  update public.escola as e
  set
    oce_tipo_acesso = x.oce_tipo_acesso,
    oce_status      = x.oce_status,
    oce_pendencia   = x.oce_pendencia
  from (
    select *
    from jsonb_to_recordset(coalesce(itens, '[]'::jsonb)) as t(
      inep text,
      oce_tipo_acesso text,
      oce_status text,
      oce_pendencia text
    )
  ) as x
  where e.inep::text = x.inep;
$$;

revoke all on function public.aplicar_situacao_oce_lote(jsonb) from public;
grant execute on function public.aplicar_situacao_oce_lote(jsonb) to service_role;
