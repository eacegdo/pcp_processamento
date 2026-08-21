package bubble

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	TypeFolhaOSP         = "fr_osp"
	TypeEscolas          = "escolas"
	TypeFornecedor       = "fornecedor_eace"
	TypeOSP              = "osp"
	TypeContrato         = "contrato_taxa_instalacao"
	TypeImportacaoEscola = "importação_escola"
	DefaultPageSize      = 100
	BaseURLVersionTest   = "https://eace.org.br/version-test/api/1.1"
)

// Client talks to the Bubble Data API (/obj/{type}).
type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

func NewClient(baseURL, token string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		BaseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		Token:      strings.TrimSpace(token),
		HTTPClient: httpClient,
	}
}

// ErrLiveIndisponivel is returned when the base URL is Bubble live (osp still 404 there).
var ErrLiveIndisponivel = fmt.Errorf("Data API live ainda não: use %s", BaseURLVersionTest)

func RecusaLive(baseURL string) error {
	u := strings.ToLower(strings.TrimSpace(baseURL))
	if u == "" {
		return nil
	}
	if strings.Contains(u, "eace.org.br") && !strings.Contains(u, "version-test") {
		return ErrLiveIndisponivel
	}
	return nil
}

func (c *Client) List(typeName string, limit, cursor int) (Page, error) {
	return c.ListConstrained(typeName, limit, cursor, "")
}

func (c *Client) ListConstrained(typeName string, limit, cursor int, constraintsJSON string) (Page, error) {
	if limit <= 0 || limit > DefaultPageSize {
		limit = DefaultPageSize
	}
	if cursor < 0 {
		cursor = 0
	}
	u, err := url.Parse(c.objURL(typeName, ""))
	if err != nil {
		return Page{}, err
	}
	q := u.Query()
	q.Set("limit", strconv.Itoa(limit))
	q.Set("cursor", strconv.Itoa(cursor))
	if strings.TrimSpace(constraintsJSON) != "" {
		q.Set("constraints", constraintsJSON)
	}
	u.RawQuery = q.Encode()

	body, err := c.doGET(u.String())
	if err != nil {
		return Page{}, err
	}
	var env Envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return Page{}, fmt.Errorf("bubble %s: json: %w", typeName, err)
	}
	return env.Response, nil
}

func (c *Client) ListFolhasOSP(limit, cursor int) ([]FolhaOSP, Page, error) {
	page, err := c.List(TypeFolhaOSP, limit, cursor)
	if err != nil {
		return nil, Page{}, err
	}
	var rows []FolhaOSP
	if err := json.Unmarshal(page.Results, &rows); err != nil {
		return nil, page, fmt.Errorf("decode fr_osp: %w", err)
	}
	return rows, page, nil
}

func (c *Client) ListOSPs(limit, cursor int, constraintsJSON string) ([]OSP, Page, error) {
	page, err := c.ListConstrained(TypeOSP, limit, cursor, constraintsJSON)
	if err != nil {
		return nil, Page{}, err
	}
	var rows []OSP
	if err := json.Unmarshal(page.Results, &rows); err != nil {
		return nil, page, fmt.Errorf("decode osp: %w", err)
	}
	return rows, page, nil
}

func (c *Client) ListImportacoesEscola(limit, cursor int, constraintsJSON string) ([]ImportacaoEscola, Page, error) {
	page, err := c.ListConstrained(TypeImportacaoEscola, limit, cursor, constraintsJSON)
	if err != nil {
		return nil, Page{}, err
	}
	var rows []ImportacaoEscola
	if err := json.Unmarshal(page.Results, &rows); err != nil {
		return nil, page, fmt.Errorf("decode importação_escola: %w", err)
	}
	return rows, page, nil
}

func ConstraintsINEP(inep string) string {
	b, _ := json.Marshal([]map[string]string{
		{"key": "inep", "constraint_type": "equals", "value": strings.TrimSpace(inep)},
	})
	return string(b)
}

func (c *Client) GetEscola(id string) (Escola, error) {
	var row Escola
	if err := c.getObj(TypeEscolas, id, &row); err != nil {
		return Escola{}, err
	}
	return row, nil
}

func (c *Client) GetFolhaOSP(id string) (FolhaOSP, error) {
	var row FolhaOSP
	if err := c.getObj(TypeFolhaOSP, id, &row); err != nil {
		return FolhaOSP{}, err
	}
	return row, nil
}

func (c *Client) GetContrato(id string) (ContratoInstalacao, error) {
	var row ContratoInstalacao
	if err := c.getObj(TypeContrato, id, &row); err != nil {
		return ContratoInstalacao{}, err
	}
	return row, nil
}

func (c *Client) objURL(typeName, id string) string {
	p := strings.TrimRight(c.BaseURL, "/") + "/obj/" + url.PathEscape(typeName)
	if id != "" {
		p += "/" + url.PathEscape(id)
	}
	return p
}

func (c *Client) getObj(typeName, id string, dest any) error {
	body, err := c.doGET(c.objURL(typeName, id))
	if err != nil {
		return err
	}
	var env struct {
		Response json.RawMessage `json:"response"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("bubble %s/%s: json: %w", typeName, id, err)
	}
	if err := json.Unmarshal(env.Response, dest); err != nil {
		return fmt.Errorf("bubble %s/%s: decode: %w", typeName, id, err)
	}
	return nil
}

func (c *Client) doGET(rawURL string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bubble GET %s: HTTP %d: %s", rawURL, resp.StatusCode, truncate(body, 300))
	}
	return body, nil
}

func truncate(b []byte, n int) string {
	s := string(b)
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
