-- CNPJ deixa de ser obrigatório no Planejado.
-- Rode no SQL Editor se a tabela já existia com o índice antigo.
-- Depois rode docs/sql/aplicar_carga_planejamento.sql para atualizar a RPC.

drop index if exists public.pcp_planejado_chave_idx;

create unique index if not exists pcp_planejado_chave_cnpj_idx
  on public.pcp (data, fase, regional, fornecedor_cnpj)
  where tipo = 'planejado' and fornecedor_cnpj <> '';

create unique index if not exists pcp_planejado_chave_nome_idx
  on public.pcp (data, fase, regional, (coalesce(fornecedor_nome, '')))
  where tipo = 'planejado' and fornecedor_cnpj = '';
