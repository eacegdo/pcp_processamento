package config_test

import (
	"testing"

	"github.com/wellington/oce_processamento/internal/config"
)

func TestLoadExigeVariaveisObrigatorias(t *testing.T) {
	t.Setenv("SUPABASE_URL", "")
	t.Setenv("SUPABASE_SERVICE_ROLE_KEY", "")
	t.Setenv("API_KEY", "")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when required env vars are missing")
	}
}

func TestLoadLeEnvComDefaultsOpcionais(t *testing.T) {
	t.Setenv("SUPABASE_URL", "https://exemplo.supabase.co")
	t.Setenv("SUPABASE_SERVICE_ROLE_KEY", "service-role")
	t.Setenv("API_KEY", "api-key")
	t.Setenv("BATCH_SIZE", "")
	t.Setenv("BATCH_MAX_RETRIES", "")
	t.Setenv("HTTP_ADDR", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.SupabaseURL != "https://exemplo.supabase.co" {
		t.Fatalf("SupabaseURL = %q", cfg.SupabaseURL)
	}
	if cfg.SupabaseServiceRoleKey != "service-role" {
		t.Fatalf("SupabaseServiceRoleKey = %q", cfg.SupabaseServiceRoleKey)
	}
	if cfg.APIKey != "api-key" {
		t.Fatalf("APIKey = %q", cfg.APIKey)
	}
	if cfg.BatchSize != 200 {
		t.Fatalf("BatchSize = %d, want 200", cfg.BatchSize)
	}
	if cfg.BatchMaxRetries != 3 {
		t.Fatalf("BatchMaxRetries = %d, want 3", cfg.BatchMaxRetries)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
}

func TestLoadRespeitaOverridesOpcionais(t *testing.T) {
	t.Setenv("SUPABASE_URL", "https://exemplo.supabase.co")
	t.Setenv("SUPABASE_SERVICE_ROLE_KEY", "service-role")
	t.Setenv("API_KEY", "api-key")
	t.Setenv("BATCH_SIZE", "50")
	t.Setenv("BATCH_MAX_RETRIES", "5")
	t.Setenv("HTTP_ADDR", ":9090")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.BatchSize != 50 || cfg.BatchMaxRetries != 5 || cfg.HTTPAddr != ":9090" {
		t.Fatalf("got batch=%d retries=%d addr=%q", cfg.BatchSize, cfg.BatchMaxRetries, cfg.HTTPAddr)
	}
}
