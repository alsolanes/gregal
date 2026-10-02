package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

func (c *Client) Approve(ctx context.Context, key string, approve, remember bool) error {
	return c.interaction(ctx, "/api/approve", map[string]any{"key": key, "approve": approve, "remember": remember})
}

func (c *Client) AnswerQuestion(ctx context.Context, key, answer string) error {
	return c.interaction(ctx, "/api/question", map[string]string{"key": key, "answer": answer})
}

func (c *Client) interaction(ctx context.Context, route string, body any) error {
	if c == nil || c.BaseURL == "" {
		return fmt.Errorf("empty service client")
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+route, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if c.SessionID != "" {
		req.Header.Set("X-Gregal-Session", c.SessionID)
	}
	res, err := c.http().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("service: HTTP %d", res.StatusCode)
	}
	return nil
}
