package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/jst"
	"github.com/hinata/track-talk/apps/api/internal/push"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// 1ヶ月プランの無料相談。
// 選手は構造化フォームで相談し、コーチは 48 時間以内に提案か辞退を返す。
// 契約は選手が提案を受けたときに作り、以降は既存のサポートプランの流れ（コーチの引受・保護者同意・カード支払い）に接続する。
// 相談の場はチャットにしない。フォームの往復だけで進める。

// コーチが提案か辞退を返す期限。動画添削の返却期限（24時間）の2倍。引受判断の90分とは別のもの。
const supportConsultRespondWindow = 48 * time.Hour

// 聞き直しは1つの相談につき3回まで。
const maxSupportConsultQuestions = 3

var errConsultState = errors.New("support consult state")

// フォームの選択肢。web の lib/support-consult.ts と合わせる。
var (
	consultTimingHints   = []string{"within_1m", "1_3m", "3_6m", "over_6m"}
	consultGoals         = []string{"pb", "final", "qualify", "podium", "experience"}
	consultPBWhen        = []string{"within_3m", "within_6m", "within_1y", "older"}
	consultForms         = []string{"good", "normal", "poor"}
	consultMenuBy        = []string{"self", "coach", "other_person"}
	consultMinutes       = []int{30, 60, 90, 120, 150}
	consultAccess        = []string{"yes", "sometimes", "no"}
	consultInjuryParts   = []string{"foot_ankle", "lower_leg", "knee", "thigh", "hip_groin", "back", "upper_body", "other"}
	consultInjuryPast    = []string{"none", "minor", "major"}
	consultFocus         = []string{"start", "acceleration", "midrace", "finish", "ground_contact", "arm_swing", "posture", "pitch_stride", "blocks", "other"}
	consultVideoReply    = []string{"none", "sometimes", "often"}
	consultStartTimings  = []string{"asap", "date"}
	consultBudgets       = []string{"under_5000", "5000_10000", "10000_20000", "over_20000"}
	consultRecordPattern = regexp.MustCompile(`^[0-9]{1,3}([.:][0-9]{1,2}){0,2}$`)
)

// consultAnswers は無料相談フォームの回答。自由記述は、大会名・一番の悩み・怪我の状態・コーチに聞きたいことだけ。
type consultAnswers struct {
	// A. 目標の大会
	GoalDecided     *bool    `json:"goal_decided"`
	CompetitionName string   `json:"competition_name,omitempty"`
	CompetitionDate string   `json:"competition_date,omitempty"`
	TimingHint      string   `json:"timing_hint,omitempty"`
	Event           string   `json:"event"`
	Goals           []string `json:"goals"`
	TargetRecord    string   `json:"target_record,omitempty"`
	// B. いまの実力
	PersonalBest string `json:"personal_best"`
	PBWhen       string `json:"pb_when"`
	Form         string `json:"form"`
	// C. 練習環境
	Club           *bool  `json:"club"`
	HasCoach       *bool  `json:"has_coach"`
	MenuBy         string `json:"menu_by"`
	PracticeDays   int    `json:"practice_days"`
	SessionMinutes int    `json:"session_minutes"`
	TrackAccess    string `json:"track_access"`
	WeightsAccess  string `json:"weights_access"`
	CanFilm        string `json:"can_film"`
	// D. 体と制約
	InjuryNow      *bool    `json:"injury_now"`
	InjuryParts    []string `json:"injury_parts"`
	InjuryNote     string   `json:"injury_note,omitempty"`
	InjuryPastYear string   `json:"injury_past_year"`
	RestPeriod     *bool    `json:"rest_period"`
	// E. 直したいこと
	Focus     []string `json:"focus"`
	MainWorry string   `json:"main_worry"`
	HasVideo  *bool    `json:"has_video"`
	// F. 希望するサポート
	WantMenu        *bool  `json:"want_menu"`
	WantVideoReply  string `json:"want_video_reply"`
	WantPeak        *bool  `json:"want_peak"`
	StartTiming     string `json:"start_timing"`
	StartDate       string `json:"start_date,omitempty"`
	Budget          string `json:"budget,omitempty"`
	QuestionToCoach string `json:"question_to_coach,omitempty"`
}

func subsetOf(values, allowed []string) bool {
	for _, v := range values {
		if !slices.Contains(allowed, v) {
			return false
		}
	}
	return true
}

// validate は回答を確かめる。coachEvents はそのコーチが相談で担当する種目。
func (a *consultAnswers) validate(coachEvents []string, today time.Time) map[string]string {
	f := map[string]string{}
	need := func(key string, ok bool, msg string) {
		if !ok {
			f[key] = msg
		}
	}
	trim := func(s *string) { *s = strings.TrimSpace(*s) }
	for _, s := range []*string{&a.CompetitionName, &a.TargetRecord, &a.PersonalBest, &a.InjuryNote, &a.MainWorry, &a.QuestionToCoach} {
		trim(s)
	}
	// A
	if a.GoalDecided == nil {
		f["goal_decided"] = "目標の大会が決まっているか選んでください"
	} else if *a.GoalDecided {
		if e := coachTextError("大会名", a.CompetitionName, 100); e != "" {
			f["competition_name"] = e
		}
		day, err := time.Parse("2006-01-02", a.CompetitionDate)
		need("competition_date", err == nil && day.Format("2006-01-02") == a.CompetitionDate && !day.Before(today), "これから開催する大会の日付を選んでください")
		a.TimingHint = ""
	} else {
		need("timing_hint", slices.Contains(consultTimingHints, a.TimingHint), "大会の時期の目安を選んでください")
		a.CompetitionName, a.CompetitionDate = "", ""
	}
	need("event", slices.Contains(coachEvents, a.Event), "このコーチが相談を受ける種目から選んでください")
	need("goals", len(a.Goals) > 0 && len(a.Goals) <= len(consultGoals) && subsetOf(a.Goals, consultGoals), "大会での目標を選んでください")
	need("target_record", a.TargetRecord == "" || consultRecordPattern.MatchString(a.TargetRecord), "目標記録は数値で入力してください（例: 11.50、4:05.20）")
	// B
	need("personal_best", consultRecordPattern.MatchString(a.PersonalBest), "自己ベストは数値で入力してください（例: 12.30、4:05.20）")
	need("pb_when", slices.Contains(consultPBWhen, a.PBWhen), "いつの記録か選んでください")
	need("form", slices.Contains(consultForms, a.Form), "最近の調子を選んでください")
	// C
	need("club", a.Club != nil, "部活所属を選んでください")
	need("has_coach", a.HasCoach != nil, "指導者の有無を選んでください")
	need("menu_by", slices.Contains(consultMenuBy, a.MenuBy), "メニューを決める人を選んでください")
	need("practice_days", a.PracticeDays >= 1 && a.PracticeDays <= 7, "週の練習日数を選んでください")
	need("session_minutes", slices.Contains(consultMinutes, a.SessionMinutes), "1回の練習時間を選んでください")
	need("track_access", slices.Contains(consultAccess, a.TrackAccess), "競技場を使えるか選んでください")
	need("weights_access", slices.Contains(consultAccess, a.WeightsAccess), "ウエイト器具を使えるか選んでください")
	need("can_film", slices.Contains(consultAccess, a.CanFilm), "動画を撮れるか選んでください")
	// D
	if a.InjuryNow == nil {
		f["injury_now"] = "今の痛み・怪我の有無を選んでください"
	} else if *a.InjuryNow {
		need("injury_parts", len(a.InjuryParts) > 0 && subsetOf(a.InjuryParts, consultInjuryParts), "痛めている部位を選んでください")
		if e := coachTextError("怪我の状態", a.InjuryNote, 200); e != "" {
			f["injury_note"] = e
		}
	} else {
		a.InjuryParts, a.InjuryNote = []string{}, ""
	}
	need("injury_past_year", slices.Contains(consultInjuryPast, a.InjuryPastYear), "過去1年の大きな怪我を選んでください")
	need("rest_period", a.RestPeriod != nil, "休む・減らす時期の有無を選んでください")
	// E
	need("focus", len(a.Focus) > 0 && subsetOf(a.Focus, consultFocus), "直したい場面や要素を選んでください")
	if e := coachTextError("一番の悩み", a.MainWorry, 500); e != "" {
		f["main_worry"] = e
	}
	need("has_video", a.HasVideo != nil, "フォーム動画の有無を選んでください")
	// F
	need("want_menu", a.WantMenu != nil, "メニュー作成の要否を選んでください")
	need("want_video_reply", slices.Contains(consultVideoReply, a.WantVideoReply), "動画返答の希望を選んでください")
	need("want_peak", a.WantPeak != nil, "大会直前の調整の要否を選んでください")
	need("start_timing", slices.Contains(consultStartTimings, a.StartTiming), "開始時期を選んでください")
	if a.StartTiming == "date" {
		day, err := time.Parse("2006-01-02", a.StartDate)
		need("start_date", err == nil && day.Format("2006-01-02") == a.StartDate && !day.Before(today), "開始したい日を選んでください")
	} else {
		a.StartDate = ""
	}
	need("budget", a.Budget == "" || slices.Contains(consultBudgets, a.Budget), "予算の目安を選び直してください")
	if utf8.RuneCountInString(a.QuestionToCoach) > 500 {
		f["question_to_coach"] = "コーチに聞きたいことは500文字以内で入力してください"
	}
	// 自由記述に連絡先は書けない。
	for key, text := range map[string]string{"competition_name": a.CompetitionName, "injury_note": a.InjuryNote, "main_worry": a.MainWorry, "question_to_coach": a.QuestionToCoach} {
		if f[key] == "" && looksLikeContact(text) {
			f[key] = "連絡先（URL・メールアドレス・SNS の ID・電話番号など）は書けません"
		}
	}
	if a.Goals == nil {
		a.Goals = []string{}
	}
	return f
}

// 受け入れ状況。コーチが手で切り替える「受付中／受付停止中」だけで決める。
// 人数の上限・満枠・残り枠は持たず、契約件数も選手には出さない。カレンダーとは連動しない。
func receptionJSON(accepting bool) map[string]any {
	status := "open"
	if !accepting {
		status = "paused"
	}
	return map[string]any{"status": status, "resume_on": nil}
}

func consultSettingsJSON(st db.SupportConsultSetting) map[string]any {
	return map[string]any{
		"accepting": st.Accepting, "events": nonNilStrings(st.Events), "description": st.Description,
		"price_min_yen": st.PriceMinYen, "price_max_yen": st.PriceMaxYen,
	}
}

// publicConsultJSON は、コーチ一覧・詳細に出す無料相談の入口。件数や残り枠は出さない。
func publicConsultJSON(st db.SupportConsultSetting) map[string]any {
	out := consultSettingsJSON(st)
	delete(out, "accepting")
	out["reception"] = receptionJSON(st.Accepting)
	return out
}

// ---- コーチ: 受付設定 ----

func (s *Server) getMyConsultSettings(w http.ResponseWriter, r *http.Request) {
	c, ok := s.approvedCoach(w, r)
	if !ok {
		return
	}
	st, err := s.q.GetSupportConsultSettings(r.Context(), c.ProfileID)
	if errors.Is(err, pgx.ErrNoRows) {
		st = db.SupportConsultSetting{CoachID: c.ProfileID, Events: []string{}}
	} else if err != nil {
		MapDBError(w, err)
		return
	}
	out := map[string]any{"settings": consultSettingsJSON(st), "fee_percent": personalFeePercent}
	if tpl, err := s.q.GetConsultTemplatePlan(r.Context(), c.ProfileID); err == nil {
		approved, aerr := s.identity.approvedPlans(r.Context(), []uuid.UUID{tpl.ID})
		out["disclosure_approved"] = aerr == nil && approved[tpl.ID]
	} else {
		out["disclosure_approved"] = false
	}
	JSON(w, http.StatusOK, out)
}

func (s *Server) saveMyConsultSettings(w http.ResponseWriter, r *http.Request) {
	c, ok := s.approvedCoach(w, r)
	if !ok {
		return
	}
	var req struct {
		Accepting   bool     `json:"accepting"`
		Events      []string `json:"events"`
		Description string   `json:"description"`
		PriceMinYen *int32   `json:"price_min_yen"`
		PriceMaxYen *int32   `json:"price_max_yen"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	req.Description = strings.TrimSpace(req.Description)
	fields := map[string]string{}
	if req.Events == nil {
		req.Events = []string{}
	}
	if !subsetOf(req.Events, c.Events) {
		fields["events"] = "自分の担当種目から選んでください"
	}
	if utf8.RuneCountInString(req.Description) > 1000 {
		fields["description"] = "説明は1000文字以内で入力してください"
	} else if req.Description != "" && looksLikeContact(req.Description) {
		fields["description"] = "連絡先（URL・メールアドレス・SNS の ID・電話番号など）は書けません"
	}
	if (req.PriceMinYen == nil) != (req.PriceMaxYen == nil) {
		fields["price_min_yen"] = "価格の目安は下限と上限をそろえて入力してください"
	} else if req.PriceMinYen != nil && (*req.PriceMinYen < minPlanPriceYen || *req.PriceMaxYen < *req.PriceMinYen || *req.PriceMaxYen > maxPlanPriceYen) {
		fields["price_min_yen"] = "価格の目安は2,000円以上で、上限は下限以上にしてください"
	}
	if req.Accepting {
		if len(req.Events) == 0 {
			fields["events"] = "相談を受ける種目を選んでください"
		}
		if req.Description == "" {
			fields["description"] = "相談で聞きたいこと・サポートできる内容を入力してください"
		}
	}
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	if req.Accepting {
		if !pledged(c) {
			Error(w, http.StatusConflict, "pledge_required", "先にパーソナル指導の誓約をしてください")
			return
		}
		if !slices.Contains(c.ServiceTypes, "competition_support") {
			FieldError(w, map[string]string{"accepting": "先に担当するサービスに1ヶ月プランを登録してください"})
			return
		}
		if !s.cfg.CoachingMessaging {
			FieldError(w, map[string]string{"accepting": "このサービスはメッセージのやりとりの準備ができるまで受け付けられません"})
			return
		}
		if !s.commerceReadyToPublish(w, r, c.ProfileID) {
			return
		}
	}
	var saved db.SupportConsultSetting
	err := s.tx(r.Context(), func(q *db.Queries) error {
		if err := q.LockCoachingPlans(r.Context(), c.ProfileID); err != nil {
			return err
		}
		var err error
		saved, err = q.SaveSupportConsultSettings(r.Context(), db.SaveSupportConsultSettingsParams{
			CoachID: c.ProfileID, Accepting: req.Accepting, Events: req.Events, Description: req.Description,
			PriceMinYen: req.PriceMinYen, PriceMaxYen: req.PriceMaxYen,
		})
		if err != nil {
			return err
		}
		if req.Accepting {
			_, err = q.CreateConsultTemplatePlan(r.Context(), db.CreateConsultTemplatePlanParams{ID: uuid.New(), CoachID: c.ProfileID})
		}
		return err
	})
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"settings": consultSettingsJSON(saved), "fee_percent": personalFeePercent})
}

// ---- 公開: 相談の入口と受け入れ状況 ----

// publicConsultEntries は、相談を受け付けているコーチごとの入口。特商法の表記が承認されたコーチだけ。
func (s *Server) publicConsultEntries(ctx context.Context, only *uuid.UUID) (map[uuid.UUID]map[string]any, error) {
	settings, err := s.q.ListPublicSupportConsultSettings(ctx, coachPledgeListedVersion)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(settings))
	for _, st := range settings {
		if only == nil || st.CoachID == *only {
			ids = append(ids, st.CoachID)
		}
	}
	out := map[uuid.UUID]map[string]any{}
	if len(ids) == 0 {
		return out, nil
	}
	templates, err := s.q.ListConsultTemplatePlans(ctx, ids)
	if err != nil {
		return nil, err
	}
	planIDs := make([]uuid.UUID, 0, len(templates))
	templateOf := map[uuid.UUID]uuid.UUID{}
	for _, t := range templates {
		planIDs = append(planIDs, t.ID)
		templateOf[t.CoachID] = t.ID
	}
	approved, err := s.identity.approvedPlans(ctx, planIDs)
	if err != nil {
		return nil, err
	}
	for _, st := range settings {
		if !slices.Contains(ids, st.CoachID) || !approved[templateOf[st.CoachID]] {
			continue
		}
		out[st.CoachID] = publicConsultJSON(st)
	}
	return out, nil
}

func (s *Server) getPublicCoachConsult(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	entries, err := s.publicConsultEntries(r.Context(), &id)
	if err != nil {
		MapDBError(w, err)
		return
	}
	entry, found := entries[id]
	if !found {
		JSON(w, http.StatusOK, map[string]any{"consult": nil})
		return
	}
	entry["fee_percent"] = personalFeePercent
	JSON(w, http.StatusOK, map[string]any{"consult": entry})
}

// ---- 選手: 相談の申込み ----

func (s *Server) consultViewJSON(c db.SupportConsult) map[string]any {
	var answers any
	_ = json.Unmarshal(c.Answers, &answers)
	return map[string]any{
		"id": c.ID, "status": c.Status, "minor": c.Minor, "event": c.Event, "answers": answers,
		"answer_by": c.AnswerBy, "contract_id": c.ContractID, "created_at": c.CreatedAt, "updated_at": c.UpdatedAt,
	}
}

func proposalJSON(p db.SupportConsultProposal) map[string]any {
	return map[string]any{
		"competition_name": p.CompetitionName, "support_start_date": p.SupportStartDate.Format("2006-01-02"),
		"competition_date": p.CompetitionDate.Format("2006-01-02"), "duration_days": proposalDays(p),
		"call_count": p.CallCount, "call_minutes": p.CallMinutes, "price_yen": p.PriceYen,
		"fee_percent": personalFeePercent, "coach_receives_yen": p.PriceYen - p.PriceYen*personalFeePercent/100,
		"content_note": p.ContentNote, "answer_to_question": p.AnswerToQuestion, "valid_until": p.ValidUntil, "valid_until_date": p.ValidUntil.In(jst.Zone).AddDate(0, 0, -1).Format("2006-01-02"),
	}
}

func proposalDays(p db.SupportConsultProposal) int {
	return int(p.CompetitionDate.Sub(p.SupportStartDate).Hours()/24) + 1
}

func (s *Server) createSupportConsult(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	if !me.CoachingDemo {
		Error(w, http.StatusForbidden, "forbidden", "パーソナル指導は準備中です")
		return
	}
	var req struct {
		CoachID uuid.UUID      `json:"coach_id"`
		Answers consultAnswers `json:"answers"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	if req.CoachID == me.ID {
		Error(w, http.StatusUnprocessableEntity, "validation_error", "自分には相談できません")
		return
	}
	entries, err := s.publicConsultEntries(r.Context(), &req.CoachID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	entry, found := entries[req.CoachID]
	if !found {
		Error(w, http.StatusNotFound, "not_found", "このコーチはいま相談を受け付けていません")
		return
	}
	if me.BirthMonth == nil {
		Error(w, http.StatusUnprocessableEntity, "birth_month_required", "先に生まれた年月を登録してください")
		return
	}
	now := time.Now()
	today := availabilityToday(now)
	coachEvents, _ := entry["events"].([]string)
	fields := req.Answers.validate(coachEvents, today)
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	// 受付停止中は新しい相談を送れない（コーチが手で切り替える。カレンダーとは連動しない）。
	if rec, _ := entry["reception"].(map[string]any); rec == nil || rec["status"] != "open" {
		Error(w, http.StatusConflict, "consult_paused", "このコーチはいま新しい相談を受け付けていません")
		return
	}
	raw, err := json.Marshal(req.Answers)
	if err != nil {
		MapDBError(w, err)
		return
	}
	minor := isMinor(*me.BirthMonth, now)
	var c db.SupportConsult
	err = s.tx(r.Context(), func(q *db.Queries) error {
		if err := q.LockCoachingPlans(r.Context(), req.CoachID); err != nil {
			return err
		}
		// 再相談は、そのコーチとの契約も相談も進行していない状態から。
		if open, err := q.HasOpenCoachingContract(r.Context(), db.HasOpenCoachingContractParams{RequesterID: me.ID, CoachID: req.CoachID}); err != nil {
			return err
		} else if open {
			return errConsultState
		}
		var err error
		c, err = q.CreateSupportConsult(r.Context(), db.CreateSupportConsultParams{
			ID: uuid.New(), AthleteID: me.ID, CoachID: req.CoachID, Minor: minor, Event: req.Answers.Event,
			Answers: raw, AnswerBy: ptrTime(now.Add(supportConsultRespondWindow)),
		})
		return err
	})
	var pgErr *pgconn.PgError
	if errors.Is(err, errConsultState) || (errors.As(err, &pgErr) && pgErr.Code == "23505") {
		Error(w, http.StatusConflict, "conflict", "このコーチとは、相談か契約が進行中です。終わってから、次の大会の相談を送れます")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	s.notify(r.Context(), c.CoachID, notifyCoaching, push.Message{
		Title: "1ヶ月プランの無料相談が届きました",
		Body:  "48時間以内に、提案か辞退を返してください",
		URL:   "/coach/support-consults", Tag: "support-consult-" + c.ID.String(),
	}, 24*time.Hour)
	JSON(w, http.StatusCreated, map[string]any{"consult": s.consultViewJSON(c)})
}

func ptrTime(t time.Time) *time.Time { return &t }

// loadConsultFor は、相談を読み、本人（選手またはコーチ）かを確かめる。違えば 404。
func (s *Server) loadConsultFor(w http.ResponseWriter, r *http.Request, q *db.Queries, me uuid.UUID, lock bool) (db.SupportConsult, bool) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return db.SupportConsult{}, false
	}
	var c db.SupportConsult
	var err error
	if lock {
		c, err = q.LockSupportConsult(r.Context(), id)
	} else {
		c, err = q.GetSupportConsult(r.Context(), id)
	}
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && c.AthleteID != me && c.CoachID != me) {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return db.SupportConsult{}, false
	}
	if err != nil {
		MapDBError(w, err)
		return db.SupportConsult{}, false
	}
	return c, true
}

func (s *Server) consultDetail(ctx context.Context, c db.SupportConsult) (map[string]any, error) {
	out := s.consultViewJSON(c)
	qs, err := s.q.ListSupportConsultQuestions(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	questions := make([]map[string]any, 0, len(qs))
	for _, q := range qs {
		questions = append(questions, map[string]any{"id": q.ID, "question": q.Question, "answer": q.Answer, "asked_at": q.AskedAt, "answered_at": q.AnsweredAt})
	}
	out["questions"] = questions
	out["proposal"] = nil
	if p, err := s.q.GetSupportConsultProposal(ctx, c.ID); err == nil {
		pj := proposalJSON(p)
		pj["expired"] = !p.ValidUntil.After(time.Now())
		out["proposal"] = pj
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	return out, nil
}

func (s *Server) getSupportConsult(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	c, ok := s.loadConsultFor(w, r, s.q, me.ID, false)
	if !ok {
		return
	}
	out, err := s.consultDetail(r.Context(), c)
	if err != nil {
		MapDBError(w, err)
		return
	}
	out["role"] = "athlete"
	if c.CoachID == me.ID {
		out["role"] = "coach"
	}
	JSON(w, http.StatusOK, map[string]any{"consult": out})
}

func (s *Server) listMySupportConsults(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	rows, err := s.q.ListAthleteSupportConsults(r.Context(), me.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		v := s.consultViewJSON(db.SupportConsult{ID: row.ID, AthleteID: row.AthleteID, CoachID: row.CoachID, Status: row.Status, Minor: row.Minor, Event: row.Event,
			Answers: row.Answers, AnswerBy: row.AnswerBy, ContractID: row.ContractID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt})
		v["coach_id"], v["coach_nickname"], v["coach_photo_url"] = row.CoachID, row.CoachNickname, s.coachPhotoURL(row.CoachPhotoPublicID)
		out = append(out, v)
	}
	JSON(w, http.StatusOK, map[string]any{"consults": out})
}

func (s *Server) listCoachSupportConsults(w http.ResponseWriter, r *http.Request) {
	c, ok := s.approvedCoach(w, r)
	if !ok {
		return
	}
	rows, err := s.q.ListCoachSupportConsults(r.Context(), c.ProfileID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		v := s.consultViewJSON(db.SupportConsult{ID: row.ID, AthleteID: row.AthleteID, CoachID: row.CoachID, Status: row.Status, Minor: row.Minor, Event: row.Event,
			Answers: row.Answers, AnswerBy: row.AnswerBy, ContractID: row.ContractID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt})
		v["athlete_nickname"] = row.AthleteNickname
		out = append(out, v)
	}
	JSON(w, http.StatusOK, map[string]any{"consults": out})
}

func (s *Server) consultConflict(w http.ResponseWriter) {
	Error(w, http.StatusConflict, "conflict", "この相談はいまその操作ができない状態です")
}

// withdrawSupportConsult は、選手が相談を取り下げる。提案を受ける前なら、いつでもできる。
func (s *Server) withdrawSupportConsult(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	var out db.SupportConsult
	var fail bool
	err := s.tx(r.Context(), func(q *db.Queries) error {
		c, ok := s.loadConsultFor(w, r, q, me.ID, true)
		if !ok {
			fail = true
			return nil
		}
		if c.AthleteID != me.ID || !slices.Contains([]string{"requested", "clarifying", "proposed", "sent_to_guardian"}, c.Status) {
			s.consultConflict(w)
			fail = true
			return nil
		}
		var err error
		out, err = q.SetSupportConsultStatus(r.Context(), db.SetSupportConsultStatusParams{ID: c.ID, Status: "withdrawn"})
		return err
	})
	if fail {
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"consult": s.consultViewJSON(out)})
}

// answerSupportConsult は、選手がコーチの聞き直しに答える。答えが届いたら、コーチの 48 時間を数え直す。
func (s *Server) answerSupportConsult(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	var req struct {
		Answer string `json:"answer"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	req.Answer = strings.TrimSpace(req.Answer)
	if e := coachPublicTextError("答え", req.Answer, 500); e != "" {
		FieldError(w, map[string]string{"answer": e})
		return
	}
	var out db.SupportConsult
	var fail bool
	err := s.tx(r.Context(), func(q *db.Queries) error {
		c, ok := s.loadConsultFor(w, r, q, me.ID, true)
		if !ok {
			fail = true
			return nil
		}
		if c.AthleteID != me.ID || c.Status != "clarifying" {
			s.consultConflict(w)
			fail = true
			return nil
		}
		n, err := q.AnswerSupportConsultQuestion(r.Context(), db.AnswerSupportConsultQuestionParams{ConsultID: c.ID, Answer: &req.Answer})
		if err != nil {
			return err
		}
		if n == 0 {
			s.consultConflict(w)
			fail = true
			return nil
		}
		out, err = q.SetSupportConsultStatus(r.Context(), db.SetSupportConsultStatusParams{ID: c.ID, Status: "requested", AnswerBy: ptrTime(time.Now().Add(supportConsultRespondWindow))})
		return err
	})
	if fail {
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	s.notify(r.Context(), out.CoachID, notifyCoaching, push.Message{
		Title: "無料相談の聞き直しに答えが届きました", Body: "48時間以内に、提案か辞退を返してください",
		URL: "/coach/support-consults", Tag: "support-consult-" + out.ID.String(),
	}, 24*time.Hour)
	JSON(w, http.StatusOK, map[string]any{"consult": s.consultViewJSON(out)})
}

// ---- コーチ: 辞退・聞き直し・提案 ----

func (s *Server) declineSupportConsult(w http.ResponseWriter, r *http.Request) {
	coach, ok := s.approvedCoach(w, r)
	if !ok {
		return
	}
	var out db.SupportConsult
	var fail bool
	err := s.tx(r.Context(), func(q *db.Queries) error {
		c, ok := s.loadConsultFor(w, r, q, coach.ProfileID, true)
		if !ok {
			fail = true
			return nil
		}
		if c.CoachID != coach.ProfileID || !slices.Contains([]string{"requested", "clarifying"}, c.Status) {
			s.consultConflict(w)
			fail = true
			return nil
		}
		var err error
		out, err = q.SetSupportConsultStatus(r.Context(), db.SetSupportConsultStatusParams{ID: c.ID, Status: "declined"})
		return err
	})
	if fail {
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	s.notify(r.Context(), out.AthleteID, notifyCoaching, push.Message{
		Title: "無料相談の結果", Body: "今回はコーチの都合で提案をいただけませんでした。費用はかかっていません。ほかのコーチにも相談できます",
		URL: "/coaching/support-consults/" + out.ID.String(), Tag: "support-consult-" + out.ID.String(),
	}, 24*time.Hour)
	JSON(w, http.StatusOK, map[string]any{"consult": s.consultViewJSON(out)})
}

// askSupportConsult は、フォームの記入だけでは分からないとき、コーチが選手に聞き直す（フォーム形式の追加質問）。
func (s *Server) askSupportConsult(w http.ResponseWriter, r *http.Request) {
	coach, ok := s.approvedCoach(w, r)
	if !ok {
		return
	}
	var req struct {
		Question string `json:"question"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	req.Question = strings.TrimSpace(req.Question)
	if e := coachPublicTextError("聞きたいこと", req.Question, 300); e != "" {
		FieldError(w, map[string]string{"question": e})
		return
	}
	var out db.SupportConsult
	var fail bool
	err := s.tx(r.Context(), func(q *db.Queries) error {
		c, ok := s.loadConsultFor(w, r, q, coach.ProfileID, true)
		if !ok {
			fail = true
			return nil
		}
		if c.CoachID != coach.ProfileID || c.Status != "requested" {
			s.consultConflict(w)
			fail = true
			return nil
		}
		n, err := q.CountSupportConsultQuestions(r.Context(), c.ID)
		if err != nil {
			return err
		}
		if n >= maxSupportConsultQuestions {
			Error(w, http.StatusConflict, "conflict", "聞き直しは3回までです。提案か辞退を返してください")
			fail = true
			return nil
		}
		if _, err := q.CreateSupportConsultQuestion(r.Context(), db.CreateSupportConsultQuestionParams{ID: uuid.New(), ConsultID: c.ID, Question: req.Question}); err != nil {
			return err
		}
		out, err = q.SetSupportConsultStatus(r.Context(), db.SetSupportConsultStatusParams{ID: c.ID, Status: "clarifying"})
		return err
	})
	if fail {
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	s.notify(r.Context(), out.AthleteID, notifyCoaching, push.Message{
		Title: "無料相談でコーチから聞き直しがあります", Body: "答えると、コーチが提案を考えます",
		URL: "/coaching/support-consults/" + out.ID.String(), Tag: "support-consult-" + out.ID.String(),
	}, 24*time.Hour)
	JSON(w, http.StatusOK, map[string]any{"consult": s.consultViewJSON(out)})
}

// supportConsultPeriodDays は、サポートプランの期間（1ヶ月）の日数。期間プランの既存の数え方
// （validPlanDuration: 1か月＝30日）を使い、新しい日数計算は作らない。期間の終わりは開始日から数えて
// 両端を含む30日目（開始日 +29日）で、目標大会の日に依存しない。
const supportConsultPeriodDays = 30

type proposalReq struct {
	// CompetitionName は目標の大会の名前（目標としての記録で、期間を決めない）。
	CompetitionName  string `json:"competition_name"`
	SupportStartDate string `json:"support_start_date"`
	CallCount        int    `json:"call_count"`
	CallMinutes      int    `json:"call_minutes"`
	PriceYen         int    `json:"price_yen"`
	ConfirmHighPrice bool   `json:"confirm_high_price"`
	ContentNote      string `json:"content_note"`
	AnswerToQuestion string `json:"answer_to_question"`
	ValidUntil       string `json:"valid_until"`
	// 提案を出すことが、コーチ側の契約への同意になる（相談由来の契約では、引受をあらためて挟まない）。
	AgreeReport    bool `json:"agree_report"`
	AgreeDetection bool `json:"agree_detection"`
}

// validate は提案を確かめる。有効期限（valid_until）は日本時間のその日の終わりまで。
// 期間の終わり（end）は開始日から1ヶ月（30日・両端を含む）で組み立て、サーバーが決める。
func (p *proposalReq) validate(today time.Time, askedQuestion bool) (map[string]string, time.Time, time.Time, time.Time) {
	f := map[string]string{}
	p.CompetitionName = strings.TrimSpace(p.CompetitionName)
	p.ContentNote = strings.TrimSpace(p.ContentNote)
	p.AnswerToQuestion = strings.TrimSpace(p.AnswerToQuestion)
	if e := coachPublicTextError("大会名", p.CompetitionName, 100); e != "" {
		f["competition_name"] = e
	}
	start, e1 := time.Parse("2006-01-02", p.SupportStartDate)
	if e1 != nil || start.Before(today) {
		f["support_start_date"] = "開始日を今日以降の日で選んでください"
	}
	end := start.AddDate(0, 0, supportConsultPeriodDays-1)
	if p.CallCount < 0 || p.CallCount > 12 {
		f["call_count"] = "通話は0〜12回で設定してください"
	}
	if !slices.Contains([]int{15, 30, 45, 60}, p.CallMinutes) {
		f["call_minutes"] = "通話は15・30・45・60分から選んでください"
	}
	switch {
	case p.PriceYen < minPlanPriceYen:
		f["price_yen"] = "価格は2,000円以上にしてください"
	case p.PriceYen > maxPlanPriceYen:
		f["price_yen"] = "価格が大きすぎます。桁を確かめてください"
	case p.PriceYen > highPlanPriceYen && !p.ConfirmHighPrice:
		f["price_yen"] = "30,000円を超える価格です。打ち間違いでないか確かめてください"
	}
	if utf8.RuneCountInString(p.ContentNote) > 500 {
		f["content_note"] = "内容の補足は500文字以内で入力してください"
	} else if p.ContentNote != "" && looksLikeContact(p.ContentNote) {
		f["content_note"] = "連絡先（URL・メールアドレス・SNS の ID・電話番号など）は書けません"
	}
	if askedQuestion {
		if e := coachPublicTextError("聞きたいことへの回答", p.AnswerToQuestion, 1000); e != "" {
			f["answer_to_question"] = e
		}
	} else if p.AnswerToQuestion != "" && looksLikeContact(p.AnswerToQuestion) {
		f["answer_to_question"] = "連絡先（URL・メールアドレス・SNS の ID・電話番号など）は書けません"
	}
	for k, v := range (contractConsentReq{AgreeReport: p.AgreeReport, AgreeDetection: p.AgreeDetection}).fields() {
		f[k] = v
	}
	var validUntil time.Time
	day, err := time.Parse("2006-01-02", p.ValidUntil)
	switch {
	case err != nil || day.Before(today):
		f["valid_until"] = "提案の有効期限を今日以降の日で選んでください"
	case e1 == nil && day.After(end):
		f["valid_until"] = "提案の有効期限は期間の終わり（開始日から1ヶ月）までにしてください"
	default:
		validUntil = time.Date(day.Year(), day.Month(), day.Day()+1, 0, 0, 0, 0, jst.Zone)
	}
	return f, start, end, validUntil
}

func (s *Server) proposeSupportConsult(w http.ResponseWriter, r *http.Request) {
	coach, ok := s.approvedCoach(w, r)
	if !ok {
		return
	}
	var req proposalReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	if !s.cfg.CoachingMessaging {
		Error(w, http.StatusConflict, "conflict", "このサービスはメッセージのやりとりの準備ができるまで提案できません")
		return
	}
	if !pledged(coach) {
		Error(w, http.StatusConflict, "pledge_required", "先にパーソナル指導の誓約をしてください")
		return
	}
	if !s.commerceReadyToPublish(w, r, coach.ProfileID) {
		return
	}
	if !s.payoutsReady(s.refreshPayoutsEnabled(r.Context(), coach)) {
		Error(w, http.StatusUnprocessableEntity, "payouts_required", payoutsNotReadyMessage)
		return
	}
	var out db.SupportConsult
	var fail bool
	err := s.tx(r.Context(), func(q *db.Queries) error {
		c, ok := s.loadConsultFor(w, r, q, coach.ProfileID, true)
		if !ok {
			fail = true
			return nil
		}
		if c.CoachID != coach.ProfileID || c.Status != "requested" || (c.AnswerBy != nil && !c.AnswerBy.After(time.Now())) {
			s.consultConflict(w)
			fail = true
			return nil
		}
		var answers consultAnswers
		_ = json.Unmarshal(c.Answers, &answers)
		fields, start, end, validUntil := req.validate(availabilityToday(time.Now()), answers.QuestionToCoach != "")
		if len(fields) > 0 {
			FieldError(w, fields)
			fail = true
			return nil
		}
		if _, err := q.CreateSupportConsultProposal(r.Context(), db.CreateSupportConsultProposalParams{
			ConsultID: c.ID, CompetitionName: req.CompetitionName, SupportStartDate: start, CompetitionDate: end,
			CallCount: int16(req.CallCount), CallMinutes: int16(req.CallMinutes), PriceYen: int32(req.PriceYen),
			ContentNote: req.ContentNote, AnswerToQuestion: req.AnswerToQuestion, ValidUntil: validUntil,
		}); err != nil {
			return err
		}
		var err error
		out, err = q.SetSupportConsultStatus(r.Context(), db.SetSupportConsultStatusParams{ID: c.ID, Status: "proposed"})
		return err
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		s.consultConflict(w)
		return
	}
	if fail {
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	s.notify(r.Context(), out.AthleteID, notifyCoaching, push.Message{
		Title: "無料相談にコーチの提案が届きました", Body: "期間・内容・価格を確認してください。提案を受けるまで、お金はかかりません",
		URL: "/coaching/support-consults/" + out.ID.String(), Tag: "support-consult-" + out.ID.String(),
	}, 24*time.Hour)
	detail, err := s.consultDetail(r.Context(), out)
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"consult": detail})
}

// ---- 提案を受ける ----

// supportStandardContent は、1ヶ月プランの「何が返ってくるか」の標準形。提案の内容欄にいつも明記し、契約の説明にも写す。
// 返答の回数や形式は選ばせない（動画への返答は期間中は無制限。追加相談という別料金もない）。
const supportStandardContent = "動画への返答は、期間中は回数の制限なく受けられます。メニューはコーチが先に作り、必要に応じて見直します。"

// startConsultContract は、相談の提案から契約を作る。コーチの同意は提案を出した時に済んでいるので、
// コーチの再引受は挟まず、そのまま「受けた」状態（支払い待ち・保護者の同意待ち）にする。
// 契約の相手（requester）は選手本人。呼ぶのは、大人が申し込んだ時と、保護者が同意した時だけ。
func (s *Server) startConsultContract(ctx context.Context, q *db.Queries, c db.SupportConsult, requester uuid.UUID, requesterConsent bool) (db.CoachingContract, error) {
	p, err := q.GetSupportConsultProposal(ctx, c.ID)
	if err != nil {
		return db.CoachingContract{}, err
	}
	tpl, err := q.GetConsultTemplatePlan(ctx, c.CoachID)
	if err != nil {
		return db.CoachingContract{}, err
	}
	var a consultAnswers
	_ = json.Unmarshal(c.Answers, &a)
	desc := strings.TrimSpace("1ヶ月プラン（無料相談の提案）。" + p.CompetitionName + "に向けた1ヶ月のサポート。" + supportStandardContent + p.ContentNote)
	title := truncateRunes("1ヶ月プラン: "+p.CompetitionName, 60)
	zero := int16(0)
	ct, err := q.CreateCoachingContract(ctx, db.CreateCoachingContractParams{
		PreferredDates: []time.Time{}, ID: uuid.New(), PlanID: tpl.ID, CoachID: c.CoachID, RequesterID: requester,
		PlanKind: "period", PlanTitle: title, DurationDays: int16(proposalDays(p)), PriceYen: p.PriceYen,
		Event: c.Event, Goal: consultGoalSummary(a, p), CurrentRecord: truncateRunes("自己ベスト "+a.PersonalBest, 100),
		Message: truncateRunes(a.MainWorry, 1000), Minor: c.Minor, RespondBy: time.Now().Add(contractRespondWindow),
		VideoReviews: &zero, ReplyFormats: []string{},
		ServiceType: ptrString("competition_support"), PlanDescription: &desc,
	})
	if err != nil {
		return ct, err
	}
	if ct, err = q.SnapshotSupportContractFromProposal(ctx, db.SnapshotSupportContractFromProposalParams{ContractID: ct.ID, ConsultID: c.ID}); err != nil {
		return ct, err
	}
	if err := q.InsertContractConsent(ctx, db.InsertContractConsentParams{
		ContractID: ct.ID, Party: "coach", ProfileID: &c.CoachID, Version: contractConsentVersion,
	}); err != nil {
		return ct, err
	}
	if requesterConsent {
		if err := q.InsertContractConsent(ctx, db.InsertContractConsentParams{
			ContractID: ct.ID, Party: "requester", ProfileID: &requester, Version: contractConsentVersion,
		}); err != nil {
			return ct, err
		}
	}
	// コーチの引受の代わり。未成年は保護者の同意と支払い待ち、大人は支払い待ち（支払いのないデモ版はそのまま開始）。
	if ct, err = q.AcceptCoachingContract(ctx, db.AcceptCoachingContractParams{ID: ct.ID, CoachID: c.CoachID, PaymentRequired: s.stripe.Configured()}); err != nil {
		return ct, err
	}
	if _, err := q.SetSupportConsultContract(ctx, db.SetSupportConsultContractParams{ID: c.ID, ContractID: &ct.ID}); err != nil {
		return ct, err
	}
	// コーチへ、この契約の取引条件（給付の内容・報酬の算定方法・支払期日など）を電磁的に明示する。
	if err := issueSupportContractStatement(ctx, q, ct); err != nil {
		return ct, err
	}
	return ct, nil
}

// checkConsultStartable は、提案がまだ有効で、契約を作れる状態かを確かめる。だめなら応答を書いて false。
func (s *Server) checkConsultStartable(w http.ResponseWriter, r *http.Request, q *db.Queries, c db.SupportConsult, requester uuid.UUID) bool {
	p, err := q.GetSupportConsultProposal(r.Context(), c.ID)
	if err != nil {
		MapDBError(w, err)
		return false
	}
	if !p.ValidUntil.After(time.Now()) || p.CompetitionDate.Before(availabilityToday(time.Now())) {
		Error(w, http.StatusConflict, "proposal_expired", "この提案は有効期限が過ぎています")
		return false
	}
	if err := q.LockCoachingPlans(r.Context(), c.CoachID); err != nil {
		MapDBError(w, err)
		return false
	}
	tpl, err := q.GetConsultTemplatePlan(r.Context(), c.CoachID)
	if err != nil {
		MapDBError(w, err)
		return false
	}
	if _, approved, err := s.identity.disclosure(r.Context(), tpl.ID); err != nil {
		identityServiceError(w)
		return false
	} else if !approved {
		Error(w, http.StatusConflict, "commerce_unregistered", "この提案はいま申し込めません")
		return false
	}
	if coach, err := s.q.GetCoachProfile(r.Context(), c.CoachID); err != nil || coach.Status != "approved" {
		Error(w, http.StatusConflict, "conflict", "この提案はいま申し込めません")
		return false
	} else if !s.payoutsReady(s.refreshPayoutsEnabled(r.Context(), coach)) {
		Error(w, http.StatusConflict, "conflict", "この提案はいま申し込めません")
		return false
	}
	if open, err := q.HasOpenCoachingContract(r.Context(), db.HasOpenCoachingContractParams{RequesterID: requester, CoachID: c.CoachID}); err != nil {
		MapDBError(w, err)
		return false
	} else if open {
		Error(w, http.StatusConflict, "conflict", "このコーチには、返事待ちか期間中の申し込みがあります")
		return false
	}
	return true
}

// acceptSupportConsult は、大人の選手が提案の内容を確認して申し込む。ここで契約ができ、コーチの再引受は挟まず、
// お支払いが済んだ時点で始まる。未成年の選手は、ここでは契約をつくれない（保護者に提案を送る）。
func (s *Server) acceptSupportConsult(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	if !me.CoachingDemo {
		Error(w, http.StatusForbidden, "forbidden", "パーソナル指導は準備中です")
		return
	}
	var req contractConsentReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	if fields := req.fields(); len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	var ct db.CoachingContract
	var fail bool
	err := s.tx(r.Context(), func(q *db.Queries) error {
		c, ok := s.loadConsultFor(w, r, q, me.ID, true)
		if !ok {
			fail = true
			return nil
		}
		if c.AthleteID != me.ID || c.Status != "proposed" {
			s.consultConflict(w)
			fail = true
			return nil
		}
		if c.Minor {
			Error(w, http.StatusConflict, "guardian_required", "18歳未満の方は、保護者に提案を送って、保護者が同意してお支払いをします")
			fail = true
			return nil
		}
		if !s.checkConsultStartable(w, r, q, c, me.ID) {
			fail = true
			return nil
		}
		var err error
		ct, err = s.startConsultContract(r.Context(), q, c, me.ID, true)
		return err
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		Error(w, http.StatusConflict, "conflict", "このコーチには、返事待ちか期間中の申し込みがあります")
		return
	}
	if fail {
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	body := "お支払いを済ませると始まります"
	if ct.Status == "active" {
		body = "「" + ct.PlanTitle + "」が始まりました"
	}
	s.notify(r.Context(), ct.CoachID, notifyCoaching, push.Message{
		Title: "無料相談の提案が受け入れられました", Body: body,
		URL: "/coach", Tag: "contract-" + ct.ID.String(),
	}, 24*time.Hour)
	JSON(w, http.StatusCreated, map[string]any{"contract": s.contractJSON(ct)})
}

// sendSupportConsultToGuardian は、未成年の選手が、届いた提案を保護者に送る。契約はまだできない。
// 保護者が、このサービスの保護者としての同意を済ませているときだけ送れる。
func (s *Server) sendSupportConsultToGuardian(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	var out db.SupportConsult
	var fail bool
	err := s.tx(r.Context(), func(q *db.Queries) error {
		c, ok := s.loadConsultFor(w, r, q, me.ID, true)
		if !ok {
			fail = true
			return nil
		}
		if c.AthleteID != me.ID || c.Status != "proposed" || !c.Minor {
			s.consultConflict(w)
			fail = true
			return nil
		}
		p, err := q.GetSupportConsultProposal(r.Context(), c.ID)
		if err != nil {
			return err
		}
		if !p.ValidUntil.After(time.Now()) || p.CompetitionDate.Before(availabilityToday(time.Now())) {
			Error(w, http.StatusConflict, "proposal_expired", "この提案は有効期限が過ぎています")
			fail = true
			return nil
		}
		g, err := q.GetGuardianConsentByMinor(r.Context(), me.ID)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && g.Status != "consented") {
			Error(w, http.StatusConflict, "guardian_not_consented", "先に保護者のリンクを作り、保護者に同意してもらってください")
			fail = true
			return nil
		}
		if err != nil {
			return err
		}
		out, err = q.SetSupportConsultStatus(r.Context(), db.SetSupportConsultStatusParams{ID: c.ID, Status: "sent_to_guardian"})
		return err
	})
	if fail {
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	detail, err := s.consultDetail(r.Context(), out)
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"consult": detail})
}

// ---- 保護者: 子に届いた提案 ----

func (s *Server) guardianConsult(w http.ResponseWriter, r *http.Request, q *db.Queries, lock bool) (db.SupportConsult, bool) {
	minor, ok := s.guardianMinor(w, r)
	if !ok {
		return db.SupportConsult{}, false
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return db.SupportConsult{}, false
	}
	var c db.SupportConsult
	var err error
	if lock {
		c, err = q.LockSupportConsult(r.Context(), id)
	} else {
		c, err = q.GetSupportConsult(r.Context(), id)
	}
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (c.AthleteID != minor || !c.Minor || (c.Status != "sent_to_guardian" && c.Status != "accepted"))) {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return db.SupportConsult{}, false
	}
	if err != nil {
		MapDBError(w, err)
		return db.SupportConsult{}, false
	}
	return c, true
}

func (s *Server) listGuardianSupportConsults(w http.ResponseWriter, r *http.Request) {
	minor, ok := s.guardianMinor(w, r)
	if !ok {
		return
	}
	rows, err := s.q.ListGuardianSupportConsults(r.Context(), minor)
	if err != nil {
		MapDBError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		c := db.SupportConsult{ID: row.ID, AthleteID: row.AthleteID, CoachID: row.CoachID, Status: row.Status, Minor: row.Minor, Event: row.Event,
			Answers: row.Answers, AnswerBy: row.AnswerBy, ContractID: row.ContractID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
		v, err := s.consultDetail(r.Context(), c)
		if err != nil {
			MapDBError(w, err)
			return
		}
		v["coach_id"], v["coach_nickname"], v["coach_photo_url"] = row.CoachID, row.CoachNickname, s.coachPhotoURL(row.CoachPhotoPublicID)
		out = append(out, v)
	}
	JSON(w, http.StatusOK, map[string]any{"consults": out})
}

func (s *Server) getGuardianSupportConsult(w http.ResponseWriter, r *http.Request) {
	c, ok := s.guardianConsult(w, r, s.q, false)
	if !ok {
		return
	}
	out, err := s.consultDetail(r.Context(), c)
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"consult": out})
}

// consentGuardianSupportConsult は、保護者が提案に同意して申し込む。ここで初めて契約ができ、
// コーチの再引受は挟まず、保護者の同意とお支払いで成立する（支払いのないデモ版は同意でそのまま開始）。
func (s *Server) consentGuardianSupportConsult(w http.ResponseWriter, r *http.Request) {
	var req contractConsentReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	if fields := req.fields(); len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	var ct db.CoachingContract
	var fail bool
	err := s.tx(r.Context(), func(q *db.Queries) error {
		c, ok := s.guardianConsult(w, r, q, true)
		if !ok {
			fail = true
			return nil
		}
		if c.Status != "sent_to_guardian" {
			s.consultConflict(w)
			fail = true
			return nil
		}
		if !s.checkConsultStartable(w, r, q, c, c.AthleteID) {
			fail = true
			return nil
		}
		var err error
		ct, err = s.startConsultContract(r.Context(), q, c, c.AthleteID, false)
		if err != nil {
			return err
		}
		if err := q.InsertContractConsent(r.Context(), db.InsertContractConsentParams{
			ContractID: ct.ID, Party: "guardian", ProfileID: nil, Version: contractConsentVersion,
		}); err != nil {
			return err
		}
		if !ct.PaymentRequired {
			ct, err = q.GuardianStartCoachingContract(r.Context(), db.GuardianStartCoachingContractParams{ID: ct.ID, RequesterID: ct.RequesterID})
		}
		return err
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		Error(w, http.StatusConflict, "conflict", "このコーチには、返事待ちか期間中の申し込みがあります")
		return
	}
	if fail {
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	if ct.Status == "active" {
		msg := push.Message{Title: "パーソナル指導が始まりました", Body: "保護者が「" + ct.PlanTitle + "」に同意しました", URL: "/coaching/personal/" + ct.ID.String(), Tag: "contract-" + ct.ID.String()}
		s.notify(r.Context(), ct.RequesterID, notifyCoaching, msg, 24*time.Hour)
		s.notify(r.Context(), ct.CoachID, notifyCoaching, msg, 24*time.Hour)
		s.writeGuardianContract(w, r, ct.ID)
		return
	}
	s.notify(r.Context(), ct.CoachID, notifyCoaching, push.Message{
		Title: "無料相談の提案が、保護者に受け入れられました", Body: "保護者がお支払いを済ませると始まります",
		URL: "/coach", Tag: "contract-" + ct.ID.String(),
	}, 24*time.Hour)
	s.consentAndCheckoutGuardian(w, r, ct)
}

func (s *Server) declineGuardianSupportConsult(w http.ResponseWriter, r *http.Request) {
	var out db.SupportConsult
	var fail bool
	err := s.tx(r.Context(), func(q *db.Queries) error {
		c, ok := s.guardianConsult(w, r, q, true)
		if !ok {
			fail = true
			return nil
		}
		if c.Status != "sent_to_guardian" {
			s.consultConflict(w)
			fail = true
			return nil
		}
		var err error
		out, err = q.SetSupportConsultStatus(r.Context(), db.SetSupportConsultStatusParams{ID: c.ID, Status: "withdrawn"})
		return err
	})
	if fail {
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	s.notify(r.Context(), out.AthleteID, notifyCoaching, push.Message{
		Title: "保護者が提案に同意しませんでした", Body: "費用はかかっていません。保護者と話してから、もう一度相談できます",
		URL: "/coaching/support-consults/" + out.ID.String(), Tag: "support-consult-" + out.ID.String(),
	}, 24*time.Hour)
	JSON(w, http.StatusOK, map[string]any{"consult": s.consultViewJSON(out)})
}

func ptrInt16(v int16) *int16    { return &v }
func ptrString(v string) *string { return &v }

func truncateRunes(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) > n {
		r = r[:n]
	}
	return string(r)
}

func consultGoalSummary(a consultAnswers, p db.SupportConsultProposal) string {
	return truncateRunes(p.CompetitionName+"に向けた1ヶ月のサポート", 200)
}

// expireSupportConsults は、コーチが 48 時間返事をしなかった相談と、有効期限が過ぎた提案を閉じる。
func (s *Server) expireSupportConsults(ctx context.Context) (int, error) {
	rows, err := s.q.ExpireSupportConsults(ctx)
	if err != nil {
		return 0, err
	}
	for _, c := range rows {
		s.notify(ctx, c.AthleteID, notifyCoaching, push.Message{
			Title: "無料相談の期限が過ぎました", Body: "費用はかかっていません。もう一度相談するか、ほかのコーチにも相談できます",
			URL: "/coaching/support-consults/" + c.ID.String(), Tag: "support-consult-" + c.ID.String(),
		}, 24*time.Hour)
	}
	return len(rows), nil
}
