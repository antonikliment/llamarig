package router

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Model struct {
	ID     string `json:"id"`
	Status struct {
		Value string `json:"value"`
	} `json:"status"`
}

type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string, client *http.Client) *Client {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), http: client}
}

func (c *Client) List(ctx context.Context) ([]Model, error) {
	return c.list(ctx, "/models")
}

func (c *Client) Reload(ctx context.Context) ([]Model, error) {
	return c.list(ctx, "/models?reload=1")
}

func (c *Client) list(ctx context.Context, path string) ([]Model, error) {
	var response struct {
		Data []Model `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, path, nil, &response); err != nil {
		return nil, err
	}
	return response.Data, nil
}

func (c *Client) Load(ctx context.Context, model string) error {
	return c.do(ctx, http.MethodPost, "/models/load", map[string]string{"model": model}, nil)
}

func (c *Client) Unload(ctx context.Context, model string) error {
	return c.do(ctx, http.MethodPost, "/models/unload", map[string]string{"model": model}, nil)
}

func (c *Client) do(ctx context.Context, method, path string, body map[string]string, dst any) error {
	var data []byte
	if body != nil {
		data, _ = json.Marshal(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("router %s %s: %w", method, path, err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		detail := ""
		if data, readErr := io.ReadAll(io.LimitReader(resp.Body, 512)); readErr == nil && len(data) > 0 {
			detail = ": " + strings.TrimSpace(string(data))
		}
		return fmt.Errorf("router %s %s returned %s%s", method, path, resp.Status, detail)
	}
	if dst != nil {
		if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
			return fmt.Errorf("decode router %s: %w", path, err)
		}
	}
	return nil
}
