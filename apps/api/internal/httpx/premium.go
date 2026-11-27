package httpx

import (
	"context"
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

// プレミアム（本体 #1053）。決まった5枠（レース全体／スタート／後半／練習／その他）に、申込全体で1〜10本の動画を送り、
// コーチが総括（全体の一言診断＋動画がある枠ごとのブロック）で返す。
// 決済・クーポン・未成年チェック・返却期限・対応中の扱いはスタンダードと同じ仕組みを通す。チケットは使えない。
// プレミアムで申し込めるのは、プレミアム価格（価格申請制で有効になった価格）があるコーチだけ。

// プランの値は価格申請制（本体 #1052）と同じ standard／premium を使う。
const (
	planStandard = coachPlanStandard
	planPremium  = coachPlanPremium

	maxPremiumVideos = 10
	// 総括の枠ごとのブロックの上限。全体の一言診断はスタンダードと同じ300字。
	maxPremiumSectionChars = 1000
)

// premiumSlots は、プレミアムの決まった5枠。並びは画面に出す順（総括のブロックもこの順）。
var premiumSlots = []string{"race", "start", "second_half", "practice", "other"}

var premiumSlotLabels = map[string]string{
	"race": "レース全体", "start": "スタート", "second_half": "後半", "practice": "練習", "other": "その他",
}

func validPremiumSlot(slot string) bool {
	return oneOf(slot, premiumSlots)
}

func stringOrNil(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

// premiumVideoReq は、プレミアムの申込の動画1本。枠ごとの入力はすべて任意。
// レース全体の動画だけ race_time（この時のタイム）・wind_speed・body_condition、練習の動画だけ practice_purpose を持てる。
type premiumVideoReq struct {
	Slot            string  `json:"slot"`
	VideoPublicID   string  `json:"video_public_id"`
	RaceTime        *string `json:"race_time"`
	WindSpeed       *string `json:"wind_speed"`
	BodyCondition   *string `json:"body_condition"`
	PracticePurpose *string `json:"practice_purpose"`
}

type premiumVideo struct {
	slot, publicID                                      string
	raceTime, windSpeed, bodyCondition, practicePurpose *string
}

// validatePremiumVideos は、動画の本数・枠・ID と枠ごとの入力を確かめる。誤りは fields に入れる。
func validatePremiumVideos(fields map[string]string, videos []premiumVideoReq) []premiumVideo {
	switch {
	case len(videos) == 0:
		fields["videos"] = "動画を1本以上追加してください"
		return nil
	case len(videos) > maxPremiumVideos:
		fields["videos"] = fmt.Sprintf("動画は全部で%d本までです", maxPremiumVideos)
		return nil
	}
	seen := map[string]bool{}
	out := make([]premiumVideo, 0, len(videos))
	for i, v := range videos {
		key := fmt.Sprintf("videos.%d", i)
		if !validPremiumSlot(v.Slot) {
			fields[key+".slot"] = "枠を選んでください"
			continue
		}
		if v.VideoPublicID == "" || !cloudinary.ValidCoachingPublicID(v.VideoPublicID) {
			fields[key+".video_public_id"] = "動画を選んでください"
			continue
		}
		if seen[v.VideoPublicID] {
			fields[key+".video_public_id"] = "同じ動画は1回だけ追加できます"
			continue
		}
		seen[v.VideoPublicID] = true
		pv := premiumVideo{slot: v.Slot, publicID: v.VideoPublicID}
		if v.Slot == "race" {
			pv.raceTime = optionalRequestText(fields, key+".race_time", v.RaceTime, 100)
			pv.windSpeed = optionalRequestText(fields, key+".wind_speed", v.WindSpeed, 50)
			pv.bodyCondition = optionalRequestText(fields, key+".body_condition", v.BodyCondition, 300)
		} else if optionalRequestText(map[string]string{}, "", v.RaceTime, 100) != nil ||
			optionalRequestText(map[string]string{}, "", v.WindSpeed, 50) != nil ||
			optionalRequestText(map[string]string{}, "", v.BodyCondition, 300) != nil {
			fields[key+".race_time"] = "この時のタイム・風速・コンディションは、レース全体の動画にだけ書けます"
		}
		if v.Slot == "practice" {
			pv.practicePurpose = optionalRequestText(fields, key+".practice_purpose", v.PracticePurpose, 300)
		} else if optionalRequestText(map[string]string{}, "", v.PracticePurpose, 300) != nil {
			fields[key+".practice_purpose"] = "練習の目的は、練習の動画にだけ書けます"
		}
		out = append(out, pv)
	}
	return out
}

// premiumPriceYen は、コーチのプレミアム有効価格（価格申請制で承認・切り替え済みの価格。本体 #1052）。
// 価格がなければ ok=false（プレミアムを受けられない）。
func (s *Server) premiumPriceYen(ctx context.Context, q *db.Queries, coachID uuid.UUID) (int32, bool, error) {
	price, err := s.coachPremiumPriceYen(ctx, q, coachID)
	if err != nil || price == nil {
		return 0, false, err
	}
	return *price, true, nil
}

// premiumNominee は、プレミアムで申し込めるコーチ（スタンダードと同じ受付条件＋プレミアム価格がある）とその価格。
func (s *Server) premiumNominee(ctx context.Context, q *db.Queries, requester uuid.UUID, event string, coach uuid.UUID) (db.ListReviewNomineesRow, int32, bool, error) {
	nominees, err := s.reviewNominees(ctx, q, requester, event, &coach)
	if err != nil || len(nominees) != 1 {
		return db.ListReviewNomineesRow{}, 0, false, err
	}
	price, ok, err := s.premiumPriceYen(ctx, q, coach)
	if err != nil || !ok {
		return db.ListReviewNomineesRow{}, 0, false, err
	}
	return nominees[0], price, true, nil
}

// createPremiumReviewRequest は、プレミアムの申込。createReviewRequest から plan=premium のときに呼ばれる。
func (s *Server) createPremiumReviewRequest(w http.ResponseWriter, r *http.Request, p db.Profile, req reviewRequestReq) {
	fields := map[string]string{}
	if !validCoachTier(req.Tier) {
		fields["tier"] = "枠を選んでください"
	}
	if !validReviewEvent(req.Event) {
		fields["event"] = "いまは100m・200m・400mだけ受け付けています"
	}
	// PB・目標タイムは申込全体に1回。風速・コンディションは申込全体には持たず、レース全体の動画ごとに持つ。
	pb := optionalRequestText(fields, "pb", req.PB, 100)
	goalTime := optionalRequestText(fields, "goal_time", req.GoalTime, 100)
	videos := validatePremiumVideos(fields, req.Videos)
	useTickets := req.Method == "tickets" || req.UseTickets || req.TicketKind != ""
	if useTickets {
		fields["method"] = "プレミアムはチケットでは申し込めません。カードかクーポンで申し込んでください"
	}
	useCoupon := req.UseCoupon || req.CouponID != nil
	if featured, on := s.featuredCoach(); on && req.RequestedCoachID == nil {
		req.RequestedCoachID = &featured
	}
	if req.RequestedCoachID == nil {
		fields["requested_coach_id"] = "コーチを選んでください"
	}
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	nominee, price, ok, err := s.premiumNominee(r.Context(), s.q, p.ID, req.Event, *req.RequestedCoachID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if !ok {
		FieldError(w, map[string]string{"requested_coach_id": "このコーチはいまプレミアムを受け付けていません。別のコーチを選んでください"})
		return
	}
	couponYen := int32(0)
	if useCoupon {
		couponYen = couponDiscount(price, referralCouponYen)
	}
	cashPrice := couponCashPrice(price, couponYen)
	payments := reviewNeedsPayment(s.stripe.Configured(), cashPrice)
	if payments && !validReviewPayMethod(req.Method) {
		FieldError(w, map[string]string{"method": "支払い方法を選んでください"})
		return
	}
	if err := requirePurchaseNotice(payments && purchaseNoticeRequired(p, time.Now()), req.GuardianPermission); err != nil {
		writeCheckoutError(w, err)
		return
	}
	for i, v := range videos {
		field := fmt.Sprintf("videos.%d.video_public_id", i)
		if !s.uploadIssuedToMe(w, r, p.ID, v.publicID, field, "動画") {
			return
		}
		if err := s.cloud.CoachingVideoReady(r.Context(), v.publicID); err != nil {
			writeAssetReadyError(w, err, "動画")
			return
		}
	}
	// 返却期限はスタンダードと同じ計算（プレミアム専用の期限は設けない）。
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
	if err := q.LockReviewAssignment(r.Context()); err != nil {
		MapDBError(w, err)
		return
	}
	// ロックを取ってから、受付状況と価格が変わっていないかを確かめ直す。
	_, lockedPrice, ok, err := s.premiumNominee(r.Context(), q, p.ID, req.Event, *req.RequestedCoachID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if !ok {
		Error(w, http.StatusConflict, "nomination_changed", "このコーチの受付状況が変わりました。別のコーチを選んでください")
		return
	}
	if lockedPrice != price {
		Error(w, http.StatusConflict, "price_changed", "このコーチのプレミアムの価格が変わりました。画面を更新してください")
		return
	}
	if useCoupon {
		if err := q.LockReviewRequester(r.Context(), p.ID); err != nil {
			MapDBError(w, err)
			return
		}
	}
	for _, v := range videos {
		id := v.publicID
		if err := lockUnusedCoachingVideo(r.Context(), q, &id, nil); err != nil {
			var rule *ticketRuleError
			if errors.As(err, &rule) {
				Error(w, http.StatusConflict, "conflict", rule.message)
			} else {
				MapDBError(w, err)
			}
			return
		}
	}
	rr, err := q.CreateReviewRequest(r.Context(), db.CreateReviewRequestParams{
		ID:               uuid.New(),
		RequesterID:      p.ID,
		Tier:             req.Tier,
		Event:            req.Event,
		Focus:            []string{},
		Question:         "",
		RequestedCoachID: req.RequestedCoachID, RequestedCoachName: nominee.Nickname,
		BasePriceYen: price, UseTickets: false, CouponDiscountYen: couponYen,
		Pb: pb, GoalTime: goalTime,
		DueAt:  due,
		Status: map[bool]string{true: "awaiting_payment", false: "queued"}[payments],
		Plan:   planPremium,
	})
	if err != nil {
		MapDBError(w, err)
		return
	}
	for i, v := range videos {
		_, err := q.CreateReviewRequestVideo(r.Context(), db.CreateReviewRequestVideoParams{
			RequestID: rr.ID, Position: int16(i + 1), Slot: v.slot, VideoPublicID: v.publicID,
			RaceTime: v.raceTime, WindSpeed: v.windSpeed, BodyCondition: v.bodyCondition, PracticePurpose: v.practicePurpose,
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

// premiumVideosJSON は、プレミアムの動画を枠ごとにまとめた形。枠は決まった順、枠内は追加した順。動画がない枠は出さない。
// withVideo が false（コーチがまだ動画を見られない状態）や保持期限で消した後は、URL を null にする。
func (s *Server) premiumVideosJSON(rows []db.ReviewRequestVideo, withVideo, purged bool) []map[string]any {
	groups := make([]map[string]any, 0, len(premiumSlots))
	for _, slot := range premiumSlots {
		items := []map[string]any{}
		for _, v := range rows {
			if v.Slot != slot {
				continue
			}
			item := map[string]any{"position": v.Position, "video_url": nil, "thumbnail_url": nil}
			if withVideo && !purged {
				item["video_url"] = s.cloud.CoachingPlaybackURL(v.VideoPublicID)
				item["thumbnail_url"] = s.cloud.CoachingThumbnailURL(v.VideoPublicID)
			}
			switch slot {
			case "race":
				item["race_time"], item["wind_speed"], item["body_condition"] = v.RaceTime, v.WindSpeed, v.BodyCondition
			case "practice":
				item["practice_purpose"] = v.PracticePurpose
			}
			items = append(items, item)
		}
		if len(items) > 0 {
			groups = append(groups, map[string]any{"slot": slot, "videos": items})
		}
	}
	return groups
}

// withPremium は、プレミアムの依頼の JSON に枠ごとの動画（premium_videos）と総括のブロック（response.sections）を足す。
// スタンダードの依頼には何も足さない（plan だけは呼び出し側の JSON に入っている）。
func (s *Server) withPremium(ctx context.Context, q *db.Queries, out map[string]any, rr db.ReviewRequest, withVideo bool) error {
	if rr.Plan != planPremium {
		return nil
	}
	rows, err := q.ListReviewRequestVideos(ctx, rr.ID)
	if err != nil {
		return err
	}
	out["premium_videos"] = s.premiumVideosJSON(rows, withVideo, rr.VideosPurgedAt != nil)
	out["video_count"] = len(rows)
	resp, ok := out["response"].(map[string]any)
	if !ok || resp == nil || resp["hidden"] == true {
		return nil
	}
	sections, err := q.ListReviewResponseSections(ctx, rr.ID)
	if err != nil {
		return err
	}
	list := make([]map[string]any, 0, len(sections))
	for _, sec := range sections {
		list = append(list, map[string]any{"slot": sec.Slot, "body": sec.Body})
	}
	resp["sections"] = list
	return nil
}

type premiumSectionReq struct {
	Slot string `json:"slot"`
	Body string `json:"body"`
}

// validatePremiumSections は、総括の枠ごとのブロックを確かめる。動画がある枠すべてに1つずつ書き、動画がない枠には書けない。
func validatePremiumSections(fields map[string]string, sections []premiumSectionReq, videoSlots map[string]bool) []premiumSectionReq {
	bySlot := map[string]string{}
	for _, sec := range sections {
		body := strings.TrimSpace(sec.Body)
		switch {
		case !validPremiumSlot(sec.Slot):
			fields["sections"] = "枠を選び直してください"
		case !videoSlots[sec.Slot]:
			fields["sections."+sec.Slot] = "動画がない枠には書けません"
		case bySlot[sec.Slot] != "":
			fields["sections."+sec.Slot] = "同じ枠は1回だけ書けます"
		case body == "":
			// 空のブロックは、下の「書いていない枠」として扱う。
		case utf8.RuneCountInString(body) > maxPremiumSectionChars:
			fields["sections."+sec.Slot] = fmt.Sprintf("%sは%d文字以内で入力してください", premiumSlotLabels[sec.Slot], maxPremiumSectionChars)
		case looksLikeContact(body):
			fields["sections."+sec.Slot] = "URL・メールアドレス・電話番号・SNS の ID など、連絡先らしきものは書けません"
		default:
			bySlot[sec.Slot] = body
		}
	}
	out := make([]premiumSectionReq, 0, len(videoSlots))
	for _, slot := range premiumSlots {
		if !videoSlots[slot] {
			continue
		}
		if bySlot[slot] == "" {
			if _, bad := fields["sections."+slot]; !bad {
				fields["sections."+slot] = premiumSlotLabels[slot] + "を入力してください"
			}
			continue
		}
		out = append(out, premiumSectionReq{Slot: slot, Body: bySlot[slot]})
	}
	return out
}

// premiumVideoSlots は、依頼の動画がある枠の集合。
func premiumVideoSlots(rows []db.ReviewRequestVideo) map[string]bool {
	slots := map[string]bool{}
	for _, v := range rows {
		slots[v.Slot] = true
	}
	return slots
}

// premiumNomineesJSON は、プレミアムの指名候補（プレミアム価格があるコーチだけ）と価格。
func (s *Server) premiumNomineesJSON(ctx context.Context, rows []db.ListReviewNomineesRow) ([]map[string]any, error) {
	ids := make([]uuid.UUID, 0, len(rows))
	for _, c := range rows {
		ids = append(ids, c.ProfileID)
	}
	prices, err := s.effectiveCoachPrices(ctx, s.q, ids)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, c := range rows {
		price := prices[c.ProfileID].Premium
		if price == nil {
			continue
		}
		out = append(out, map[string]any{"id": c.ProfileID, "nickname": c.Nickname, "photo_url": s.coachPhotoURL(c.PhotoPublicID),
			"events": c.Events, "years": c.Years, "best_record": c.BestRecord, "achievements": c.Achievements, "premium_price_yen": *price})
	}
	return out, nil
}

// submitPremiumReviewResponse は、プレミアムの総括を返す。全体の一言診断（300字・必須）と、動画がある枠ごとのブロック
// （各1000字・必須）、任意の見本動画。返却期限・確定（capture）・クーポンの消化・通知はスタンダードの返却と同じ。
func (s *Server) submitPremiumReviewResponse(w http.ResponseWriter, r *http.Request, p db.Profile, current db.ReviewRequest, req reviewResponseReq) {
	rows, err := s.q.ListReviewRequestVideos(r.Context(), current.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	hitokoto := strings.TrimSpace(req.Hitokoto)
	fields := map[string]string{}
	if e := responseTextError("一言診断", hitokoto); e != "" {
		fields["hitokoto"] = e
	}
	sections := validatePremiumSections(fields, req.Sections, premiumVideoSlots(rows))
	var demo *string
	if req.DemoVideoPublicID != "" {
		if !cloudinary.ValidCoachingPublicID(req.DemoVideoPublicID) {
			fields["demo_video_public_id"] = "見本動画を選び直してください"
		} else {
			demo = &req.DemoVideoPublicID
		}
	}
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	if demo != nil {
		if !s.coachingVideoFree(w, r, *demo, "demo_video_public_id") {
			return
		}
		if !s.uploadIssuedToMe(w, r, p.ID, *demo, "demo_video_public_id", "見本動画") {
			return
		}
		if err := s.cloud.CoachingVideoReady(r.Context(), *demo); err != nil {
			writeAssetReadyError(w, err, "見本動画")
			return
		}
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := s.q.WithTx(tx)
	rr, err := q.MarkReviewReturned(r.Context(), db.MarkReviewReturnedParams{ID: current.ID, CoachID: &p.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusConflict, "conflict", "この依頼にはもう返答できません")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	if err := lockUnusedCoachingVideo(r.Context(), q, demo, nil); err != nil {
		writeCheckoutError(w, err)
		return
	}
	rs, err := q.CreatePremiumReviewResponse(r.Context(), db.CreatePremiumReviewResponseParams{
		RequestID: rr.ID, CoachID: &p.ID, Hitokoto: &hitokoto, DemoVideoPublicID: demo,
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		Error(w, http.StatusConflict, "conflict", "この見本動画はもう使っています")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	for _, sec := range sections {
		if err := q.CreateReviewResponseSection(r.Context(), db.CreateReviewResponseSectionParams{RequestID: rr.ID, Slot: sec.Slot, Body: sec.Body}); err != nil {
			MapDBError(w, err)
			return
		}
	}
	// プレミアムはチケットを使えないので、チケットの精算はない。クーポンは返却で使用済にする（本体 #1031 と同じ）。
	if err := consumeReviewCoupon(r.Context(), q, rr.ID); err != nil {
		MapDBError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		MapDBError(w, err)
		return
	}
	s.captureReturnedReview(r.Context(), rr.ID)
	if err := insertReviewNotification(r.Context(), s.q, rr.RequesterID, "review_returned", rr.ID); err != nil {
		slog.Warn("review notification", "kind", "review_returned", "err", err)
	}
	s.queuePush(r.Context(), rr.RequesterID, notifyReview, push.Message{
		Title: "プレミアムの総括が届きました",
		Body:  "コーチからの総括を見てみましょう",
		URL:   "/coaching/requests/" + rr.ID.String(),
		Tag:   "review-" + rr.ID.String(),
	}, 24*time.Hour, time.Now().Add(24*time.Hour), "review_returned:"+rr.ID.String())
	out := s.coachRequestJSON(rr, true)
	out["response"] = s.reviewResponseJSON(rs)
	if err := s.withPremium(r.Context(), s.q, out, rr, true); err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, out)
}
