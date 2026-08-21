package main

import (
	"bytes"
	"flag"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/wellington/pcp_processamento/internal/bubble"
	"github.com/wellington/pcp_processamento/internal/config"
	"github.com/wellington/pcp_processamento/internal/domain"
	"github.com/wellington/pcp_processamento/internal/programado"
	"github.com/wellington/pcp_processamento/internal/supabase"
	"github.com/wellington/pcp_processamento/internal/worker"
)

func main() {
	_ = godotenv.Load()

	mesStr := flag.String("mes", "", "mês civil YYYY-MM (padrão: mês atual em America/Sao_Paulo)")
	env := flag.String("env", "test", "ambiente Bubble: test (version-test) ou live")
	out := flag.String("o", "programado.json", "arquivo JSON de saída")
	somenteJSON := flag.Bool("somente-json", false, "não grava no Supabase")
	flag.Parse()

	base, err := bubble.BaseDoAmbiente(*env)
	if err != nil {
		log.Fatal(err)
	}
	token := tokenDoAmbiente(*env)
	if token == "" {
		if live(*env) {
			log.Fatal("BUBBLE_API_TOKEN_LIVE ou BUBBLE_API_TOKEN é obrigatório para -env live")
		}
		log.Fatal("BUBBLE_API_TOKEN é obrigatório (coloque no .env)")
	}

	mes, err := bubble.MesCivil(*mesStr)
	if err != nil {
		log.Fatalf("mes: %v", err)
	}

	log.Printf("puxando programado %04d-%02d de %s (origem=%s)", mes.Year(), mes.Month(), base, bubble.OrigemDaBase(base))
	c := bubble.NewClient(base, token, nil)
	got, err := c.PuxarMes(mes)
	if err != nil {
		log.Fatalf("puxar: %v", err)
	}
	for _, s := range got.Skips {
		log.Printf("skip folha=%s inep=%s: %s", s.FolhaID, s.INEP, s.Motivo)
	}

	raw, err := bubble.EncodeProgramadoJSON(got.Itens)
	if err != nil {
		log.Fatalf("json: %v", err)
	}
	if err := os.WriteFile(*out, append(raw, '\n'), 0o644); err != nil {
		log.Fatalf("gravar %s: %v", *out, err)
	}
	log.Printf("gravou %d folhas em %s (%d skips)", len(got.Itens), *out, len(got.Skips))

	if *somenteJSON {
		return
	}

	items, err := programado.ParseJSON(bytes.NewReader(raw))
	if err != nil {
		log.Fatalf("nenhum programado válido para o Supabase: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	jobs := supabase.NewJobStore(cfg.SupabaseURL, cfg.SupabaseServiceRoleKey, nil)
	pcp := supabase.NewPcpStore(cfg.SupabaseURL, cfg.SupabaseServiceRoleKey, nil)
	w := worker.New(jobs, pcp, worker.Config{
		BatchSize:  cfg.BatchSize,
		MaxRetries: cfg.BatchMaxRetries,
	})

	fileName := "puxar-" + mes.Format("2006-01") + ".json"
	job, err := jobs.Create(len(items), domain.TipoProgramado, fileName, items)
	if err != nil {
		log.Fatalf("job: %v", err)
	}
	log.Printf("job %s enfileirado (%d itens); aplicando no Supabase", job.ID, len(items))
	for w.ProcessNext() {
	}
	done, ok := jobs.Get(job.ID)
	if !ok {
		log.Fatalf("job %s sumiu", job.ID)
	}
	if done.Status != "success" {
		log.Fatalf("job %s: %s %s", done.ID, done.Status, done.ErrorMessage)
	}
	log.Printf("job %s success — programado gravado em pcp", done.ID)
}

func live(env string) bool {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "live", "prod", "produção", "producao":
		return true
	default:
		return false
	}
}

func tokenDoAmbiente(env string) string {
	if live(env) {
		if v := strings.TrimSpace(os.Getenv("BUBBLE_API_TOKEN_LIVE")); v != "" {
			return v
		}
	}
	return strings.TrimSpace(os.Getenv("BUBBLE_API_TOKEN"))
}
