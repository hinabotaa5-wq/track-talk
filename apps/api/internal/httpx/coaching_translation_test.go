package httpx_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

// 海外コーチとのやり取りの自動翻訳（本体 #916）。Anthropic は AnthropicEndpoint の代役サーバーで確かめる（実 API は叩かない）。

// fakeTranslator は Claude の代役。既定では原文を "EN<…>"／"JA<…>" で包んで返し、どの原文がどちらへ訳されたかを確かめられるようにする。
type fakeTranslator struct {
	mu      sync.Mutex
	prompts []string
	status  int
	// override が空でない文字列を返したら、その本文（JSON）をそのまま返す。
	override func(prompt string) string
}

func (f *fakeTranslator) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.prompts)
}

func (f *fakeTranslator) lastPrompt() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.prompts) == 0 {
		return ""
	}
	return f.prompts[len(f.prompts)-1]
}

func translationSource(prompt string) string {
	i := strings.LastIndex(prompt, "source:\n<<<\n")
	j := strings.LastIndex(prompt, "\n>>>")
	if i < 0 || j < i {
		return ""
	}
	return prompt[i+len("source:\n<<<\n") : j]
}

func defaultTranslation(prompt string) string {
	wrap := func(s string) string {
		if strings.Contains(prompt, "into natural English") {
			return "EN<" + s + ">"
		}
		return "JA<" + s + ">"
	}
	source := translationSource(prompt)
	var reply any
	switch {
	case strings.Contains(prompt, "good_point, fix_points (an array) and drill"):
		var src struct {
			GoodPoint string   `json:"good_point"`
			FixPoints []string `json:"fix_points"`
			Drill     string   `json:"drill"`
		}
		_ = json.Unmarshal([]byte(source), &src)
		fixes := []string{}
		for _, f := range src.FixPoints {
			fixes = append(fixes, wrap(f))
		}
		reply = map[string]any{"good_point": wrap(src.GoodPoint), "fix_points": fixes, "drill": wrap(src.Drill)}
	case strings.Contains(prompt, "JSON training menu"):
		var src struct {
			Title, Period, Note string
			Days                []struct{ Day, Content string }
		}
		_ = json.Unmarshal([]byte(source), &src)
		days := []map[string]string{}
		for _, d := range src.Days {
			days = append(days, map[string]string{"day": wrap(d.Day), "content": wrap(d.Content)})
		}
		reply = map[string]any{"title": wrap(src.Title), "period": "", "note": "", "days": days}
	default:
		reply = map[string]any{"text": wrap(source)}
	}
	b, _ := json.Marshal(reply)
	return string(b)
}

func (f *fakeTranslator) serve(t *testing.T) string {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body struct {
			Model    string `json:"model"`
			Messages []struct {
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"messages"`
		}
		_ = json.Unmarshal(raw, &body)
		prompt := body.Messages[0].Content[0].Text
		f.mu.Lock()
		f.prompts = append(f.prompts, prompt)
		status, override := f.status, f.override
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if status != 0 {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"type":"error","error":{"type":"api_error","message":"boom"}}`))
			return
		}
		text := ""
		if override != nil {
			text = override(prompt)
		}
		if text == "" {
			text = defaultTranslation(prompt)
		}
		payload, _ := json.Marshal(map[string]any{
			"id": "msg_test", "type": "message", "role": "assistant", "model": body.Model,
			"content":     []any{map[string]string{"type": "text", "text": text}},
			"stop_reason": "end_turn",
			"usage":       map[string]int{"input_tokens": 1, "output_tokens": 1},
		})
		_, _ = w.Write(payload)
	}))
	t.Cleanup(server.Close)
	return server.URL
}

func translationConfig(endpoint string, mutate func(*config.Config)) func(*config.Config) {
	return func(c *config.Config) {
		c.CoachingTranslationEnabled = true
		c.AnthropicAPIKey = "test"
		c.AnthropicEndpoint = endpoint
		if mutate != nil {
			mutate(c)
		}
	}
}

// setupTranslationStripe は、決済の代役つきのサーバー（添削・追加質問用）。
func setupTranslationStripe(t *testing.T, ai *fakeTranslator, mutate func(*config.Config)) http.Handler {
	t.Helper()
	h, _ := setupStripeConfig(t, translationConfig(ai.serve(t), mutate))
	execSQLArgs(t, `TRUNCATE coaching_translation_attempts`)
	return h
}

// setupTranslation は、決済なしのサーバー（やりとり欄用）。
func setupTranslation(t *testing.T, ai *fakeTranslator, mutate func(*config.Config)) http.Handler {
	t.Helper()
	h := setupConfig(t, translationConfig(ai.serve(t), mutate))
	execSQLArgs(t, `TRUNCATE coaching_translation_attempts`)
	return h
}

func setLocale(t *testing.T, id any, locale string) {
	t.Helper()
	execSQLArgs(t, `UPDATE profiles SET locale = $2 WHERE id = $1`, id, locale)
}

// jsonText は、応答の JSON の中に現れる形（< や > はエスケープされる）の文字列リテラル。
func jsonText(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func sourceHash(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}

const (
	translationQuestion = "スタートの出だしで腰が落ちる気がします。100m 11秒20 です"
	translationInfo     = "動画のどの区間（スタートか中盤か後半か）を見てほしいのか、具体的に教えてください。"
)

func translationResponseBody() map[string]any {
	return map[string]any{
		"good_point": "1台目までの8歩が安定しています",
		"fix_points": []string{"踏切が近いので半足ぶん手前で踏み切りましょう", "抜き脚の膝を高く"},
		"drill":      "ミニハードル5台を1.5m間隔で3セット",
		"hitokoto":   "次は抜き脚だけ意識してみましょう",
	}
}

// translationReview は、チケットで依頼して担当コーチ（locale を指定）が決まった添削を作る。
func translationReview(t *testing.T, h http.Handler, locale string) (owner, coach string, coachID uuid.UUID, id string) {
	t.Helper()
	admin := adminToken(t, h)
	coachID, coach = onDutyCoach(t, h, admin)
	setLocale(t, coachID, locale)
	_, owner = demoRequester(t, h, admin)
	buySixTickets(t, h, owner)
	body := reviewBodyFor(t, h, owner)
	body["method"] = "tickets"
	body["question"] = translationQuestion
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", owner, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var out struct{ ID string }
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	id = out.ID
	if r := doJSON(t, h, http.MethodPost, "/v1/coach/requests/"+id+"/start", coach, nil); r.Code != http.StatusOK {
		t.Fatalf("start %d %s", r.Code, r.Body.String())
	}
	return
}

type reviewTranslationRow struct {
	QuestionEn, QuestionHash, GoodPointJa, DrillJa, ResponseHash, InfoJa, InfoHash string
	FixPointsJa                                                                    []string
	Rows                                                                           int
}

func storedReviewTranslation(t *testing.T, id string) reviewTranslationRow {
	t.Helper()
	var r reviewTranslationRow
	queryRowArgs(t, `SELECT count(*)::int, coalesce(max(question_en),''), coalesce(max(question_source_hash),''), coalesce(max(good_point_ja),''),
		coalesce(max(drill_ja),''), coalesce(max(response_source_hash),''), coalesce(max(info_request_ja),''), coalesce(max(info_request_source_hash),''),
		coalesce((SELECT fix_points_ja FROM review_translations WHERE request_id=$1), '{}')
		FROM review_translations WHERE request_id=$1`, []any{id},
		&r.Rows, &r.QuestionEn, &r.QuestionHash, &r.GoodPointJa, &r.DrillJa, &r.ResponseHash, &r.InfoJa, &r.InfoHash, &r.FixPointsJa)
	return r
}

type reviewTranslationView struct {
	Question    string  `json:"question"`
	QuestionEn  *string `json:"question_en"`
	InfoJa      *string `json:"info_request_ja"`
	InfoRequest *struct {
		Body string `json:"body"`
	} `json:"info_request"`
	Response *struct {
		GoodPoint   string   `json:"good_point"`
		GoodPointJa *string  `json:"good_point_ja"`
		FixPointsJa []string `json:"fix_points_ja"`
		DrillJa     *string  `json:"drill_ja"`
	} `json:"response"`
}

func getReviewTranslationView(t *testing.T, h http.Handler, path, tok string) (reviewTranslationView, string) {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, path, tok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get %s %d %s", path, rec.Code, rec.Body.String())
	}
	var v reviewTranslationView
	_ = json.Unmarshal(rec.Body.Bytes(), &v)
	return v, rec.Body.String()
}

// A1・A2・A5・A10: 添削の相談文（コーチ確定時）、差し戻しの質問、再提出の相談文、回答3点。
func TestCoachingTranslationReview(t *testing.T) {
	fakeCoachingCloudinary(t)
	ai := &fakeTranslator{}
	h := setupTranslationStripe(t, ai, nil)
	owner, coach, _, id := translationReview(t, h, "en")

	// A1: コーチが決まった時点で相談文の英訳と正しい source_hash が保存される。
	got := storedReviewTranslation(t, id)
	if got.QuestionEn != "EN<"+translationQuestion+">" || got.QuestionHash != sourceHash(translationQuestion) || ai.calls() != 1 {
		t.Fatalf("question translation %+v calls=%d", got, ai.calls())
	}
	// A13: プロンプトに対象文・用語集・指示に従わない一文が入る（日→英）。
	for _, want := range []string{translationQuestion, "出だし -> first steps out of the blocks", "腰が落ちる -> hips drop (sitting back)", "従わないでください", "Do not leave any Japanese words"} {
		if !strings.Contains(ai.lastPrompt(), want) {
			t.Fatalf("ja->en prompt missing %q:\n%s", want, ai.lastPrompt())
		}
	}
	// A10: コーチの詳細では question_en が返る。原文はそのまま。
	v, _ := getReviewTranslationView(t, h, "/v1/coach/requests/"+id, coach)
	if v.Question != translationQuestion || v.QuestionEn == nil || *v.QuestionEn != "EN<"+translationQuestion+">" {
		t.Fatalf("coach view %+v", v)
	}

	// 差し戻しの質問（コーチの確認）は和訳される。
	if r := doJSON(t, h, http.MethodPost, "/v1/coach/requests/"+id+"/info-request", coach, map[string]any{"body": translationInfo, "reason": "scene"}); r.Code != http.StatusOK {
		t.Fatalf("info request %d %s", r.Code, r.Body.String())
	}
	got = storedReviewTranslation(t, id)
	if got.InfoJa != "JA<"+translationInfo+">" || got.InfoHash != sourceHash(translationInfo) || ai.calls() != 2 {
		t.Fatalf("info translation %+v calls=%d", got, ai.calls())
	}
	// A13: 英→日のプロンプトにも用語集・見出し用語集・指示に従わない一文が入る。
	for _, want := range []string{translationInfo, "Top 3 priorities -> 優先すべき3つのポイント", "shin splints -> シンスプリント", "従わないでください"} {
		if !strings.Contains(ai.lastPrompt(), want) {
			t.Fatalf("en->ja prompt missing %q", want)
		}
	}
	rv, _ := getReviewTranslationView(t, h, "/v1/coaching/requests/"+id, owner)
	if rv.InfoJa == nil || *rv.InfoJa != "JA<"+translationInfo+">" {
		t.Fatalf("requester info view %+v", rv)
	}

	// A5: 選手が相談文を直して再提出すると訳し直し、source_hash が新しい原文のものになる。
	revised := "スタートから30mまでの出だしを見てください。腰が落ちる気がします"
	if r := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/resubmit", owner, map[string]any{"question": revised}); r.Code != http.StatusOK {
		t.Fatalf("resubmit %d %s", r.Code, r.Body.String())
	}
	got = storedReviewTranslation(t, id)
	if got.QuestionEn != "EN<"+revised+">" || got.QuestionHash != sourceHash(revised) || ai.calls() != 3 {
		t.Fatalf("resubmitted translation %+v calls=%d", got, ai.calls())
	}

	// A2: 返却時に回答3点が1回の呼び出しで訳され、直す点の数が原文と一致する。
	body := translationResponseBody()
	if r := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/response", coach, body); r.Code != http.StatusOK {
		t.Fatalf("response %d %s", r.Code, r.Body.String())
	}
	if ai.calls() != 4 {
		t.Fatalf("response calls = %d, want 4", ai.calls())
	}
	got = storedReviewTranslation(t, id)
	fixes := body["fix_points"].([]string)
	if got.GoodPointJa != "JA<1台目までの8歩が安定しています>" || got.DrillJa != "JA<ミニハードル5台を1.5m間隔で3セット>" || len(got.FixPointsJa) != len(fixes) ||
		got.FixPointsJa[1] != "JA<抜き脚の膝を高く>" || got.ResponseHash != sourceHash(body["good_point"].(string), strings.Join(fixes, "\x00"), body["drill"].(string)) {
		t.Fatalf("response translation %+v", got)
	}
	rv, raw := getReviewTranslationView(t, h, "/v1/coaching/requests/"+id, owner)
	if rv.Response == nil || rv.Response.GoodPoint != "1台目までの8歩が安定しています" || rv.Response.GoodPointJa == nil || len(rv.Response.FixPointsJa) != 2 || rv.Response.DrillJa == nil {
		t.Fatalf("requester response view %s", raw)
	}
	if rv.QuestionEn == nil || *rv.QuestionEn != "EN<"+revised+">" {
		t.Fatalf("requester question_en %s", raw)
	}

	// A5・A11: 原文が変わらなければ backfill でも API を呼ばない（全て skipped）。
	tadmin := adminToken(t, h)
	rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching-translations/backfill", tadmin, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"translated":0`) || !strings.Contains(rec.Body.String(), `"skipped":3`) || !strings.Contains(rec.Body.String(), `"failed":0`) {
		t.Fatalf("backfill %d %s", rec.Code, rec.Body.String())
	}
	if ai.calls() != 4 {
		t.Fatalf("backfill called API: %d", ai.calls())
	}

	// A6: 原文が変わったあとに生成が失敗したら、古い訳は公開 API に出ない。
	execSQLArgs(t, `UPDATE review_requests SET question = 'なおした相談文です' WHERE id = $1`, id)
	ai.mu.Lock()
	ai.status = http.StatusInternalServerError
	ai.mu.Unlock()
	rec = doJSON(t, h, http.MethodPost, "/v1/admin/coaching-translations/backfill", tadmin, nil)
	if !strings.Contains(rec.Body.String(), `"failed":1`) {
		t.Fatalf("backfill after change %s", rec.Body.String())
	}
	rv, raw = getReviewTranslationView(t, h, "/v1/coaching/requests/"+id, owner)
	if rv.QuestionEn != nil {
		t.Fatalf("stale question_en exposed %s", raw)
	}
	if rv.Response == nil || rv.Response.GoodPointJa == nil {
		t.Fatalf("current response translation should remain %s", raw)
	}

	// A12: 依頼を消すと訳の行も消える（CASCADE）。依頼は支払い・チケットの記録が参照していて消せないので、外部キーの定義で確かめる。
	var cascades int
	queryRowArgs(t, `SELECT count(*)::int FROM pg_constraint WHERE conrelid='review_translations'::regclass AND confrelid='review_requests'::regclass AND contype='f' AND confdeltype='c'`, nil, &cascades)
	if cascades != 1 {
		t.Fatal("review_translations should cascade on review_requests delete")
	}
}

// A1・A10: 国内コーチ（locale = 'ja'）のペアでは生成が走らず、訳の項目は全て null。
func TestCoachingTranslationSkipsDomesticCoach(t *testing.T) {
	fakeCoachingCloudinary(t)
	ai := &fakeTranslator{}
	h := setupTranslationStripe(t, ai, nil)
	owner, coach, _, id := translationReview(t, h, "ja")
	if r := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/response", coach, translationResponseBody()); r.Code != http.StatusOK {
		t.Fatalf("response %d %s", r.Code, r.Body.String())
	}
	if ai.calls() != 0 {
		t.Fatalf("domestic coach called API %d times", ai.calls())
	}
	// 訳の行があっても、対象外のペアでは使わない。
	execSQLArgs(t, `INSERT INTO review_translations (request_id, question_en, question_source_hash) VALUES ($1, 'leaked', $2)`, id, sourceHash(translationQuestion))
	for _, path := range []string{"/v1/coaching/requests/" + id, "/v1/coach/requests/" + id} {
		tok := owner
		if strings.HasPrefix(path, "/v1/coach/") {
			tok = coach
		}
		v, raw := getReviewTranslationView(t, h, path, tok)
		if v.QuestionEn != nil || v.InfoJa != nil || v.Response == nil || v.Response.GoodPointJa != nil || v.Response.FixPointsJa != nil || v.Response.DrillJa != nil {
			t.Fatalf("domestic view has translations %s", raw)
		}
		for _, key := range []string{`"question_en":null`, `"info_request_ja":null`, `"good_point_ja":null`, `"fix_points_ja":null`, `"drill_ja":null`} {
			if !strings.Contains(raw, key) {
				t.Fatalf("missing %s in %s", key, raw)
			}
		}
	}
	// backfill も国内コーチのペアは対象にしない。
	rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching-translations/backfill", adminToken(t, h), nil)
	if !strings.Contains(rec.Body.String(), `"translated":0,`) && !strings.Contains(rec.Body.String(), `"translated":0}`) || ai.calls() != 0 {
		t.Fatalf("backfill domestic %s calls=%d", rec.Body.String(), ai.calls())
	}
}

// A6・A13: 生成の失敗（500・不正な JSON・上限超え・キー欠け・型違い・直す点の数の不一致）でも返却は成功し、訳は保存されない。
func TestCoachingTranslationFailuresFailOpen(t *testing.T) {
	fakeCoachingCloudinary(t)
	cases := map[string]func(string) string{
		"status 500":   nil,
		"invalid json": func(string) string { return "not json" },
		"too long": func(string) string {
			return `{"good_point":"` + strings.Repeat("あ", 30001) + `","fix_points":["a","b"],"drill":"c"}`
		},
		"missing key": func(string) string { return `{"good_point":"a","fix_points":["a","b"]}` },
		"wrong type":  func(string) string { return `{"good_point":1,"fix_points":["a","b"],"drill":"c"}` },
		"fix count":   func(string) string { return `{"good_point":"a","fix_points":["a"],"drill":"c"}` },
	}
	for name, override := range cases {
		t.Run(name, func(t *testing.T) {
			ai := &fakeTranslator{}
			h := setupTranslationStripe(t, ai, nil)
			owner, coach, _, id := translationReview(t, h, "en")
			ai.mu.Lock()
			if override == nil {
				ai.status = http.StatusInternalServerError
			} else {
				ai.override = override
			}
			ai.mu.Unlock()
			before := ai.calls()
			if r := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/response", coach, translationResponseBody()); r.Code != http.StatusOK {
				t.Fatalf("response must succeed: %d %s", r.Code, r.Body.String())
			}
			if ai.calls() != before+1 {
				t.Fatalf("calls %d -> %d", before, ai.calls())
			}
			if got := storedReviewTranslation(t, id); got.ResponseHash != "" || got.GoodPointJa != "" {
				t.Fatalf("stored on failure %+v", got)
			}
			v, raw := getReviewTranslationView(t, h, "/v1/coaching/requests/"+id, owner)
			if v.Response == nil || v.Response.GoodPointJa != nil || v.Response.FixPointsJa != nil || v.Response.DrillJa != nil {
				t.Fatalf("view after failure %s", raw)
			}
		})
	}
}

// A7: フラグ OFF、または API キーが空なら生成せず、本体の書き込みは成功する。コーチプロフィール翻訳のフラグとは独立。
func TestCoachingTranslationFlagAndKey(t *testing.T) {
	fakeCoachingCloudinary(t)
	for name, mutate := range map[string]func(*config.Config){
		"flag off, coach profile flag on": func(c *config.Config) {
			c.CoachingTranslationEnabled = false
			c.CoachProfileTranslationEnabled = true
		},
		"no api key": func(c *config.Config) { c.AnthropicAPIKey = "" },
	} {
		t.Run(name, func(t *testing.T) {
			ai := &fakeTranslator{}
			h := setupTranslationStripe(t, ai, mutate)
			owner, coach, _, id := translationReview(t, h, "en")
			if r := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/response", coach, translationResponseBody()); r.Code != http.StatusOK {
				t.Fatalf("response %d %s", r.Code, r.Body.String())
			}
			// コーチプロフィール翻訳のフラグが開いていると承認時にそちらは呼ばれうるので、やり取りの訳の呼び出しだけを数える。
			for _, p := range ai.prompts {
				if strings.Contains(p, "source:\n<<<") {
					t.Fatalf("coaching translation called: %s", p)
				}
			}
			if got := storedReviewTranslation(t, id); got.Rows != 0 {
				t.Fatalf("stored %+v", got)
			}
			v, _ := getReviewTranslationView(t, h, "/v1/coaching/requests/"+id, owner)
			if v.QuestionEn != nil || v.Response == nil || v.Response.GoodPointJa != nil {
				t.Fatalf("view %+v", v)
			}
		})
	}
	// コーチプロフィール翻訳のフラグが閉じていても、本機能のフラグが開いていれば訳す。
	ai := &fakeTranslator{}
	h := setupTranslationStripe(t, ai, func(c *config.Config) { c.CoachProfileTranslationEnabled = false })
	_, _, _, id := translationReview(t, h, "en")
	if got := storedReviewTranslation(t, id); got.QuestionEn == "" {
		t.Fatal("translation should run independently of coach profile flag")
	}
}

// A8: 予約上限（300/日・30/分）を超えたら生成を諦め、本体の書き込みは成功する。
func TestCoachingTranslationBudget(t *testing.T) {
	fakeCoachingCloudinary(t)
	for name, seed := range map[string]string{
		"daily":  `INSERT INTO coaching_translation_attempts (attempted_at) SELECT now() - interval '2 hours' FROM generate_series(1, 300)`,
		"minute": `INSERT INTO coaching_translation_attempts (attempted_at) SELECT clock_timestamp() FROM generate_series(1, 30)`,
	} {
		t.Run(name, func(t *testing.T) {
			ai := &fakeTranslator{}
			h := setupTranslationStripe(t, ai, nil)
			owner, coach, _, id := translationReview(t, h, "en")
			execSQLArgs(t, `TRUNCATE coaching_translation_attempts`)
			execSQLArgs(t, seed)
			before := ai.calls()
			if r := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/response", coach, translationResponseBody()); r.Code != http.StatusOK {
				t.Fatalf("response %d %s", r.Code, r.Body.String())
			}
			if ai.calls() != before {
				t.Fatalf("API called over budget")
			}
			if got := storedReviewTranslation(t, id); got.ResponseHash != "" {
				t.Fatal("stored over budget")
			}
			if v, _ := getReviewTranslationView(t, h, "/v1/coaching/requests/"+id, owner); v.Response == nil || v.Response.GoodPointJa != nil {
				t.Fatalf("view %+v", v)
			}
		})
	}
}

// A9: 連絡先らしい訳文は、その項目だけ保存しない。他の項目は保存される。
func TestCoachingTranslationDropsContactLike(t *testing.T) {
	fakeCoachingCloudinary(t)
	ai := &fakeTranslator{}
	h := setupTranslationStripe(t, ai, nil)
	owner, coach, _, id := translationReview(t, h, "en")
	ai.mu.Lock()
	ai.override = func(string) string {
		return `{"good_point":"詳しくは https://example.com を見てください","fix_points":["coach@example.com に連絡","抜き脚の膝を高く"],"drill":"ミニハードルを3セット"}`
	}
	ai.mu.Unlock()
	if r := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/response", coach, translationResponseBody()); r.Code != http.StatusOK {
		t.Fatalf("response %d %s", r.Code, r.Body.String())
	}
	got := storedReviewTranslation(t, id)
	if got.GoodPointJa != "" || len(got.FixPointsJa) != 2 || got.FixPointsJa[0] != "" || got.FixPointsJa[1] != "抜き脚の膝を高く" || got.DrillJa != "ミニハードルを3セット" || got.ResponseHash == "" {
		t.Fatalf("contact-like kept %+v", got)
	}
	v, raw := getReviewTranslationView(t, h, "/v1/coaching/requests/"+id, owner)
	if v.Response == nil || v.Response.GoodPointJa != nil || v.Response.DrillJa == nil || *v.Response.DrillJa != "ミニハードルを3セット" {
		t.Fatalf("view %s", raw)
	}
}

type consultationTranslationView struct {
	ID              string  `json:"id"`
	Question        string  `json:"question"`
	QuestionEn      *string `json:"question_en"`
	AnswerJa        *string `json:"answer_ja"`
	ClarifyBodyEn   *string `json:"clarify_body_en"`
	ClarificationJa *string `json:"clarification_ja"`
}

func consultationRoomTranslations(t *testing.T, h http.Handler, tok, path string) []consultationTranslationView {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, path, tok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("room %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Consultations []consultationTranslationView `json:"consultations"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out.Consultations
}

// A3・A10: 追加質問。下書きでは訳さず、送信で質問、回答・確認・補足でそれぞれを訳す。
// 確認（clarify_body）は選手、補足回答（clarification）はコーチが書く（実物の向き）。
func TestCoachingTranslationConsultation(t *testing.T) {
	ai := &fakeTranslator{}
	h := setupTranslationStripe(t, ai, nil)
	owner, coach, root := endedCashRoom(t, h)
	var coachID uuid.UUID
	queryRowArgs(t, `SELECT coach_id FROM coaching_contracts WHERE id=$1`, []any{root}, &coachID)
	setLocale(t, coachID, "en")
	path := "/v1/coaching/contracts/" + root + "/consultations"
	question := "大会前の調整で、流しは何本がいいですか"
	c := consultationDraft(t, h, owner, path, uuid.NewString(), question)
	if ai.calls() != 0 {
		t.Fatalf("draft called API %d", ai.calls())
	}
	submitted := doJSON(t, h, http.MethodPost, "/v1/coaching/consultations/"+c.ID+"/submit", owner, map[string]any{"guardian_permission": true})
	if submitted.Code != http.StatusOK || !strings.Contains(submitted.Body.String(), `"question_en":`+jsonText("EN<"+question+">")) {
		t.Fatalf("submit %d %s", submitted.Code, submitted.Body.String())
	}
	if code := payConsultation(t, h, c.ID, 1200); code != 200 {
		t.Fatalf("pay %d", code)
	}
	consultationAction(t, h, coach, c.ID, "accept", nil)
	answer := "Do 3 relaxed runs at submaximal effort."
	if r := doJSON(t, h, http.MethodPost, "/v1/coaching/consultations/"+c.ID+"/answer", coach, map[string]any{"body": answer}); r.Code != 200 || !strings.Contains(r.Body.String(), `"answer_ja":`+jsonText("JA<"+answer+">")) {
		t.Fatalf("answer %d %s", r.Code, r.Body.String())
	}
	clarify := "流しの距離も教えてください"
	consultationAction(t, h, owner, c.ID, "clarify", map[string]any{"body": clarify})
	clarification := "Each run should be about 100m."
	consultationAction(t, h, coach, c.ID, "clarification", map[string]any{"body": clarification})
	if ai.calls() != 4 {
		t.Fatalf("calls %d, want 4", ai.calls())
	}
	var qEn, aJa, cEn, clJa, qHash string
	queryRowArgs(t, `SELECT question_en, answer_ja, clarify_body_en, clarification_ja, question_source_hash FROM consultation_translations WHERE consultation_id=$1`, []any{c.ID}, &qEn, &aJa, &cEn, &clJa, &qHash)
	if qEn != "EN<"+question+">" || aJa != "JA<"+answer+">" || cEn != "EN<"+clarify+">" || clJa != "JA<"+clarification+">" || qHash != sourceHash(question) {
		t.Fatalf("stored %q %q %q %q %q", qEn, aJa, cEn, clJa, qHash)
	}
	// A10: ルームの一覧で全ての訳が返る（選手・コーチどちらにも）。
	for _, tok := range []string{owner, coach} {
		rows := consultationRoomTranslations(t, h, tok, path)
		if len(rows) != 1 || rows[0].Question != question || rows[0].QuestionEn == nil || rows[0].AnswerJa == nil || rows[0].ClarifyBodyEn == nil || *rows[0].ClarificationJa != "JA<"+clarification+">" {
			t.Fatalf("room view %+v", rows)
		}
	}
	// A11: 新しい訳はスキップされる。
	rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching-translations/backfill", adminToken(t, h), nil)
	if !strings.Contains(rec.Body.String(), `"skipped":4`) || !strings.Contains(rec.Body.String(), `"translated":0`) {
		t.Fatalf("backfill %s", rec.Body.String())
	}
	// A12: 追加質問を消すと訳も消える。
	var before int
	queryRowArgs(t, `SELECT count(*)::int FROM consultation_translations WHERE consultation_id=$1`, []any{c.ID}, &before)
	execSQLArgs(t, `DELETE FROM coach_earnings WHERE consultation_id=$1`, c.ID)
	execSQLArgs(t, `DELETE FROM coaching_consultations WHERE id=$1`, c.ID)
	var after int
	queryRowArgs(t, `SELECT count(*)::int FROM consultation_translations WHERE consultation_id=$1`, []any{c.ID}, &after)
	if before != 1 || after != 0 {
		t.Fatalf("cascade %d -> %d", before, after)
	}
}

type messageTranslationView struct {
	ID         string          `json:"id"`
	SenderRole string          `json:"sender_role"`
	Body       string          `json:"body"`
	BodyEn     *string         `json:"body_en"`
	BodyJa     *string         `json:"body_ja"`
	Menu       json.RawMessage `json:"menu"`
	MenuJa     json.RawMessage `json:"menu_ja"`
}

func messageTranslations(t *testing.T, h http.Handler, tok, id string) ([]messageTranslationView, string) {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/coaching/contracts/"+id+"/messages", tok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Messages []messageTranslationView `json:"messages"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out.Messages, rec.Body.String()
}

// A4・A10・A11・A12: やりとり欄。選手と保護者の文は英訳、コーチの文とメニューは和訳。
func TestCoachingTranslationMessages(t *testing.T) {
	fakeCoachingCloudinary(t)
	ai := &fakeTranslator{}
	h := setupTranslation(t, ai, nil)
	tc, tu, id := activeContract(t, h)
	var coachID uuid.UUID
	queryRowArgs(t, `SELECT coach_id FROM coaching_contracts WHERE id=$1`, []any{id}, &coachID)
	setLocale(t, coachID, "en")
	path := "/v1/coaching/contracts/" + id + "/messages"
	userBody := "今週は流しを5本やりました"
	rec := doJSON(t, h, http.MethodPost, path, tu, map[string]any{"body": userBody})
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"body_en":`+jsonText("EN<"+userBody+">")) || !strings.Contains(rec.Body.String(), `"body_ja":null`) {
		t.Fatalf("user send %d %s", rec.Code, rec.Body.String())
	}
	menu := map[string]any{"title": "Week 1", "period": "12/1-12/7", "days": []map[string]any{{"day": "Mon", "content": "Strides 100m x5"}, {"day": "Wed", "content": "A-skip 3 sets"}, {"day": "Sat", "content": "Rest"}}}
	coachBody := "Here is your menu for week 1."
	if rec := doJSON(t, h, http.MethodPost, path, tc, map[string]any{"body": coachBody, "menu": menu}); rec.Code != http.StatusCreated {
		t.Fatalf("coach send %d %s", rec.Code, rec.Body.String())
	}
	if ai.calls() != 3 {
		t.Fatalf("calls %d, want 3 (user body, coach body, menu)", ai.calls())
	}
	// 保護者の文（今の API では送れない既存の行）は backfill で英訳される。
	guardianBody := "保護者です。よろしくお願いします"
	execSQLArgs(t, `INSERT INTO coaching_messages (id, contract_id, sender_id, sender_role, body) VALUES ($1, $2, NULL, 'guardian', $3)`, uuid.New(), id, guardianBody)
	rec = doJSON(t, h, http.MethodPost, "/v1/admin/coaching-translations/backfill", adminToken(t, h), nil)
	if !strings.Contains(rec.Body.String(), `"translated":1`) || !strings.Contains(rec.Body.String(), `"skipped":2`) || !strings.Contains(rec.Body.String(), `"failed":0`) {
		t.Fatalf("backfill %s", rec.Body.String())
	}

	msgs, raw := messageTranslations(t, h, tu, id)
	if len(msgs) != 3 {
		t.Fatalf("messages %s", raw)
	}
	byRole := map[string]messageTranslationView{}
	for _, m := range msgs {
		byRole[m.SenderRole] = m
	}
	if m := byRole["requester"]; m.Body != userBody || m.BodyEn == nil || *m.BodyEn != "EN<"+userBody+">" || m.BodyJa != nil {
		t.Fatalf("requester message %+v", m)
	}
	if m := byRole["guardian"]; m.BodyEn == nil || *m.BodyEn != "EN<"+guardianBody+">" || m.BodyJa != nil {
		t.Fatalf("guardian message %+v", m)
	}
	m := byRole["coach"]
	if m.BodyJa == nil || *m.BodyJa != "JA<"+coachBody+">" || m.BodyEn != nil || len(m.MenuJa) == 0 {
		t.Fatalf("coach message %+v", m)
	}
	var menuJa struct {
		Title string `json:"title"`
		Days  []struct {
			Day     string `json:"day"`
			Content string `json:"content"`
		} `json:"days"`
	}
	_ = json.Unmarshal(m.MenuJa, &menuJa)
	if menuJa.Title != "JA<Week 1>" || len(menuJa.Days) != 3 || menuJa.Days[0].Day != "JA<Mon>" || menuJa.Days[1].Content != "JA<A-skip 3 sets>" || menuJa.Days[2].Content != "JA<Rest>" {
		t.Fatalf("menu_ja %s", m.MenuJa)
	}
	var hash string
	queryRowArgs(t, `SELECT source_hash FROM coaching_message_translations WHERE message_id=$1`, []any{m.ID}, &hash)
	var storedMenu string
	queryRowArgs(t, `SELECT menu::text FROM coaching_messages WHERE id=$1`, []any{m.ID}, &storedMenu)
	if hash != sourceHash(coachBody, storedMenu) {
		t.Fatalf("message hash %s", hash)
	}

	// A6: メニューの日数が合わない応答は保存しない。送信は成功する。
	ai.mu.Lock()
	ai.override = func(p string) string {
		if strings.Contains(p, "JSON training menu") {
			return `{"title":"x","period":"","note":"","days":[{"day":"a","content":"b"}]}`
		}
		return ""
	}
	ai.mu.Unlock()
	rec = doJSON(t, h, http.MethodPost, path, tc, map[string]any{"menu": menu})
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"menu_ja":null`) {
		t.Fatalf("menu mismatch send %d %s", rec.Code, rec.Body.String())
	}

	// A12: メッセージを消すと訳も消える。
	execSQLArgs(t, `DELETE FROM coaching_messages WHERE id=$1`, m.ID)
	var left int
	queryRowArgs(t, `SELECT count(*)::int FROM coaching_message_translations WHERE message_id=$1`, []any{m.ID}, &left)
	if left != 0 {
		t.Fatal("message translation survived delete")
	}
}

// A11: backfill は管理者だけが呼べる。1件失敗しても残りを続け、件数を返す。
func TestCoachingTranslationBackfill(t *testing.T) {
	fakeCoachingCloudinary(t)
	ai := &fakeTranslator{}
	h := setupTranslation(t, ai, func(c *config.Config) { c.CoachingTranslationEnabled = true })
	tc, tu, id := activeContract(t, h)
	// まだ ja のコーチのうちに送る（訳は作られない）。
	for _, b := range []string{"一通目です", "二通目です", "三通目です"} {
		if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts/"+id+"/messages", tu, map[string]any{"body": b}); rec.Code != http.StatusCreated {
			t.Fatalf("send %d", rec.Code)
		}
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts/"+id+"/messages", tc, map[string]any{"body": "コーチの返信です"}); rec.Code != http.StatusCreated {
		t.Fatalf("coach send %d", rec.Code)
	}
	if ai.calls() != 0 {
		t.Fatal("ja coach translated")
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching-translations/backfill", tu, nil); rec.Code != http.StatusForbidden && rec.Code != http.StatusNotFound {
		t.Fatalf("non-admin backfill %d", rec.Code)
	}
	tadmin := adminToken(t, h)
	// ja のコーチのペアは対象外。
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching-translations/backfill", tadmin, nil); !strings.Contains(rec.Body.String(), `"failed":0,"skipped":0,"translated":0`) || ai.calls() != 0 {
		t.Fatalf("ja backfill %s calls=%d", rec.Body.String(), ai.calls())
	}
	var coachID uuid.UUID
	queryRowArgs(t, `SELECT coach_id FROM coaching_contracts WHERE id=$1`, []any{id}, &coachID)
	setLocale(t, coachID, "en")
	// 2通目だけ失敗させる。残りは続けて処理される。
	ai.mu.Lock()
	ai.override = func(p string) string {
		if strings.Contains(p, "二通目です") {
			return "not json"
		}
		return ""
	}
	ai.mu.Unlock()
	rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching-translations/backfill", tadmin, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"failed":1,"skipped":0,"translated":3`) {
		t.Fatalf("backfill %d %s", rec.Code, rec.Body.String())
	}
	ai.mu.Lock()
	ai.override = nil
	ai.mu.Unlock()
	rec = doJSON(t, h, http.MethodPost, "/v1/admin/coaching-translations/backfill", tadmin, nil)
	if !strings.Contains(rec.Body.String(), `"failed":0,"skipped":3,"translated":1`) {
		t.Fatalf("second backfill %s", rec.Body.String())
	}
	msgs, raw := messageTranslations(t, h, tc, id)
	for _, m := range msgs {
		if (m.SenderRole == "requester" && m.BodyEn == nil) || (m.SenderRole == "coach" && m.BodyJa == nil) {
			t.Fatalf("missing translation %s", raw)
		}
	}
}
