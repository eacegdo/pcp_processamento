package supabase

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func NewClient(supabaseURL, serviceRoleKey string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		baseURL: strings.TrimRight(supabaseURL, "/") + "/rest/v1",
		apiKey:  serviceRoleKey,
		http:    httpClient,
	}
}

func (c *Client) do(method, pathQuery string, body io.Reader, prefer string) (*http.Response, error) {
	url := c.baseURL + pathQuery
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("apikey", c.apiKey)
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if prefer != "" {
		req.Header.Set("Prefer", prefer)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("supabase %s %s: %s: %s", method, pathQuery, resp.Status, strings.TrimSpace(string(b)))
	}
	return resp, nil
}
