// Package claude は、Anthropic の Claude API に文だけのプロンプトを送り、決めた形の JSON を受け取る。
// 感覚メモのまとめに使う。有料の API なので、送った内容はモデルの学習に使われない。
package claude

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"
)

// DefaultModel は、短い日本語をまとめるだけなので、いちばん軽くて安いモデル。
const DefaultModel = "claude-haiku-5-5"

// ErrRateLimited は、回数の上限(429)や混雑(529)で断られたとき。時間をおけば作れる。
var ErrRateLimited = errors.New("claude rate limit")

type Client struct {
	Key   string
	Model string
	// MaxTokens は返事(考える分を含む)の上限。
	MaxTokens int64
	// Endpoint はテストで Claude の代わりのサーバーに向けるためのもの。空なら本物の API。
	Endpoint string
}

// GenerateJSON は、prompt を送り、schema の形の JSON の文字列を返す。
func (c Client) GenerateJSON(ctx context.Context, prompt string, schema map[string]any) (string, error) {
	// 失敗は呼び出し側が回数に数えて、自動ではやり直さない。
	opts := []option.RequestOption{option.WithAPIKey(c.Key), option.WithMaxRetries(0)}
	if c.Endpoint != "" {
		opts = append(opts, option.WithBaseURL(c.Endpoint))
	}
	client := anthropic.NewClient(opts...)
	model := c.Model
	if model == "" {
		model = DefaultModel
	}
	res, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: c.MaxTokens,
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(prompt))},
		// まとめは深く考えなくてよいので、考える量を減らして早く安く返す。
		OutputConfig: anthropic.OutputConfigParam{
			Effort: anthropic.OutputConfigEffortLow,
			Format: anthropic.JSONOutputFormatParam{Schema: schema},
		},
	})
	if err != nil {
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) && (apiErr.StatusCode == 429 || apiErr.StatusCode == 529) {
			return "", fmt.Errorf("%w: %d", ErrRateLimited, apiErr.StatusCode)
		}
		return "", err
	}
	if res.StopReason == anthropic.StopReasonRefusal || res.StopReason == anthropic.StopReasonMaxTokens {
		return "", fmt.Errorf("claude stop_reason %s", res.StopReason)
	}
	var text string
	for _, block := range res.Content {
		if b, ok := block.AsAny().(anthropic.TextBlock); ok {
			text += b.Text
		}
	}
	if strings.TrimSpace(text) == "" {
		return "", errors.New("claude response に本文がありません")
	}
	return text, nil
}
