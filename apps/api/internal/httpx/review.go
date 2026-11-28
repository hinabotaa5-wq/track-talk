package httpx

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/cloudinary"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/push"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// おまかせ添削の依頼（決済なしのデモ版）。
// 決済ができるまでは、運営が配った招待コードを入れた人（profiles.coaching_demo）だけが依頼できる。

// 見てほしい点。並びは画面に出す順。
var reviewFocusOptions = []string{"start", "acceleration", "top-speed", "takeoff", "arm", "contact", "rhythm", "other"}

const (
	defaultDailyCap = 5
	maxDailyCap     = 20
)

// 招待コードに使う文字。読み違えやすい 0/O と 1/I は外す。
const demoCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// 添削を受け付ける種目。いまは100m・200m・400m（Web の reviewEvents と同じ種目・同じ並びにする）。
var reviewEvents = []string{"100m", "200m", "400m"}

func validReviewEvent(event string) bool {
	return oneOf(event, reviewEvents)
}

// normalizeFocus は、重複を除いて画面と同じ順に並べ直す。知らない値があれば false。
func normalizeFocus(focus []string) ([]string, bool) {
	picked := map[string]bool{}
	for _, f := range focus {
		picked[f] = true
	}
	out := make([]string, 0, len(picked))
	for _, f := range reviewFocusOptions {
		if picked[f] {
			out = append(out, f)
			delete(picked, f)
		}
	}
	return out, len(picked) == 0
}

// reviewWaiting は、まだ返却を待っている状態か。
func reviewWaiting(status string) bool {
	return status == "queued" || status == "offered" || status == "in_review"
}

func newDemoCode() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, len(buf))
	for i, b := range buf {
		out[i] = demoCodeAlphabet[int(b)%len(demoCodeAlphabet)]
	}
	return string(out), nil
}

func (s *Server) reviewRequestJSON(rr db.ReviewRequest) map[string]any {
	return map[string]any{
		"id":       rr.ID,
		"plan":     rr.Plan, // standard／premium（本体 #1053）
		"tier":     rr.Tier,
		"event":    rr.Event,
		"focus":    rr.Focus,
		"question": rr.Question,
		// 申込で選手が任意で書いた選手情報（本体 #1013 補足）。未入力は null。
		"pb": rr.Pb, "goal_time": rr.GoalTime, "wind_speed": rr.WindSpeed, "body_condition": rr.BodyCondition,
		// 申込で選手が任意で書いた「意識していること」（本体 #1093）。未入力は null。
		"focus_note":         rr.FocusNote,
		"started_at":         rr.StartedAt,
		"canceled_at":        rr.CanceledAt,
		"status":             rr.Status,
		"due_at":             rr.DueAt,
		"created_at":         rr.CreatedAt,
		"requested_coach_id": rr.RequestedCoachID, "requested_coach_name": rr.RequestedCoachName,
		"base_price_yen": rr.BasePriceYen, "use_tickets": rr.UseTickets,
		// 依頼の作成時に固定した追加項目のタイトル（本体 #1086）。指名コーチのいないスタンダード・プレミアムは空。
		"offer_items":     nonNilStrings(rr.OfferItems),
		"total_price_yen": rr.BasePriceYen, "cash_price_yen": reviewCashPrice(rr),
		// 適用したクーポンの割引額（運営負担。本体 #1031）。使っていなければ 0。
		"coupon_discount_yen": rr.CouponDiscountYen,
		"video_url":           reviewVideoURL(rr, s.cloud.CoachingPlaybackURL),
		"thumbnail_url":       reviewVideoURL(rr, s.cloud.CoachingThumbnailURL),
		"info_request":        infoRequestJSON(rr),
		"withdrawal":          withdrawalJSON(rr),
		// スレッドの終わりから7日を過ぎて、動画ファイル（依頼の動画とコーチの見本動画）を消したか（本体 #506）。
		"videos_purged": rr.VideosPurgedAt != nil,
		// 海外コーチとのやり取りの訳（本体 #916）。詳細の応答でだけ withReviewTranslations が入れ、それ以外は null。
		"question_en":     nil,
		"info_request_ja": nil,
	}
}

// reviewVideoURL は、依頼の動画の URL。保持期間を過ぎて消した後は nil。
func reviewVideoURL(rr db.ReviewRequest, url func(string) string) any {
	if rr.VideoPublicID == nil || rr.VideosPurgedAt != nil {
		return nil
	}
	return url(*rr.VideoPublicID)
}

type coachDutyReq struct {
	OnDuty   *bool `json:"on_duty"`
	DailyCap *int  `json:"daily_cap"`
}

// updateCoachDuty は、承認済みのコーチの「受付中」と1日の上限本数を変える。
// 依頼の打診は Web プッシュで届くので、通知を受け取れる端末がないと受付中にはできない。
func (s *Server) updateCoachDuty(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	var req coachDutyReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	current, err := s.q.GetCoachProfile(r.Context(), p.ID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && current.Status != "approved") {
		Error(w, http.StatusForbidden, "forbidden", "承認されたコーチだけが受付を設定できます")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	onDuty := current.OnDuty
	if req.OnDuty != nil {
		onDuty = *req.OnDuty
	}
	dailyCap := int(current.DailyCap)
	if req.DailyCap != nil {
		dailyCap = *req.DailyCap
	}
	if dailyCap < 1 || dailyCap > maxDailyCap {
		FieldError(w, map[string]string{"daily_cap": "1日の上限は1〜20本で選んでください"})
		return
	}
	if onDuty && !current.OnDuty && !s.payoutsReady(s.refreshPayoutsEnabled(r.Context(), current)) {
		Error(w, http.StatusUnprocessableEntity, "payouts_required", payoutsNotReadyMessage)
		return
	}
	if onDuty && !current.OnDuty {
		// 登録の件数だけでは、届かない宛先でも受付中になれてしまう。通知を使えるときは、
		// 接続テストを送り、1台以上のプッシュサービスが受理したことを確かめる。
		var devices int64
		var err error
		if s.push.Configured() {
			var sent int
			sent, err = s.sendPush(r.Context(), p.ID, push.Message{
				Title: "スタンダードの通知テスト",
				Body:  "依頼の受付を始める前の接続テストです",
				URL:   "/coach",
				Tag:   "coach-duty-test",
			}, 10*time.Minute)
			devices = int64(sent)
		} else {
			devices, err = s.q.CountPushSubscriptions(r.Context(), p.ID)
		}
		if err != nil {
			MapDBError(w, err)
			return
		}
		if devices == 0 {
			Error(w, http.StatusUnprocessableEntity, "validation_error", "依頼は通知で届くので、先にこの端末で通知を受け取れるようにしてください")
			return
		}
	}
	c, err := s.q.UpdateCoachDuty(r.Context(), db.UpdateCoachDutyParams{OnDuty: onDuty, DailyCap: int16(dailyCap), ProfileID: p.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusForbidden, "forbidden", "承認されたコーチだけが受付を設定できます")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	if c.OnDuty && !current.OnDuty {
		s.pushQuietly(r.Context(), p.ID, notifyImportant, push.Message{
			Title: "依頼の受付を始めました",
			Body:  "スタンダードの依頼が届くと、この端末に通知します",
			URL:   "/coach",
			Tag:   "coach-duty",
		}, 10*time.Minute)
	}
	// 受付を始めたり上限を上げたりしたら、待っている依頼をすぐ回す。
	if c.OnDuty && (!current.OnDuty || c.DailyCap > current.DailyCap) {
		s.dispatchQueuedReviews(r.Context())
	}
	JSON(w, http.StatusOK, map[string]any{"application": s.coachProfileJSON(c)})
}

type demoCodeReq struct {
	Code string `json:"code"`
}

// joinCoachingDemo は、運営が配った招待コードでデモの参加者になる。
func (s *Server) joinCoachingDemo(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	var req demoCodeReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	if p.CoachingDemo {
		JSON(w, http.StatusOK, map[string]any{"demo": true})
		return
	}
	code := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(req.Code), "-", ""))
	if len(code) != 8 {
		FieldError(w, map[string]string{"code": "招待コードは8文字です"})
		return
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := s.q.WithTx(tx)
	if _, err := q.UseDemoCode(r.Context(), db.UseDemoCodeParams{Code: code, UsedBy: &p.ID}); errors.Is(err, pgx.ErrNoRows) {
		FieldError(w, map[string]string{"code": "この招待コードは使えません"})
		return
	} else if err != nil {
		MapDBError(w, err)
		return
	}
	if err := q.SetCoachingDemo(r.Context(), p.ID); err != nil {
		MapDBError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"demo": true})
}

// optionalRequestText は、申込の任意入力を1つ確かめる。空白だけ・未入力は nil（NULL で保存）。
// 上限を超える・連絡先らしき文字列があるときは fields にその項目の誤りを入れる。
func optionalRequestText(fields map[string]string, key string, value *string, limit int) *string {
	if value == nil {
		return nil
	}
	v := strings.TrimSpace(*value)
	if v == "" {
		return nil
	}
	if utf8.RuneCountInString(v) > limit {
		fields[key] = fmt.Sprintf("%d文字以内で記入してください", limit)
	} else if looksLikeContact(v) {
		fields[key] = "URL・メールアドレス・電話番号・SNS の ID など、連絡先らしきものは書けません"
	}
	return &v
}

type reviewRequestReq struct {
	Tier          string   `json:"tier"`
	Event         string   `json:"event"`
	Focus         []string `json:"focus"`
	Question      *string  `json:"question"`
	VideoPublicID string   `json:"video_public_id"`
	// 支払い方法（reviewPaymentMethods のどれか。今は card のみ）。Stripe の鍵があるときだけ使う。
	Method     string `json:"method"`
	UseTickets bool   `json:"use_tickets"`
	// チケットで払うときの種別（normal＝通常チケット／reward＝無料チケット。本体 #687）。
	// 空なら従来どおり種別を区別せず失効の早いものから使う。
	TicketKind   string `json:"ticket_kind"`
	ReferralCode string `json:"referral_code"`
	// 紹介特典の ¥500 クーポンを1枚使うか（本体 #1031）。coupon_id を指定したらその1枚、なければ期限の早いもの。
	// チケットとは併用できない。
	UseCoupon bool       `json:"use_coupon"`
	CouponID  *uuid.UUID `json:"coupon_id"`
	// 選手が選んだコーチ。おまかせ(REVIEW_RANDOM_ASSIGN_ENABLED)を止めている間は必須。
	RequestedCoachID *uuid.UUID `json:"requested_coach_id"`
	// 選手が任意で書ける選手情報（本体 #1013 補足）。空白だけ・未入力は NULL で保存する。
	PB            *string `json:"pb"`
	GoalTime      *string `json:"goal_time"`
	WindSpeed     *string `json:"wind_speed"`
	BodyCondition *string `json:"body_condition"`
	// いま意識して取り組んでいること（本体 #1093）。任意・300字まで。空白だけ・未入力は NULL。
	FocusNote *string `json:"focus_note"`
	// プラン（standard／premium。空はスタンダード）と、プレミアムの動画（本体 #1053）。スタンダードでは videos を使わない。
	Plan   string            `json:"plan"`
	Videos []premiumVideoReq `json:"videos"`
	// 18歳未満が自分で払うときの、購入のたびの宣言（minor_purchase.go）。
	minorDeclarationReq
}

func (s *Server) createReviewRequest(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	// 招待コードのゲートは、支払いのない検証用（Stripe の鍵がない環境）のデモ版にだけ残す。
	// 支払いがある環境では、正規の購入（単発・チケット）で誰でも申し込める（本体 #569）。
	if !s.reviewOpenTo(p) {
		Error(w, http.StatusForbidden, "forbidden", "スタンダードは準備中です")
		return
	}
	var req reviewRequestReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	switch req.Plan {
	case planPremium:
		s.createPremiumReviewRequest(w, r, p, req)
		return
	case "", planStandard:
	default:
		FieldError(w, map[string]string{"plan": "プランを選び直してください"})
		return
	}
	fields := map[string]string{}
	if !validCoachTier(req.Tier) {
		fields["tier"] = "枠を選んでください"
	}
	if !validReviewEvent(req.Event) {
		fields["event"] = "いまは100m・200m・400mだけ受け付けています"
	}
	focus := []string{}
	question := ""
	if req.Question != nil || req.VideoPublicID == "" {
		if req.Question != nil {
			question = strings.TrimSpace(*req.Question)
		}
		if question == "" {
			fields["question"] = "相談内容を記入してください"
		}
		if utf8.RuneCountInString(question) > 1000 {
			fields["question"] = "1000文字以内で記入してください"
		} else if looksLikeContact(question) {
			// 外部の連絡先への誘導を防ぐため、相談内容にも他の接点と同じ判定をかける（本体 #485）。
			fields["question"] = "URL・メールアドレス・電話番号・SNS の ID など、連絡先らしきものは書けません"
		}
	} else {
		var known bool
		focus, known = normalizeFocus(req.Focus)
		if !known || len(focus) == 0 {
			fields["focus"] = "見てほしい点を1つ以上選んでください"
		}
	}
	// 選手情報は、相談内容の経路・動画のみの経路のどちらでも、値があれば長さと連絡先だけを確かめる。
	pb := optionalRequestText(fields, "pb", req.PB, 100)
	goalTime := optionalRequestText(fields, "goal_time", req.GoalTime, 100)
	windSpeed := optionalRequestText(fields, "wind_speed", req.WindSpeed, 50)
	bodyCondition := optionalRequestText(fields, "body_condition", req.BodyCondition, 300)
	focusNote := optionalRequestText(fields, "focus_note", req.FocusNote, 300)
	// 申込には動画が必須（#1036）。動画なしの依頼は新しく作れない（過去の動画なし依頼の閲覧・返答は変えない）。
	if req.VideoPublicID == "" || !cloudinary.ValidCoachingPublicID(req.VideoPublicID) {
		fields["video_public_id"] = "動画を選んでください"
	}
	useTickets := req.Method == "tickets" || req.UseTickets
	if !validTicketKind(req.TicketKind) || (req.TicketKind != "" && !useTickets) {
		fields["ticket_kind"] = "チケットの種類を選び直してください"
	}
	useCoupon := req.UseCoupon || req.CouponID != nil
	if useCoupon && useTickets {
		fields["use_coupon"] = "クーポンとチケットは一緒に使えません"
	}
	// 特集モードでは、コーチを選ばずに送った依頼も特集コーチへの指名として扱う（本体 #985）。指名の確認は下の候補で行う。
	if featured, on := s.featuredCoach(); on && req.RequestedCoachID == nil {
		req.RequestedCoachID = &featured
	}
	if req.RequestedCoachID == nil && !s.cfg.ReviewRandomAssign {
		fields["requested_coach_id"] = "コーチを選んでください"
	}
	coachName := ""
	if req.RequestedCoachID != nil && len(fields) == 0 {
		nominees, err := s.reviewNominees(r.Context(), s.q, p.ID, req.Event, req.RequestedCoachID)
		if err != nil {
			MapDBError(w, err)
			return
		}
		if len(nominees) != 1 {
			FieldError(w, map[string]string{"requested_coach_id": "このコーチはいま依頼を受け付けていません。別のコーチを選んでください"})
			return
		}
		coachName = nominees[0].Nickname
	}
	// 料金は指名したコーチのいま有効なスタンダード価格（本体 #1052。未設定のコーチは reviewPriceYen）。
	// チケットで払うときは現金の支払いはなく、報酬の元になる額も従来どおり reviewPriceYen（チケットの扱いは変えない）。
	// クーポンを使うときは価格から額面を引いた額を払う（0円になれば支払いはない。本体 #1031）。
	basePrice := reviewPriceYen
	if !useTickets && len(fields) == 0 {
		price, err := s.coachStandardPriceYen(r.Context(), s.q, req.RequestedCoachID)
		if err != nil {
			MapDBError(w, err)
			return
		}
		basePrice = price
	}
	// 添削の追加項目（本体 #1086）。指名したコーチ（特集モードの自動指名を含む）のいま有効なタイトルを依頼に固定する。
	// コーチ未定の依頼（ReviewRandomAssign）は空のまま（回答の欄は固定の4項目だけ）。
	offerItems := []string{}
	if req.RequestedCoachID != nil && len(fields) == 0 {
		items, err := s.coachOfferItemsFor(r.Context(), req.RequestedCoachID)
		if err != nil {
			MapDBError(w, err)
			return
		}
		offerItems = items
	}
	cashPrice := int32(0)
	couponYen := int32(0)
	if useCoupon {
		couponYen = couponDiscount(basePrice, referralCouponYen)
	}
	if !useTickets {
		cashPrice = couponCashPrice(basePrice, couponYen)
	}
	payments := reviewNeedsPayment(s.stripe.Configured(), cashPrice)
	if payments && !validReviewPayMethod(req.Method) {
		fields["method"] = "支払い方法を選んでください"
	}
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	// 購入前チェックは、未成年（の可能性がある人）の支払いだけで求める（本体 #1045）。
	if err := requirePurchaseNotice(payments && purchaseNoticeRequired(p, time.Now()), req.GuardianPermission); err != nil {
		writeCheckoutError(w, err)
		return
	}
	var video *string
	if req.VideoPublicID != "" {
		if !s.uploadIssuedToMe(w, r, p.ID, req.VideoPublicID, "video_public_id", "動画") {
			return
		}
		if err := s.cloud.CoachingVideoReady(r.Context(), req.VideoPublicID); err != nil {
			writeAssetReadyError(w, err, "動画")
			return
		}
		video = &req.VideoPublicID
	}
	// 目安は送る画面に出したものと同じ計算で決め、依頼に保存する。これを過ぎたら expired になる。
	due, _, err := s.estimateReviewDue(r.Context(), p.ID, req.Tier, req.Event, time.Now())
	if err != nil {
		MapDBError(w, err)
		return
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := s.q.WithTx(tx)
	if req.RequestedCoachID != nil {
		if err := q.LockReviewAssignment(r.Context()); err != nil {
			MapDBError(w, err)
			return
		}
		nominees, err := s.reviewNominees(r.Context(), q, p.ID, req.Event, req.RequestedCoachID)
		if err != nil {
			MapDBError(w, err)
			return
		}
		if len(nominees) != 1 {
			Error(w, http.StatusConflict, "nomination_changed", "このコーチの受付状況が変わりました。別のコーチを選んでください")
			return
		}
	}
	if useTickets || useCoupon {
		// 同時に送られた2本が、どちらも「まだ0本」と読んで作られないように、ロックを取ってから確かめ直す。
		if err := q.LockReviewRequester(r.Context(), p.ID); err != nil {
			MapDBError(w, err)
			return
		}
	}
	if err := lockUnusedCoachingVideo(r.Context(), q, video, nil); err != nil {
		var rule *ticketRuleError
		if errors.As(err, &rule) {
			Error(w, 409, "conflict", rule.message)
		} else {
			MapDBError(w, err)
		}
		return
	}
	rr, err := q.CreateReviewRequest(r.Context(), db.CreateReviewRequestParams{
		ID:               uuid.New(),
		RequesterID:      p.ID,
		Tier:             req.Tier,
		Event:            req.Event,
		Focus:            focus,
		Question:         question,
		RequestedCoachID: req.RequestedCoachID, RequestedCoachName: coachName,
		BasePriceYen: basePrice, UseTickets: useTickets, CouponDiscountYen: couponYen,
		Pb: pb, GoalTime: goalTime, WindSpeed: windSpeed, BodyCondition: bodyCondition,
		FocusNote:     focusNote,
		VideoPublicID: video,
		DueAt:         due,
		// 支払いがあるときは、支払いが届いてから割り当てを始める。
		Status: map[bool]string{true: "awaiting_payment", false: "queued"}[payments],
		Plan:   planStandard,
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		Error(w, http.StatusConflict, "conflict", "この動画はもう送っています（お支払いがまだなら、送った依頼の一覧から払えます）")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	if len(offerItems) > 0 {
		if err := q.SetReviewRequestOfferItems(r.Context(), db.SetReviewRequestOfferItemsParams{ID: rr.ID, Items: offerItems}); err != nil {
			MapDBError(w, err)
			return
		}
		rr.OfferItems = offerItems
	}
	if useTickets {
		if err := s.reserveReviewTickets(r.Context(), q, p, rr, req.TicketKind); err != nil {
			writeCheckoutError(w, err)
			return
		}
	}
	if useCoupon {
		if err := reserveReviewCoupon(r.Context(), q, p.ID, req.CouponID, rr); err != nil {
			writeCheckoutError(w, err)
			return
		}
	}
	if !payments {
		rr, err = s.assignReview(r.Context(), q, rr)
		if err != nil {
			MapDBError(w, err)
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		MapDBError(w, err)
		return
	}
	s.finishReviewRequestCreate(w, r, p, rr, payments, req)
}

// finishReviewRequestCreate は、作った依頼の支払い画面を作る（払うものがなければ通知と訳を済ませる）までを行い、応答を書く。
// スタンダードとプレミアム（本体 #1053）の両方がここを通る。
func (s *Server) finishReviewRequestCreate(w http.ResponseWriter, r *http.Request, p db.Profile, rr db.ReviewRequest, payments bool, req reviewRequestReq) {
	if payments {
		url, err := s.reviewCheckoutFor(r.Context(), rr, p, req.Method, req.ReferralCode, req.minorDeclarationReq)
		if err != nil {
			// 支払い画面を作れなかったら、同じ動画で送り直せるように依頼を消す。
			if derr := s.tx(r.Context(), func(q *db.Queries) error {
				if t, e := q.GetTicketReservationForReview(r.Context(), &rr.ID); e == nil {
					if e = settleReviewTickets(r.Context(), q, t, false); e != nil {
						return e
					}
				} else if !errors.Is(e, pgx.ErrNoRows) {
					return e
				}
				if e := releaseReviewCouponFor(r.Context(), q, rr.ID); e != nil {
					return e
				}
				return q.DeleteAwaitingReviewRequest(r.Context(), rr.ID)
			}); derr != nil {
				slog.Warn("delete unpaid review request", "err", derr)
			}
			writeCheckoutError(w, err)
			return
		}
		out := s.reviewRequestJSON(rr)
		out["checkout_url"] = url
		if err := s.withPremium(r.Context(), s.q, out, rr, true); err != nil {
			MapDBError(w, err)
			return
		}
		JSON(w, http.StatusCreated, out)
		return
	}
	s.sendPendingOutbox(r.Context(), pushOutboxBatch)
	// 指名でコーチが決まった依頼は、ここで相談文を訳す（海外コーチのときだけ。失敗しても依頼は成功のまま）。
	if rr.CoachID != nil {
		s.translateReviewAfterWrite(r.Context(), rr, true, false, false)
	}
	out := s.reviewRequestJSON(rr)
	s.withReviewTranslations(r.Context(), out, rr)
	if err := s.withPremium(r.Context(), s.q, out, rr, true); err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusCreated, out)
}

func (s *Server) listMyReviewRequests(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	page, per, offset := s.queryPage(r)
	total, err := s.q.CountMyReviewRequests(r.Context(), p.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	rows, err := s.q.ListMyReviewRequests(r.Context(), db.ListMyReviewRequestsParams{RequesterID: p.ID, Limit: per, Offset: offset})
	if err != nil {
		MapDBError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, rr := range rows {
		out = append(out, myReviewRequestListItemJSON(rr))
	}
	JSON(w, http.StatusOK, map[string]any{"requests": out, "meta": meta{Page: int(page), PerPage: int(per), Total: total}})
}

// myReviewRequestListItemJSON は、選手の依頼一覧の1行。一覧ページが使う項目だけを返す（本体 #1072）。
// 6項目に、プレミアムの印に使う plan（本体 #1053）を加えた7項目。詳細と作成の応答は reviewRequestJSON のまま全項目を返す。
func myReviewRequestListItemJSON(rr db.ReviewRequest) map[string]any {
	return map[string]any{
		"id":          rr.ID,
		"plan":        rr.Plan,
		"event":       rr.Event,
		"status":      rr.Status,
		"canceled_at": rr.CanceledAt,
		"withdrawal":  withdrawalJSON(rr),
		"created_at":  rr.CreatedAt,
	}
}

// getReviewRequest は、依頼した本人にだけ依頼を見せる（他の人には無いものとして返す）。
func (s *Server) getReviewRequest(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	rr, err := s.q.GetReviewRequest(r.Context(), id)
	if err == nil && rr.RequesterID != p.ID {
		err = pgx.ErrNoRows
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	// tick が遅れていても、目安を過ぎた依頼は開いた時点で期限切れにする。
	// 返金や報酬の記録まで含む tick 全体はここでは回さない（Scheduler の tick と重ならないように）。
	// 開いた人が通信を切っても途中で止まらないよう、リクエストの取り消しは引き継がない。
	overdue := reviewWaiting(rr.Status) && !time.Now().Before(rr.DueAt)
	// 再提出期限を過ぎた差し戻しも、開いた時点で終える。
	unresubmitted := rr.Status == "needs_info" && rr.ResubmitBy != nil && !time.Now().Before(*rr.ResubmitBy)
	if unresubmitted {
		if _, err := s.expireUnresubmittedReviews(context.WithoutCancel(r.Context())); err != nil {
			slog.Warn("expire unresubmitted reviews on read", "err", err)
		} else if fresh, err := s.q.GetReviewRequest(context.WithoutCancel(r.Context()), id); err == nil {
			rr = fresh
		}
	}
	if overdue {
		ctx := context.WithoutCancel(r.Context())
		if overdue, err := s.expireOverdueReviews(ctx); err != nil {
			slog.Warn("expire overdue reviews on read", "err", err)
		} else {
			for _, o := range overdue {
				s.notifyReviewExpired(ctx, o)
			}
			if fresh, err := s.q.GetReviewRequest(ctx, id); err == nil {
				rr = fresh
			}
		}
	}
	w, read := s.coachingReadOnSuccess(w, r, p.ID, "player", &id, "/coaching/requests/"+id.String())
	defer read()
	s.writeRequesterReview(w, r, rr)
}

func (s *Server) createDemoCode(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	// 重なる確率はほぼないが、重なったら作り直す。
	for range 3 {
		code, err := newDemoCode()
		if err != nil {
			Error(w, http.StatusInternalServerError, "internal_error", "サーバーエラーが発生しました")
			return
		}
		row, err := s.q.CreateDemoCode(r.Context(), code)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			continue
		}
		if err != nil {
			MapDBError(w, err)
			return
		}
		JSON(w, http.StatusCreated, map[string]any{"code": row.Code, "created_at": row.CreatedAt, "used_at": nil, "used_by_nickname": nil})
		return
	}
	Error(w, http.StatusInternalServerError, "internal_error", "サーバーエラーが発生しました")
}

func (s *Server) listDemoCodes(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	rows, err := s.q.ListDemoCodes(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	if rows == nil {
		rows = []db.ListDemoCodesRow{}
	}
	JSON(w, http.StatusOK, map[string]any{"codes": rows})
}
