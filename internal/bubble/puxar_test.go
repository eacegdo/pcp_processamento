package bubble_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wellington/pcp_processamento/internal/bubble"
	"github.com/wellington/pcp_processamento/internal/domain"
	"github.com/wellington/pcp_processamento/internal/programado"
)

func TestPuxarMesMontaJSONDoProgramado(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/obj/osp":
			_, _ = io.WriteString(w, `{
			  "response": {"cursor":0,"remaining":0,"results":[{
			    "_id":"osp1","status":"Nota Fiscal","OSnum":13,
			    "Previsão de entrega":"2026-08-08T17:44:00.000Z",
			    "FR":["fr1"]
			  }]}
			}`)
		case r.URL.Path == "/obj/fr_osp":
			_, _ = io.WriteString(w, `{
			  "response": {"cursor":0,"remaining":0,"results":[{
			    "_id":"fr1","INEP":"15026868","UF":"PA",
			    "Escola":"esc1","OSP":"osp1",
			    "lista de contratos_instalação":["c1"]
			  }]}
			}`)
		case r.URL.Path == "/obj/fr_osp/fr1":
			_, _ = io.WriteString(w, `{
			  "response": {
			    "_id":"fr1","INEP":"15026868","UF":"PA",
			    "Escola":"esc1","OSP":"osp1",
			    "lista de contratos_instalação":["c1"]
			  }
			}`)
		case r.URL.Path == "/obj/contrato_taxa_instalacao":
			_, _ = io.WriteString(w, `{
			  "response": {"cursor":0,"remaining":0,"results":[{
			    "_id":"c1","Descrição":"Kit Cobertura Wi-Fi",
			    "Tipo de obra":"4-IMPLANTAÇÃO_DE_REDE_INTERNA"
			  }]}
			}`)
		case strings.HasSuffix(r.URL.Path, "/contrato_taxa_instalacao/c1"):
			_, _ = io.WriteString(w, `{
			  "response": {
			    "_id":"c1","Descrição":"Kit Cobertura Wi-Fi",
			    "Tipo de obra":"4-IMPLANTAÇÃO_DE_REDE_INTERNA"
			  }
			}`)
		case r.URL.Path == "/obj/escolas":
			_, _ = io.WriteString(w, `{
			  "response": {"cursor":0,"remaining":0,"results":[{
			    "_id":"esc1","INEP":"15026868","UF":"PA","FASE":"3",
			    "Regional":"Norte","Status Geral":"Conectada",
			    "fornecedor_ri":"Q13 TECNOLOGIA",
			    "cnpj_fornecedor_ri":"30.161.238/0001-60"
			  }]}
			}`)
		case r.URL.Path == "/obj/escolas/esc1":
			_, _ = io.WriteString(w, `{
			  "response": {
			    "_id":"esc1","INEP":"15026868","UF":"PA","FASE":"3",
			    "Regional":"Norte","Status Geral":"Conectada",
			    "fornecedor_ri":"Q13 TECNOLOGIA",
			    "cnpj_fornecedor_ri":"30.161.238/0001-60"
			  }
			}`)
		case strings.Contains(r.URL.Path, "import"):
			_, _ = io.WriteString(w, `{
			  "response": {"cursor":0,"remaining":0,"results":[{
			    "inep":"15026868","data_relatorio":"2026-08-10T17:55:00.000Z"
			  }]}
			}`)
		default:
			t.Errorf("path inesperado %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	c := bubble.NewClient(srv.URL, "tok", srv.Client())
	got, err := c.PuxarMes(time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Itens) != 1 {
		t.Fatalf("itens=%d skips=%+v", len(got.Itens), got.Skips)
	}
	if got.Itens[0].Data.Format("2006-01-02") != "2026-08-10" {
		t.Fatalf("data = %s", got.Itens[0].Data)
	}
	if got.Itens[0].Provisoria == nil || *got.Itens[0].Provisoria {
		t.Fatal("provisoria")
	}

	raw, err := bubble.EncodeProgramadoJSON(got.Itens)
	if err != nil {
		t.Fatal(err)
	}
	items, err := programado.ParseJSON(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].INEP != "15026868" || items[0].Fase != "3" || items[0].Regional != "NO" {
		t.Fatalf("%+v raw=%s", items, raw)
	}
	if items[0].Data.Format("2006-01-02") != "2026-08-10" {
		t.Fatalf("parse data = %s", items[0].Data)
	}
}

func TestPuxarMesListaFolhasEmLote(t *testing.T) {
	var getsPorID int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if len(strings.Split(strings.Trim(r.URL.Path, "/"), "/")) >= 3 {
			getsPorID++
		}
		switch {
		case r.URL.Path == "/obj/osp":
			_, _ = io.WriteString(w, `{"response":{"cursor":0,"remaining":0,"results":[
			  {"_id":"osp1","status":"NF","OSnum":1,"Previsão de entrega":"2026-08-08T17:44:00.000Z","FR":["fr1","fr2"]}
			]}}`)
		case r.URL.Path == "/obj/fr_osp":
			_, _ = io.WriteString(w, `{"response":{"cursor":0,"remaining":0,"results":[
			  {"_id":"fr1","INEP":"1","Escola":"e1","lista de contratos_instalação":["c1"]},
			  {"_id":"fr2","INEP":"2","Escola":"e1","lista de contratos_instalação":["c1"]}
			]}}`)
		case r.URL.Path == "/obj/contrato_taxa_instalacao":
			_, _ = io.WriteString(w, `{"response":{"cursor":0,"remaining":0,"results":[
			  {"_id":"c1","Descrição":"Kit RI","Tipo de obra":"4-IMPLANTAÇÃO_DE_REDE_INTERNA"}
			]}}`)
		case r.URL.Path == "/obj/escolas":
			_, _ = io.WriteString(w, `{"response":{"cursor":0,"remaining":0,"results":[
			  {"_id":"e1","INEP":"1","UF":"PA","FASE":"3","Regional":"Norte","Status Geral":"Em planejamento","fornecedor_ri":"Q13"}
			]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	c := bubble.NewClient(srv.URL, "tok", srv.Client())
	got, err := c.PuxarMes(time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Itens) != 2 {
		t.Fatalf("itens=%d skips=%+v", len(got.Itens), got.Skips)
	}
	if getsPorID != 0 {
		t.Fatalf("gets por id = %d, quer lote", getsPorID)
	}
}

func TestEncodeProgramadoJSONFormatoPCP(t *testing.T) {
	prov := true
	raw, err := bubble.EncodeProgramadoJSON([]domain.ItemCarga{{
		Tipo:       domain.TipoProgramado,
		Data:       time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC),
		Fase:       "3",
		Regional:   "NO",
		UF:         "PA",
		INEP:       "15026868",
		Quantidade: 1,
		Provisoria: &prov,
		Origem:     domain.OrigemVersionTest,
	}})
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("%s", raw)
	}
	if rows[0]["data"] != "08/08/2026" || rows[0]["inep"] != "15026868" || rows[0]["quantidade"] != float64(1) {
		t.Fatalf("%v", rows[0])
	}
	if rows[0]["provisoria"] != true {
		t.Fatalf("provisoria = %v", rows[0]["provisoria"])
	}
	if rows[0]["origem"] != "version-test" {
		t.Fatalf("origem = %v", rows[0]["origem"])
	}
}
