// Package openai は、OpenAI の Chat Completions を1回呼ぶだけの小さなクライアント。
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// DefaultModel は、Client.Model が空のときに使うモデル。OPENAI_MODEL で変えられる。
const DefaultModel = "gpt-5-mini"

const defaultAPIBase = "https://api.openai.com"

type Client struct {
	Key   string
	Model string
	// APIBase はテストで OpenAI の代わりのサーバーに向けるためのもの。空なら本物の OpenAI。
	APIBase string
	HTTP    *http.Client
}

// GenerateJSON は、プロンプトを送り、返ってきた JSON の文字列を返す。
func (c Client) GenerateJSON(ctx context.Context, prompt string) (string, error) {
	model := c.Model
	if model == "" {
		model = DefaultModel
	}
	base := c.APIBase
	if base == "" {
		base = defaultAPIBase
	}
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	body, err := json.Marshal(map[string]any{
		"model":           model,
		"messages":        []map[string]string{{"role": "user", "content": prompt}},
		"response_format": map[string]string{"type": "json_object"},
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(base, "/")+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Key)
	res, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return "", err
	}
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openai %s: %s", res.Status, truncate(payload))
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return "", fmt.Errorf("openai response: %w", err)
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return "", errors.New("openai response に本文がありません")
	}
	return parsed.Choices[0].Message.Content, nil
}

func truncate(payload []byte) string {
	text := strings.Join(strings.Fields(string(payload)), " ")
	if len(text) > 300 {
		return text[:300]
	}
	return text
}
