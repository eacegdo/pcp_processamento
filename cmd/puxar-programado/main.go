package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/wellington/pcp_processamento/internal/bubble"
)

func main() {
	_ = godotenv.Load()

	mesStr := flag.String("mes", "", "mês civil YYYY-MM (padrão: mês atual em America/Sao_Paulo)")
	out := flag.String("o", "programado.json", "arquivo JSON de saída")
	flag.Parse()

	base := strings.TrimSpace(os.Getenv("BUBBLE_BASE_URL"))
	if base == "" {
		base = bubble.BaseURLVersionTest
	}
	if err := bubble.RecusaLive(base); err != nil {
		log.Fatal(err)
	}
	token := strings.TrimSpace(os.Getenv("BUBBLE_API_TOKEN"))
	if token == "" {
		log.Fatal("BUBBLE_API_TOKEN é obrigatório (coloque no .env)")
	}

	mes, err := parseMes(*mesStr)
	if err != nil {
		log.Fatalf("mes: %v", err)
	}

	log.Printf("puxando programado %04d-%02d de %s", mes.Year(), mes.Month(), base)
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
	log.Printf("gravou %d itens em %s (%d skips)", len(got.Itens), *out, len(got.Skips))
}

func parseMes(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		loc, err := time.LoadLocation("America/Sao_Paulo")
		if err != nil {
			loc = time.UTC
		}
		now := time.Now().In(loc)
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC), nil
	}
	t, err := time.Parse("2006-01", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("use YYYY-MM, veio %q", s)
	}
	return t, nil
}
