package competitions

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const Model = "gemini-3.8-flash"

var ErrRateLimited = errors.New("gemini rate limit")

var generateEndpoint = "https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent"

type Client struct {
	Key   string
	Model string
	HTTP  *http.Client
}

func (c Client) Extract(ctx context.Context, page Page) ([]Draft, error) {
	var parts []map[string]any
	for _, pdf := range page.PDFs {
		parts = append(parts, map[string]any{"inline_data": map[string]string{
			"mime_type": "application/pdf",
			"data":      base64.StdEncoding.EncodeToString(pdf),
		}})
	}
	parts = append(parts, map[string]any{"text": extractionPrompt(page)})
	text, err := c.generate(ctx, parts)
	if err != nil {
		return nil, err
	}
	return parseDrafts(text)
}

// generate は、Gemini にリクエストを1回送り、返ってきた JSON の文字列を返す。
func (c Client) generate(ctx context.Context, parts []map[string]any) (string, error) {
	model := c.Model
	if model == "" {
		model = Model
	}
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	body, err := json.Marshal(map[string]any{
		"contents": []map[string]any{
			{"role": "user", "parts": parts},
		},
		"generationConfig": map[string]any{
			"temperature":      0,
			"responseMimeType": "application/json",
			"thinkingConfig":   map[string]string{"thinkingLevel": "low"},
		},
	})
	if err != nil {
		return "", err
	}
	endpoint := generateEndpoint
	if strings.Contains(generateEndpoint, "%s") {
		endpoint = fmt.Sprintf(generateEndpoint, model)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", c.Key)
	res, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return "", err
	}
	if res.StatusCode == http.StatusTooManyRequests || bytes.Contains(payload, []byte("RESOURCE_EXHAUSTED")) {
		return "", fmt.Errorf("%w: %s", ErrRateLimited, truncate(payload))
	}
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gemini %s: %s", res.Status, truncate(payload))
	}
	return responseText(payload)
}

func extractionPrompt(page Page) string {
	prompt := baseExtractionPrompt(page)
	if page.plainText {
		prompt += "\n\n" + plainTextNote
	}
	if len(page.PDFs) > 0 {
		prompt += "\n\n" + pdfNote
	}
	return prompt
}

// pdfNote は、日程表がページの文字ではなく添付の PDF のときに付け足す。
const pdfNote = `注意: 大会一覧は添付のPDFです。PDFの中の大会をすべて読んでください。PDFにURLは無いので guidelines_url と district_url は空文字にしてください。
PDFが複数あるときは、同じ大会が複数のPDFに載っていても1件だけ返してください。
回数が大会名とは別の列にあるときや、「81宍道湖一周駅伝」のように大会名の先頭に数字だけが付いているときは、「第81回宍道湖一周駅伝」のように書いてください。
GP や ① などの区分記号は大会名に入れないでください。
理事会、講習会、会議、合宿などの行事と、中止、延期の大会は返さないでください。`

// plainTextNote は、ページが表の行ではなく、ただの文字の行のときに付け足す。
const plainTextNote = `注意: 次の本文は表ではなく、ページの文字を行ごとに並べたものです。リンクは「文字 URL」の形です。大会名、日付、会場、要項のリンクは近い行にあります。
会場の欄が無ければ location は空文字にし、大会名の括弧内の数字や語を会場にしないでください。
中止、延期になった大会は返さないでください。大会以外のお知らせ、リンク集、過去の年度は返さないでください。`

func baseExtractionPrompt(page Page) string {
	return fmt.Sprintf(`この陸上競技の大会一覧から、大会だけをJSONで返してください。形式は {"meets":[{"name":"","starts_on":"","ends_on":"","location":"","guidelines_url":"","district_url":""}]} です。
競技年度は%d年です。4月から12月はその年、1月から3月は翌年です。月と日が別の欄なら組み合わせてください。期日欄も日です。日が範囲なら開始日と終了日にしてください。日付はYYYY-MM-DDにしてください。終了日が開始日と同じ、または単日なら ends_on は空文字です。会場欄があれば、その文字を location にしてください。会場欄が無く、大会名の括弧内が会場なら、そこを location にしてください。
guidelines_url は「要項」欄にある要項のURLだけにしてください。申込、結果、タイムテーブル、注意事項、競技日程、エントリーは入れないでください。
要項欄が陸協、スポ協、体連、大会公式ホームページへのリンクなら、そのURLは district_url に入れ、guidelines_url は空文字にしてください。
ページに無いURLは空文字にしてください。会場が無ければ location は空文字です。日付が読めない行は返さないでください。

%s`, page.SeasonYear, page.Body)
}

func responseText(payload []byte) (string, error) {
	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text    string `json:"text"`
					Thought bool   `json:"thought"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return "", fmt.Errorf("gemini response: %w", err)
	}
	if len(parsed.Candidates) == 0 {
		return "", fmt.Errorf("gemini response に候補がありません")
	}
	var text string
	for _, part := range parsed.Candidates[0].Content.Parts {
		if part.Thought || strings.TrimSpace(part.Text) == "" {
			continue
		}
		text = part.Text
	}
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("gemini response に本文がありません")
	}
	return text, nil
}

func parseDrafts(raw string) ([]Draft, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)
	var wrapped struct {
		Meets []Draft `json:"meets"`
	}
	if err := json.Unmarshal([]byte(raw), &wrapped); err == nil && wrapped.Meets != nil {
		return wrapped.Meets, nil
	}
	var list []Draft
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return nil, fmt.Errorf("gemini json: %w", err)
	}
	return list, nil
}

func truncate(payload []byte) string {
	text := strings.Join(strings.Fields(string(payload)), " ")
	if len(text) > 300 {
		return text[:300]
	}
	return text
}
