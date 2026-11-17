package httpx_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

const coachTranslationReply = `{"nickname_en":"Test Coach","hitokoto_en":"Happy to help","achievements_en":"National semifinalist","best_record_en":"100m 10.95 (2025)"}`

func coachTranslationHash(nickname, hitokoto, achievements, bestRecord string) string {
	sum := sha256.Sum256([]byte(nickname + "\x00" + hitokoto + "\x00" + achievements + "\x00" + bestRecord))
	return hex.EncodeToString(sum[:])
}

func setupCoachTranslation(t *testing.T, ai *fakeClaude) (http.Handler, string) {
	t.Helper()
	endpoint := ai.serve(t)
	h := setupConfig(t, func(c *config.Config) {
		c.CoachProfileTranslationEnabled = true
		c.AnthropicAPIKey = "test"
		c.AnthropicEndpoint = endpoint
	})
	return h, endpoint
}

func applyCoachWithHitokoto(t *testing.T, h http.Handler, ta string) {
	t.Helper()
	body := coachApplicationBody()
	body["hitokoto"] = "よろしくお願いします"
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", ta, body); rec.Code != http.StatusOK {
		t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
	}
}

func storedTranslation(t *testing.T, coach uuid.UUID) (hash, nicknameEn string, count int) {
	t.Helper()
	queryRowArgs(t, `SELECT coalesce((SELECT source_hash FROM coach_profile_translations WHERE coach_id=$1),''), coalesce((SELECT nickname_en FROM coach_profile_translations WHERE coach_id=$1),''), (SELECT count(*)::int FROM coach_profile_translations WHERE coach_id=$1)`, []any{coach}, &hash, &nicknameEn, &count)
	return
}

// 項目1・3・8: 承認時に訳と正しい source_hash が保存され、公開APIで返る。再実行ではAPIを呼ばない。
func TestCoachTranslationOnApprove(t *testing.T) {
	ai := &fakeClaude{reply: coachTranslationReply}
	h, _ := setupCoachTranslation(t, ai)
	applicant := uuid.New()
	ta := token(t, applicant)
	applyCoachWithHitokoto(t, h, ta)
	withCoachPhoto(t, applicant)
	admin := uuid.New()
	tadmin := token(t, admin)
	doJSON(t, h, http.MethodGet, "/v1/me", tadmin, nil)
	setAdmin(t, admin)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+applicant.String()+"/approve", tadmin, nil); rec.Code != http.StatusOK {
		t.Fatalf("approve %d %s", rec.Code, rec.Body.String())
	}
	hash, nicknameEn, count := storedTranslation(t, applicant)
	want := coachTranslationHash("テストコーチ", "よろしくお願いします", "インターハイ 100m 準決勝", "100m 10秒95（2025年）")
	if count != 1 || hash != want || nicknameEn != "Test Coach" {
		t.Fatalf("stored hash=%q nickname_en=%q count=%d want hash %q", hash, nicknameEn, count, want)
	}
	rec := doJSON(t, h, http.MethodGet, "/v1/coaches", "", nil)
	body := rec.Body.String()
	for _, want := range []string{`"nickname_en":"Test Coach"`, `"hitokoto_en":"Happy to help"`, `"achievements_en":"National semifinalist"`, `"best_record_en":"100m 10.95 (2025)"`, `"nickname":"テストコーチ"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("public coaches missing %s: %s", want, body)
		}
	}
	ai.mu.Lock()
	calls := len(ai.prompts)
	ai.mu.Unlock()
	if calls != 1 {
		t.Fatalf("claude calls = %d, want 1", calls)
	}
	// backfill 再実行では API を呼ばず skipped になる（項目3・9の一部）。
	rec = doJSON(t, h, http.MethodPost, "/v1/admin/coach-profile-translations/backfill", tadmin, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"skipped":1`) {
		t.Fatalf("backfill %d %s", rec.Code, rec.Body.String())
	}
	ai.mu.Lock()
	calls = len(ai.prompts)
	ai.mu.Unlock()
	if calls != 1 {
		t.Fatalf("claude calls after backfill = %d, want 1", calls)
	}
}

// 項目2: プロフィール変更で訳が再生成され source_hash が更新される。
func TestCoachTranslationOnProfileUpdate(t *testing.T) {
	ai := &fakeClaude{reply: coachTranslationReply}
	h, _ := setupCoachTranslation(t, ai)
	applicant := uuid.New()
	ta := token(t, applicant)
	applyCoachWithHitokoto(t, h, ta)
	withCoachPhoto(t, applicant)
	admin := uuid.New()
	tadmin := token(t, admin)
	doJSON(t, h, http.MethodGet, "/v1/me", tadmin, nil)
	setAdmin(t, admin)
	doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+applicant.String()+"/approve", tadmin, nil)
	ai.reply = `{"nickname_en":"Test Coach","hitokoto_en":"Updated note","achievements_en":"National semifinalist","best_record_en":"100m 10.95 (2025)"}`
	edit := profileUpdateBody()
	edit["hitokoto"] = "新しいひとこと"
	if rec := doJSON(t, h, http.MethodPut, "/v1/coach/me/profile", ta, edit); rec.Code != http.StatusOK {
		t.Fatalf("update %d %s", rec.Code, rec.Body.String())
	}
	hash, _, _ := storedTranslation(t, applicant)
	want := coachTranslationHash("テストコーチ", "新しいひとこと", "インターハイ 100m 準決勝", "100m 10秒95（2025年）")
	if hash != want {
		t.Fatalf("hash after update = %q want %q", hash, want)
	}
	if body := doJSON(t, h, http.MethodGet, "/v1/coaches", "", nil).Body.String(); !strings.Contains(body, `"hitokoto_en":"Updated note"`) {
		t.Fatalf("public after update %s", body)
	}
}

// 項目4・5: 生成失敗でも承認・保存は成功し、古い訳は公開されない。
func TestCoachTranslationFailureFailOpen(t *testing.T) {
	ai := &fakeClaude{status: http.StatusInternalServerError}
	h, _ := setupCoachTranslation(t, ai)
	applicant := uuid.New()
	ta := token(t, applicant)
	applyCoachWithHitokoto(t, h, ta)
	withCoachPhoto(t, applicant)
	admin := uuid.New()
	tadmin := token(t, admin)
	doJSON(t, h, http.MethodGet, "/v1/me", tadmin, nil)
	setAdmin(t, admin)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+applicant.String()+"/approve", tadmin, nil); rec.Code != http.StatusOK {
		t.Fatalf("approve with failing AI = %d %s", rec.Code, rec.Body.String())
	}
	if _, _, count := storedTranslation(t, applicant); count != 0 {
		t.Fatalf("translation stored on failure")
	}
	if body := doJSON(t, h, http.MethodGet, "/v1/coaches", "", nil).Body.String(); !strings.Contains(body, `"nickname_en":null`) {
		t.Fatalf("expected null *_en: %s", body)
	}
	// 成功させてから原文を変えて失敗させると、古い訳は出ない。
	ai.status = 0
	ai.reply = coachTranslationReply
	if rec := doJSON(t, h, http.MethodPut, "/v1/coach/me/profile", ta, profileUpdateBody()); rec.Code != http.StatusOK {
		t.Fatalf("update1 %d", rec.Code)
	}
	ai.status = http.StatusInternalServerError
	edit := profileUpdateBody()
	edit["hitokoto"] = "変わったひとこと"
	if rec := doJSON(t, h, http.MethodPut, "/v1/coach/me/profile", ta, edit); rec.Code != http.StatusOK {
		t.Fatalf("update with failing AI = %d %s", rec.Code, rec.Body.String())
	}
	body := doJSON(t, h, http.MethodGet, "/v1/coaches", "", nil).Body.String()
	if strings.Contains(body, `"nickname_en":"Test Coach"`) || !strings.Contains(body, `"nickname_en":null`) {
		t.Fatalf("stale translation leaked: %s", body)
	}
	// 不正JSON・上限超えでも保存は成功する。
	ai.status = 0
	ai.reply = "not json"
	edit["hitokoto"] = "また変わった"
	if rec := doJSON(t, h, http.MethodPut, "/v1/coach/me/profile", ta, edit); rec.Code != http.StatusOK {
		t.Fatalf("update invalid json = %d", rec.Code)
	}
	ai.reply = `{"nickname_en":"` + strings.Repeat("a", 101) + `","hitokoto_en":"","achievements_en":"","best_record_en":""}`
	edit["hitokoto"] = "さらに変わった"
	if rec := doJSON(t, h, http.MethodPut, "/v1/coach/me/profile", ta, edit); rec.Code != http.StatusOK {
		t.Fatalf("update over limit = %d", rec.Code)
	}
}

// 項目6: フラグOFF・キー空では呼ばず null。
func TestCoachTranslationFlagOffAndNoKey(t *testing.T) {
	ai := &fakeClaude{reply: coachTranslationReply}
	endpoint := ai.serve(t)
	h := setupConfig(t, func(c *config.Config) {
		c.AnthropicAPIKey = "test"
		c.AnthropicEndpoint = endpoint
	})
	applicant := uuid.New()
	ta := token(t, applicant)
	applyApproveCoach(t, h, applicant, ta)
	if body := doJSON(t, h, http.MethodGet, "/v1/coaches", "", nil).Body.String(); !strings.Contains(body, `"nickname_en":null`) {
		t.Fatalf("flag off body %s", body)
	}
	h2 := setupConfig(t, func(c *config.Config) {
		c.CoachProfileTranslationEnabled = true
		c.AnthropicEndpoint = endpoint
	})
	applicant2 := uuid.New()
	ta2 := token(t, applicant2)
	applyApproveCoach(t, h2, applicant2, ta2)
	ai.mu.Lock()
	calls := len(ai.prompts)
	ai.mu.Unlock()
	if calls != 0 {
		t.Fatalf("claude called %d times with flag off / no key", calls)
	}
}

// 項目7: 予約上限を超えたら生成を諦め、承認は成功する。
func TestCoachTranslationBudgetExhausted(t *testing.T) {
	ai := &fakeClaude{reply: coachTranslationReply}
	h, _ := setupCoachTranslation(t, ai)
	execSQLArgs(t, `INSERT INTO coach_profile_translation_attempts (attempted_at) SELECT clock_timestamp() FROM generate_series(1,100)`)
	applicant := uuid.New()
	ta := token(t, applicant)
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", ta, coachApplicationBody()); rec.Code != http.StatusOK {
		t.Fatalf("apply %d", rec.Code)
	}
	withCoachPhoto(t, applicant)
	admin := uuid.New()
	tadmin := token(t, admin)
	doJSON(t, h, http.MethodGet, "/v1/me", tadmin, nil)
	setAdmin(t, admin)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+applicant.String()+"/approve", tadmin, nil); rec.Code != http.StatusOK {
		t.Fatalf("approve over budget = %d %s", rec.Code, rec.Body.String())
	}
	ai.mu.Lock()
	calls := len(ai.prompts)
	ai.mu.Unlock()
	if calls != 0 {
		t.Fatalf("calls over budget = %d", calls)
	}
}

// 項目8（サポート一覧・プラン詳細）・9（backfill認可と件数）。
func TestCoachTranslationSupportAndPlanAndBackfill(t *testing.T) {
	ai := &fakeClaude{reply: coachTranslationReply}
	h, _ := setupCoachTranslation(t, ai)
	consultID, _ := consultCoach(t, h)
	_, _, planID := planCoach(t, h, 5)
	if body := doJSON(t, h, http.MethodGet, "/v1/coaches/support-plan", "", nil).Body.String(); !strings.Contains(body, `"nickname_en":"Test Coach"`) {
		t.Fatalf("support-plan nickname_en missing: %s", body)
	}
	_ = consultID
	if body := doJSON(t, h, http.MethodGet, "/v1/coaching/plans/"+planID, "", nil).Body.String(); !strings.Contains(body, `"nickname_en":"Test Coach"`) {
		t.Fatalf("plan nickname_en missing: %s", body)
	}
	// 管理者以外は backfill を呼べない。
	plain := token(t, uuid.New())
	doJSON(t, h, http.MethodGet, "/v1/me", plain, nil)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coach-profile-translations/backfill", plain, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin backfill = %d %s", rec.Code, rec.Body.String())
	}
	// 訳の無いコーチを作り（直接行を消す）、backfill で translated になる。片方が失敗しても他は進む。
	execSQLArgs(t, `DELETE FROM coach_profile_translations`)
	ai.status = http.StatusInternalServerError
	admin := adminToken(t, h)
	rec := doJSON(t, h, http.MethodPost, "/v1/admin/coach-profile-translations/backfill", admin, nil)
	var counts struct {
		Translated int `json:"translated"`
		Skipped    int `json:"skipped"`
		Failed     int `json:"failed"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &counts)
	if rec.Code != http.StatusOK || counts.Failed == 0 || counts.Translated != 0 {
		t.Fatalf("backfill failing = %d %+v %s", rec.Code, counts, rec.Body.String())
	}
	ai.status = 0
	rec = doJSON(t, h, http.MethodPost, "/v1/admin/coach-profile-translations/backfill", admin, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &counts)
	if counts.Translated == 0 || counts.Failed != 0 {
		t.Fatalf("backfill ok = %+v %s", counts, rec.Body.String())
	}
}

// 項目10: コーチのプロフィール行を消すと訳も消える（CASCADE）。
func TestCoachTranslationCascade(t *testing.T) {
	ai := &fakeClaude{reply: coachTranslationReply}
	h, _ := setupCoachTranslation(t, ai)
	applicant := uuid.New()
	ta := token(t, applicant)
	applyApproveCoach(t, h, applicant, ta)
	if _, _, count := storedTranslation(t, applicant); count != 1 {
		t.Fatalf("translation count = %d", count)
	}
	execSQLArgs(t, `DELETE FROM coach_profiles WHERE profile_id=$1`, applicant)
	if _, _, count := storedTranslation(t, applicant); count != 0 {
		t.Fatalf("translation remains after cascade, count=%d", count)
	}
}

const coachTranslationContactReply = `{"nickname_en":"Test Coach","hitokoto_en":"DM me @coach_taro or 090-0000-0000","achievements_en":"See https://example.com/coach","best_record_en":"100m 10.95 (2025)"}`

func storedTranslationFields(t *testing.T, coach uuid.UUID) (nickname, hitokoto, achievements, bestRecord string) {
	t.Helper()
	queryRowArgs(t, `SELECT nickname_en, hitokoto_en, achievements_en, best_record_en FROM coach_profile_translations WHERE coach_id=$1`, []any{coach}, &nickname, &hitokoto, &achievements, &bestRecord)
	return
}

func assertNoContactShape(t *testing.T, body string) {
	t.Helper()
	for _, bad := range []string{"@coach_taro", "090-0000-0000", "https://example.com"} {
		if strings.Contains(body, bad) {
			t.Fatalf("response contains %q: %s", bad, body)
		}
	}
}

// 承認経路: 連絡先の形を含む項目だけが空で保存され、承認は成功し、公開APIにも出ない。
func TestCoachTranslationContactLikeFieldsDroppedOnApprove(t *testing.T) {
	ai := &fakeClaude{reply: coachTranslationContactReply}
	h, _ := setupCoachTranslation(t, ai)
	applicant := uuid.New()
	ta := token(t, applicant)
	applyCoachWithHitokoto(t, h, ta)
	withCoachPhoto(t, applicant)
	admin := uuid.New()
	tadmin := token(t, admin)
	doJSON(t, h, http.MethodGet, "/v1/me", tadmin, nil)
	setAdmin(t, admin)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+applicant.String()+"/approve", tadmin, nil); rec.Code != http.StatusOK {
		t.Fatalf("approve %d %s", rec.Code, rec.Body.String())
	}
	nick, hito, ach, best := storedTranslationFields(t, applicant)
	if nick != "Test Coach" || hito != "" || ach != "" || best != "100m 10.95 (2025)" {
		t.Fatalf("stored = %q %q %q %q", nick, hito, ach, best)
	}
	rec := doJSON(t, h, http.MethodGet, "/v1/coaches/"+applicant.String(), "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get coach %d %s", rec.Code, rec.Body.String())
	}
	assertNoContactShape(t, rec.Body.String())
	assertNoContactShape(t, doJSON(t, h, http.MethodGet, "/v1/coaches", "", nil).Body.String())
	ai.mu.Lock()
	calls := len(ai.prompts)
	ai.mu.Unlock()
	if calls != 1 {
		t.Fatalf("claude calls = %d, want 1", calls)
	}
}

// プロフィール変更経路と backfill 経路: どちらも同じ判定が効き、source_hash は新しい原文のものになる。
func TestCoachTranslationContactLikeFieldsDroppedOnUpdateAndBackfill(t *testing.T) {
	ai := &fakeClaude{reply: coachTranslationReply}
	h, _ := setupCoachTranslation(t, ai)
	applicant := uuid.New()
	ta := token(t, applicant)
	applyCoachWithHitokoto(t, h, ta)
	withCoachPhoto(t, applicant)
	admin := uuid.New()
	tadmin := token(t, admin)
	doJSON(t, h, http.MethodGet, "/v1/me", tadmin, nil)
	setAdmin(t, admin)
	doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+applicant.String()+"/approve", tadmin, nil)

	ai.reply = coachTranslationContactReply
	edit := profileUpdateBody()
	edit["hitokoto"] = "新しいひとこと"
	if rec := doJSON(t, h, http.MethodPut, "/v1/coach/me/profile", ta, edit); rec.Code != http.StatusOK {
		t.Fatalf("update %d %s", rec.Code, rec.Body.String())
	}
	hash, _, _ := storedTranslation(t, applicant)
	if want := coachTranslationHash("テストコーチ", "新しいひとこと", "インターハイ 100m 準決勝", "100m 10秒95（2025年）"); hash != want {
		t.Fatalf("hash after update = %q want %q", hash, want)
	}
	nick, hito, ach, best := storedTranslationFields(t, applicant)
	if nick != "Test Coach" || hito != "" || ach != "" || best != "100m 10.95 (2025)" {
		t.Fatalf("stored after update = %q %q %q %q", nick, hito, ach, best)
	}
	assertNoContactShape(t, doJSON(t, h, http.MethodGet, "/v1/coaches/"+applicant.String(), "", nil).Body.String())

	// backfill: 訳の行を消してから作り直しても同じ判定が効く。
	execSQLArgs(t, `DELETE FROM coach_profile_translations`)
	rec := doJSON(t, h, http.MethodPost, "/v1/admin/coach-profile-translations/backfill", tadmin, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"translated":1`) {
		t.Fatalf("backfill %d %s", rec.Code, rec.Body.String())
	}
	nick, hito, ach, best = storedTranslationFields(t, applicant)
	if nick != "Test Coach" || hito != "" || ach != "" || best != "100m 10.95 (2025)" {
		t.Fatalf("stored after backfill = %q %q %q %q", nick, hito, ach, best)
	}
	assertNoContactShape(t, doJSON(t, h, http.MethodGet, "/v1/coaches", "", nil).Body.String())
}

// approveCoachForTranslation は申請から承認までを行い、承認後の AI 呼び出し回数を返す。
// pre は承認前（コーチ ID が決まった後）に予約台帳へ行を足すための関数。
func approveCoachForTranslation(t *testing.T, h http.Handler, ai *fakeClaude, pre func(coach uuid.UUID)) (uuid.UUID, int) {
	t.Helper()
	applicant := uuid.New()
	ta := token(t, applicant)
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", ta, coachApplicationBody()); rec.Code != http.StatusOK {
		t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
	}
	withCoachPhoto(t, applicant)
	if pre != nil {
		pre(applicant)
	}
	admin := uuid.New()
	tadmin := token(t, admin)
	doJSON(t, h, http.MethodGet, "/v1/me", tadmin, nil)
	setAdmin(t, admin)
	ai.mu.Lock()
	before := len(ai.prompts)
	ai.mu.Unlock()
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+applicant.String()+"/approve", tadmin, nil); rec.Code != http.StatusOK {
		t.Fatalf("approve %d %s", rec.Code, rec.Body.String())
	}
	ai.mu.Lock()
	defer ai.mu.Unlock()
	return applicant, len(ai.prompts) - before
}

// 堅牢化 細目1: migration 後に coach_id 列と (coach_id, attempted_at) の索引がある。
func TestCoachTranslationAttemptsHasCoachColumnAndIndex(t *testing.T) {
	_ = setup(t)
	var cols, idx int
	queryRowArgs(t, `SELECT (SELECT count(*)::int FROM information_schema.columns WHERE table_name='coach_profile_translation_attempts' AND column_name='coach_id' AND data_type='uuid' AND is_nullable='YES'),
		(SELECT count(*)::int FROM pg_indexes WHERE tablename='coach_profile_translation_attempts' AND indexdef LIKE '%(coach_id, attempted_at)%')`, nil, &cols, &idx)
	if cols != 1 || idx != 1 {
		t.Fatalf("coach_id column=%d index=%d", cols, idx)
	}
}

// 細目2: 予約すると試行の行にコーチ ID が入る。失敗した試行も行（枠）として残る（細目5）。
func TestCoachTranslationAttemptRecordsCoachIDEvenOnFailure(t *testing.T) {
	ai := &fakeClaude{status: http.StatusInternalServerError}
	h, _ := setupCoachTranslation(t, ai)
	coach, _ := approveCoachForTranslation(t, h, ai, nil)
	var n int
	queryRowArgs(t, `SELECT count(*)::int FROM coach_profile_translation_attempts WHERE coach_id=$1`, []any{coach}, &n)
	if n != 1 {
		t.Fatalf("attempt rows for coach after failed call = %d, want 1", n)
	}
}

// 細目3・5: コーチ別の日上限（20）に達したコーチだけ止まり、他のコーチは通る。
func TestCoachTranslationPerCoachDailyLimit(t *testing.T) {
	ai := &fakeClaude{reply: coachTranslationReply}
	h, _ := setupCoachTranslation(t, ai)
	_, calls := approveCoachForTranslation(t, h, ai, func(c uuid.UUID) {
		execSQLArgs(t, `INSERT INTO coach_profile_translation_attempts (attempted_at, coach_id) SELECT clock_timestamp() - interval '2 hours', $1 FROM generate_series(1,20)`, c)
	})
	if calls != 0 {
		t.Fatalf("calls for coach at daily limit = %d, want 0", calls)
	}
	// 別のコーチは全体上限（現在 20 件）の範囲で通る。
	_, calls = approveCoachForTranslation(t, h, ai, nil)
	if calls != 1 {
		t.Fatalf("calls for other coach = %d, want 1", calls)
	}
}

// 細目3: コーチ別の分上限（5）。
func TestCoachTranslationPerCoachMinuteLimit(t *testing.T) {
	ai := &fakeClaude{reply: coachTranslationReply}
	h, _ := setupCoachTranslation(t, ai)
	_, calls := approveCoachForTranslation(t, h, ai, func(c uuid.UUID) {
		execSQLArgs(t, `INSERT INTO coach_profile_translation_attempts (attempted_at, coach_id) SELECT clock_timestamp(), $1 FROM generate_series(1,5)`, c)
	})
	if calls != 0 {
		t.Fatalf("calls for coach at minute limit = %d, want 0", calls)
	}
}

// 細目4: 全体上限に達していれば、コーチ別枠が残っていても止まる。
func TestCoachTranslationGlobalLimitStillApplies(t *testing.T) {
	ai := &fakeClaude{reply: coachTranslationReply}
	h, _ := setupCoachTranslation(t, ai)
	// 日上限：他コーチ 10 人分で全体 100 件。
	execSQLArgs(t, `INSERT INTO coach_profile_translation_attempts (attempted_at, coach_id) SELECT clock_timestamp() - interval '2 hours', gen_random_uuid() FROM generate_series(1,100)`)
	if _, calls := approveCoachForTranslation(t, h, ai, nil); calls != 0 {
		t.Fatalf("calls at global daily limit = %d, want 0", calls)
	}
	// 分上限：直近1分に別コーチ分が 10 件。
	execSQLArgs(t, `TRUNCATE coach_profile_translation_attempts`)
	execSQLArgs(t, `INSERT INTO coach_profile_translation_attempts (attempted_at, coach_id) SELECT clock_timestamp(), gen_random_uuid() FROM generate_series(1,10)`)
	if _, calls := approveCoachForTranslation(t, h, ai, nil); calls != 0 {
		t.Fatalf("calls at global minute limit = %d, want 0", calls)
	}
}

// 細目6: coach_id が NULL の既存行は全体にだけ数え、コーチ別には入らない。
func TestCoachTranslationNullCoachRowsCountOnlyGlobally(t *testing.T) {
	ai := &fakeClaude{reply: coachTranslationReply}
	h, _ := setupCoachTranslation(t, ai)
	execSQLArgs(t, `INSERT INTO coach_profile_translation_attempts (attempted_at) SELECT clock_timestamp() - interval '2 hours' FROM generate_series(1,50)`)
	var nulls int
	queryRowArgs(t, `SELECT count(*)::int FROM coach_profile_translation_attempts WHERE coach_id IS NULL`, nil, &nulls)
	if nulls != 50 {
		t.Fatalf("null rows = %d", nulls)
	}
	// 50 件はコーチ別（20）を超えるが、コーチ別には入らないので通る。
	if _, calls := approveCoachForTranslation(t, h, ai, nil); calls != 1 {
		t.Fatalf("calls with 50 NULL rows = %d, want 1", calls)
	}
	// 全体では数える：NULL 行 50 + 他コーチ 50 で上限。
	execSQLArgs(t, `INSERT INTO coach_profile_translation_attempts (attempted_at, coach_id) SELECT clock_timestamp() - interval '2 hours', gen_random_uuid() FROM generate_series(1,49)`)
	if _, calls := approveCoachForTranslation(t, h, ai, nil); calls != 0 {
		t.Fatalf("calls at global limit via NULL rows = %d, want 0", calls)
	}
}
