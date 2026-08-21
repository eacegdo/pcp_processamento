package bubble_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wellington/pcp_processamento/internal/bubble"
)

func TestListFolhasOSPEnviaBearerEDecodifica(t *testing.T) {
	page := testdata(t, "fr_osp_page.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/obj/fr_osp" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("auth = %q", r.Header.Get("Authorization"))
		}
		if r.URL.Query().Get("limit") != "100" {
			t.Fatalf("limit = %s", r.URL.Query().Get("limit"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(page)
	}))
	t.Cleanup(srv.Close)

	c := bubble.NewClient(srv.URL, "test-token", srv.Client())
	rows, p, err := c.ListFolhasOSP(100, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || p.Remaining != 27 {
		t.Fatalf("rows=%d remaining=%d", len(rows), p.Remaining)
	}
	if rows[0].INEP != "15026868" {
		t.Fatalf("%+v", rows[0])
	}
}

func TestListSemTokenNaoMandaAuthorization(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Fatalf("auth inesperado: %q", r.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(w).Encode(bubble.Envelope{Response: bubble.Page{Results: []byte("[]")}})
	}))
	t.Cleanup(srv.Close)
	c := bubble.NewClient(srv.URL, "", srv.Client())
	if _, _, err := c.ListFolhasOSP(1, 0); err != nil {
		t.Fatal(err)
	}
}

func TestRecusaLive(t *testing.T) {
	if err := bubble.RecusaLive("https://eace.org.br/api/1.1"); err == nil {
		t.Fatal("live deveria recusar")
	}
	if err := bubble.RecusaLive("https://eace.org.br/version-test/api/1.1"); err != nil {
		t.Fatal(err)
	}
	if err := bubble.RecusaLive("http://127.0.0.1:1234"); err != nil {
		t.Fatal(err)
	}
}
