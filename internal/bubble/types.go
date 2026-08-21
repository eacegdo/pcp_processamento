package bubble

import (
	"encoding/json"
	"strings"
)

// Envelope is the Data API list response.
type Envelope struct {
	Response Page `json:"response"`
}

type Page struct {
	Cursor    int             `json:"cursor"`
	Results   json.RawMessage `json:"results"`
	Count     int             `json:"count"`
	Remaining int             `json:"remaining"`
}

// FolhaOSP is custom.fr_osp. Lives under OSP.FR; lista de contratos aponta para contrato_taxa_instalacao.
type FolhaOSP struct {
	ID                       string   `json:"_id"`
	CreatedDate              string   `json:"Created Date"`
	ModifiedDate             string   `json:"Modified Date"`
	INEP                     string   `json:"INEP"`
	UF                       string   `json:"UF"`
	Municipio                string   `json:"Município"`
	EscolaID                 string   `json:"Escola"`
	OSPID                    string   `json:"OSP"`
	Status                   string   `json:"status"`
	Tipo                     string   `json:"Tipo"`
	DataPrevistaPagamento    string   `json:"data prevista para pagamento"`
	ValidadoFinanceiro       bool     `json:"Validado financeiro"`
	ValidadoRegional         bool     `json:"Validado Regional"`
	ValidadoSalaTecnica      bool     `json:"Validado sala técnica"`
	Precessado               bool     `json:"Precessado"`
	EnviadoParaSAP           bool     `json:"Enviado para SAP"`
	ValorTotal               float64  `json:"Valor total"`
	NotaFiscalNumero         string   `json:"NotaFiscal_numero"`
	NumeroContratoSAP        int      `json:"numero_contrato_sap"`
	ListaContratosInstalacao []string `json:"lista de contratos_instalação"`
}

// OSP is custom.osp (version-test Data API).
type OSP struct {
	ID                      string   `json:"_id"`
	CodUnico                string   `json:"Cod_unico"`
	Status                  string   `json:"status"`
	PrevisaoEntrega         string   `json:"Previsão de entrega"`
	FornecedorID            string   `json:"Fornecedor"`
	Regional                FlexText `json:"Regional"`
	FRs                     []string `json:"FR"`
	ContratosTaxaInstalacao []string `json:"contrato_taxa_instalação"`
	FREsperadas             int      `json:"FR esperadas"`
	OSnum                   *float64 `json:"OSnum"`
}

// ImportacaoEscola is custom.importação_escola. Data API slug keeps the accent.
type ImportacaoEscola struct {
	ID            string `json:"_id"`
	INEP          string `json:"inep"`
	Data          string `json:"data"`
	DataRelatorio string `json:"data_relatorio"`
	Alteracao     string `json:"Alteração"`
	Area          string `json:"Área"`
}

// ContratoInstalacao is custom.contrato_taxa_instalacao.
type ContratoInstalacao struct {
	ID         string `json:"_id"`
	Descricao  string `json:"Descrição"`
	TipoDeObra string `json:"Tipo de obra"`
	Status     string `json:"status"`
	EscolaID   string `json:"escola"`
	UF         string `json:"UF"`
	Quantidade int    `json:"Quantidade"`
}

// Escola is custom.escolas. Regional here is the display name (Norte), not the sigla.
type Escola struct {
	ID               string `json:"_id"`
	INEP             string `json:"INEP"`
	Nome             string `json:"NOME ESCOLA"`
	UF               string `json:"UF"`
	Municipio        string `json:"MUNICIPIO"`
	Fase             string `json:"FASE"`
	Regional         string `json:"Regional"`
	StatusEscola     string `json:"STATUS ESCOLA"`
	StatusGeral      string `json:"Status Geral"`
	FornecedorRI     string `json:"fornecedor_ri"`
	CNPJFornecedorRI string `json:"cnpj_fornecedor_ri"`
	FornecedorRE     string `json:"fornecedor_re"`
	CNPJFornecedorRE string `json:"cnpj_fornecedor_re"`
}

// Fornecedor is custom.fornecedor_eace. Cadastro; the school already carries RI name/CNPJ.
type Fornecedor struct {
	ID           string   `json:"_id"`
	CNPJ         string   `json:"CNPJ"`
	NomeFantasia string   `json:"Nome Fantasia"`
	RazaoSocial  string   `json:"Razão social"`
	Ativo        bool     `json:"Ativo"`
	TipoEmpresa  []string `json:"Tipo de empresa"`
}

// FlexText accepts a JSON string or array of strings (Bubble list vs text).
type FlexText string

func (f *FlexText) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "" || s == "null" {
		*f = ""
		return nil
	}
	if s[0] == '"' {
		var v string
		if err := json.Unmarshal(b, &v); err != nil {
			return err
		}
		*f = FlexText(strings.TrimSpace(v))
		return nil
	}
	if s[0] == '[' {
		var vs []string
		if err := json.Unmarshal(b, &vs); err != nil {
			return err
		}
		for _, v := range vs {
			v = strings.TrimSpace(v)
			if v != "" {
				*f = FlexText(v)
				return nil
			}
		}
		*f = ""
		return nil
	}
	var v string
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*f = FlexText(v)
	return nil
}
