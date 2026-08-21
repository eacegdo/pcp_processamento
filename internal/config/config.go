package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	SupabaseURL            string
	SupabaseServiceRoleKey string
	APIKey                 string
	BatchSize              int
	BatchMaxRetries        int
	HTTPAddr               string
	BubbleBaseURL          string
	BubbleAPIToken         string
}

func Load() (Config, error) {
	cfg := Config{
		SupabaseURL:            os.Getenv("SUPABASE_URL"),
		SupabaseServiceRoleKey: os.Getenv("SUPABASE_SERVICE_ROLE_KEY"),
		APIKey:                 os.Getenv("API_KEY"),
		BatchSize:              200,
		BatchMaxRetries:        3,
		HTTPAddr:               ":8080",
		BubbleBaseURL:          os.Getenv("BUBBLE_BASE_URL"),
		BubbleAPIToken:         os.Getenv("BUBBLE_API_TOKEN"),
	}
	if cfg.SupabaseURL == "" || cfg.SupabaseServiceRoleKey == "" || cfg.APIKey == "" {
		return Config{}, fmt.Errorf("SUPABASE_URL, SUPABASE_SERVICE_ROLE_KEY e API_KEY são obrigatórios")
	}
	if v := os.Getenv("BATCH_SIZE"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return Config{}, fmt.Errorf("BATCH_SIZE inválido: %q", v)
		}
		cfg.BatchSize = n
	}
	if v := os.Getenv("BATCH_MAX_RETRIES"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return Config{}, fmt.Errorf("BATCH_MAX_RETRIES inválido: %q", v)
		}
		cfg.BatchMaxRetries = n
	}
	if v := os.Getenv("HTTP_ADDR"); v != "" {
		cfg.HTTPAddr = v
	}
	return cfg, nil
}
