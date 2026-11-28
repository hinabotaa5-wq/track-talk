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

// プレミアム（本体 #1053、補足 7231984993・補足2 7231991592 の再設計）。選手が動画をテーマごとのグループに分けて
// 申込全体で1〜6本を送り、コーチがグループごとの自由記入＋最後の「まとめ」で返す。制限は合計6本だけで、
// グループ数にもグループごとの本数にも制限はない（練習の動画だけ・1グループに6本全部でもよい）。
// 決済・クーポン・未成年チェック・返却期限・対応中の扱いはスタンダードと同じ仕組みを通す。チケットは使えない。
// プレミアムで申し込めるのは、プレミアム価格（価格申請制で有効になった価格）があるコーチだけ。

// プランの値は価格申請制（本体 #1052）と同じ standard／premium を使う。
const (
	planStandard = coachPlanStandard
	planPremium  = coachPlanPremium

	maxPremiumVideos = 6
	// グループのテーマ名と、グループごとの「何を意識して取り組んでいるか・何が目的か」の上限。
	maxPremiumGroupTitleChars = 30
	maxPremiumFocusNoteChars  = 300
	// 返却のグループごとの記入と、最後のまとめの上限。
	maxPremiumSectionChars = 2000
	maxPremiumSummaryChars = 2000
)

// premiumKinds は、グループのテーマのテンプレ（レース全体／スタート／後半／練習／その他）。並びは選ぶ画面の順。
// 全部を埋める義務はなく、同じテンプレのグループを何個作ってもよい。
var premiumKinds = []string{"race", "start", "second_half", "practice", "other"}

var premiumKindLabels = map[string]string{
	"race": "レース全体", "start": "スタート", "second_half": "後半", "practice": "練習", "other": "その他",
}

func validPremiumKind(kind string) bool {
	return oneOf(kind, premiumKinds)
}

func stringOrNil(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

// premiumVideoReq は、グループの中の動画1本。この時のタイム・風速・その時の体のコンディションは任意で、
// レース全体のグループの動画にだけ書ける。
type premiumVideoReq struct {
	VideoPublicID string  `json:"video_public_id"`
	RaceTime      *string `json:"race_time"`
	WindSpeed     *string `json:"wind_speed"`
	BodyCondition *string `json:"body_condition"`
}

// premiumGroupReq は、動画のグループ1つ。kind はテーマのテンプレ、title はテーマ名（空ならテンプレ名）、
// focus_note は何を意識して取り組んでいるか・何が目的か（任意、全グループ共通の欄）。
type premiumGroupReq struct {
	Kind      string            `json:"kind"`
	Title     *string           `json:"title"`
	FocusNote *string           `json:"focus_note"`
	Videos    []premiumVideoReq `json:"videos"`
}

type premiumVideo struct {
	publicID                           string
	raceTime, windSpeed, bodyCondition *string
}

type premiumGroup struct {
	kind, title string
	focusNote   *string
	videos      []premiumVideo
}

// validatePremiumGroups は、グループ（テンプレ・テーマ名・意識と目的）と動画（本数・ID・レースの入力）を確かめる。誤りは fields に入れる。
func validatePremiumGroups(fields map[string]string, groups []premiumGroupReq) []premiumGroup {
	total := 0
	for _, g := range groups {
		total += len(g.Videos)
	}
	switch {
	case total == 0:
		fields["videos"] = "動画を1本以上追加してください"
		return nil
	case total > maxPremiumVideos:
		fields["videos"] = fmt.Sprintf("動画は全部で%d本までです", maxPremiumVideos)
		return nil
	}
	seenVideo := map[string]bool{}
	seenTitle := map[string]bool{}
	out := make([]premiumGroup, 0, len(groups))
	for i, g := range groups {
		key := fmt.Sprintf("groups.%d", i)
		if !validPremiumKind(g.Kind) {
			fields[key+".kind"] = "テーマを選んでください"
			continue
		}
		if len(g.Videos) == 0 {
			fields[key+".videos"] = "グループに動画を1本以上入れてください"
			continue
		}
		title := premiumKindLabels[g.Kind]
		if g.Title != nil && strings.TrimSpace(*g.Title) != "" {
			title = strings.TrimSpace(*g.Title)
		}
		switch {
		case utf8.RuneCountInString(title) > maxPremiumGroupTitleChars:
			fields[key+".title"] = fmt.Sprintf("テーマ名は%d文字以内で記入してください", maxPremiumGroupTitleChars)
		case looksLikeContact(title):
			fields[key+".title"] = "URL・メールアドレス・電話番号・SNS の ID など、連絡先らしきものは書けません"
		case seenTitle[title]:
			// 返却の引用単位はテーマ名なので、同じ名前のグループは作らない。
			fields[key+".title"] = "同じテーマ名のグループがあります。テーマ名を変えてください"
		}
		seenTitle[title] = true
		pg := premiumGroup{kind: g.Kind, title: title, focusNote: optionalRequestText(fields, key+".focus_note", g.FocusNote, maxPremiumFocusNoteChars)}
		for j, v := range g.Videos {
			vkey := fmt.Sprintf("%s.videos.%d", key, j)
			if v.VideoPublicID == "" || !cloudinary.ValidCoachingPublicID(v.VideoPublicID) {
				fields[vkey+".video_public_id"] = "動画を選んでください"
				continue
			}
			if seenVideo[v.VideoPublicID] {
				fields[vkey+".video_public_id"] = "同じ動画は1回だけ追加できます"
				continue
			}
			seenVideo[v.VideoPublicID] = true
			pv := premiumVideo{publicID: v.VideoPublicID}
			if g.Kind == "race" {
				pv.raceTime = optionalRequestText(fields, vkey+".race_time", v.RaceTime, 100)
				pv.windSpeed = optionalRequestText(fields, vkey+".wind_speed", v.WindSpeed, 50)
				pv.bodyCondition = optionalRequestText(fields, vkey+".body_condition", v.BodyCondition, 300)
			} else if optionalRequestText(map[string]string{}, "", v.RaceTime, 100) != nil ||
				optionalRequestText(map[string]string{}, "", v.WindSpeed, 50) != nil ||
				optionalRequestText(map[string]string{}, "", v.BodyCondition, 300) != nil {
				fields[vkey+".race_time"] = "この時のタイム・風速・コンディションは、レース全体のグループの動画にだけ書けます"
			}
			pg.videos = append(pg.videos, pv)
		}
		out = append(out, pg)
	}
	return out
}

// premiumVideoIDs は、グループを通した動画の並び（申込で追加した順）。
func premiumVideoIDs(groups []premiumGroup) []premiumVideo {
	out := []premiumVideo{}
	for _, g := range groups {
		out = append(out, g.videos...)
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
	// PB・目標タイムは申込全体に1回。風速・コンディションは申込全体には持たず、レース全体のグループの動画ごとに持つ。
	// 意識していること（focus_note）も申込全体には持たず、グループごとに持つ。
	pb := optionalRequestText(fields, "pb", req.PB, 100)
	goalTime := optionalRequestText(fields, "goal_time", req.GoalTime, 100)
	groups := validatePremiumGroups(fields, req.Groups)
	videos := premiumVideoIDs(groups)
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
	position := int16(0)
	for gi, g := range groups {
		groupPosition := int16(gi + 1)
		if err := q.CreateReviewRequestVideoGroup(r.Context(), db.CreateReviewRequestVideoGroupParams{
			RequestID: rr.ID, Position: groupPosition, Kind: g.kind, Title: g.title, FocusNote: g.focusNote,
		}); err != nil {
			MapDBError(w, err)
			return
		}
		for _, v := range g.videos {
			position++
			_, err := q.CreateReviewRequestVideo(r.Context(), db.CreateReviewRequestVideoParams{
				RequestID: rr.ID, Position: position, GroupPosition: groupPosition, VideoPublicID: v.publicID,
				RaceTime: v.raceTime, WindSpeed: v.windSpeed, BodyCondition: v.bodyCondition,
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

// premiumGroupsJSON は、プレミアムの動画をグループごとにまとめた形。グループは選手が作った順、グループ内は追加した順。
// withVideo が false（コーチがまだ動画を見られない状態）や保持期限で消した後は、URL を null にする。
// レース全体のグループの動画にだけ、この時のタイム・風速・コンディションを出す。
func (s *Server) premiumGroupsJSON(groups []db.ReviewRequestVideoGroup, rows []db.ReviewRequestVideo, withVideo, purged bool) []map[string]any {
	out := make([]map[string]any, 0, len(groups))
	for _, g := range groups {
		items := []map[string]any{}
		for _, v := range rows {
			if v.GroupPosition != g.Position {
				continue
			}
			item := map[string]any{"position": v.Position, "video_url": nil, "thumbnail_url": nil}
			if withVideo && !purged {
				item["video_url"] = s.cloud.CoachingPlaybackURL(v.VideoPublicID)
				item["thumbnail_url"] = s.cloud.CoachingThumbnailURL(v.VideoPublicID)
			}
			if g.Kind == "race" {
				item["race_time"], item["wind_speed"], item["body_condition"] = v.RaceTime, v.WindSpeed, v.BodyCondition
			}
			items = append(items, item)
		}
		out = append(out, map[string]any{"position": g.Position, "kind": g.Kind, "title": g.Title, "focus_note": g.FocusNote, "videos": items})
	}
	return out
}

// withPremium は、プレミアムの依頼の JSON にグループごとの動画（premium_groups）と、返却のグループごとの記入
// （response.sections）・まとめ（response.summary）を足す。スタンダードの依頼には何も足さない（plan だけは呼び出し側の JSON に入っている）。
func (s *Server) withPremium(ctx context.Context, q *db.Queries, out map[string]any, rr db.ReviewRequest, withVideo bool) error {
	if rr.Plan != planPremium {
		return nil
	}
	groups, err := q.ListReviewRequestVideoGroups(ctx, rr.ID)
	if err != nil {
		return err
	}
	rows, err := q.ListReviewRequestVideos(ctx, rr.ID)
	if err != nil {
		return err
	}
	out["premium_groups"] = s.premiumGroupsJSON(groups, rows, withVideo, rr.VideosPurgedAt != nil)
	out["video_count"] = len(rows)
	resp, ok := out["response"].(map[string]any)
	if !ok || resp == nil || resp["hidden"] == true {
		return nil
	}
	sections, err := q.ListReviewResponseSections(ctx, rr.ID)
	if err != nil {
		return err
	}
	titles := map[int16]string{}
	for _, g := range groups {
		titles[g.Position] = g.Title
	}
	list := make([]map[string]any, 0, len(sections))
	for _, sec := range sections {
		list = append(list, map[string]any{"group_position": sec.GroupPosition, "title": titles[sec.GroupPosition], "body": sec.Body})
	}
	resp["sections"] = list
	return nil
}

type premiumSectionReq struct {
	GroupPosition int16  `json:"group_position"`
	Body          string `json:"body"`
}

type premiumSection struct {
	groupPosition int16
	body          string
}

// validatePremiumSections は、返却のグループごとの記入を確かめる。動画があるグループすべてに1つずつ書き、
// 無いグループには書けない。項目分けはしない自由記入（グループ名が見出し・引用単位）。
func validatePremiumSections(fields map[string]string, sections []premiumSectionReq, groups []db.ReviewRequestVideoGroup) []premiumSection {
	known := map[int16]string{}
	for _, g := range groups {
		known[g.Position] = g.Title
	}
	byGroup := map[int16]string{}
	for _, sec := range sections {
		key := fmt.Sprintf("sections.%d", sec.GroupPosition)
		body := strings.TrimSpace(sec.Body)
		switch {
		case known[sec.GroupPosition] == "":
			fields[key] = "動画のグループを選び直してください"
		case byGroup[sec.GroupPosition] != "":
			fields[key] = "同じグループは1回だけ書けます"
		case body == "":
			// 空の記入は、下の「書いていないグループ」として扱う。
		case utf8.RuneCountInString(body) > maxPremiumSectionChars:
			fields[key] = fmt.Sprintf("「%s」は%d文字以内で入力してください", known[sec.GroupPosition], maxPremiumSectionChars)
		case looksLikeContact(body):
			fields[key] = "URL・メールアドレス・電話番号・SNS の ID など、連絡先らしきものは書けません"
		default:
			byGroup[sec.GroupPosition] = body
		}
	}
	out := make([]premiumSection, 0, len(groups))
	for _, g := range groups {
		key := fmt.Sprintf("sections.%d", g.Position)
		if byGroup[g.Position] == "" {
			if _, bad := fields[key]; !bad {
				fields[key] = "「" + g.Title + "」を入力してください"
			}
			continue
		}
		out = append(out, premiumSection{groupPosition: g.Position, body: byGroup[g.Position]})
	}
	return out
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

// submitPremiumReviewResponse は、プレミアムの返却。動画があるグループごとの自由記入（各2000字・必須）と、最後のまとめ
// （2000字・必須）、任意の見本動画。返却期限・確定（capture）・クーポンの消化・通知はスタンダードの返却と同じ。
func (s *Server) submitPremiumReviewResponse(w http.ResponseWriter, r *http.Request, p db.Profile, current db.ReviewRequest, req reviewResponseReq) {
	groups, err := s.q.ListReviewRequestVideoGroups(r.Context(), current.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	summary := strings.TrimSpace(req.Summary)
	fields := map[string]string{}
	switch {
	case summary == "":
		fields["summary"] = "まとめを入力してください"
	case utf8.RuneCountInString(summary) > maxPremiumSummaryChars:
		fields["summary"] = fmt.Sprintf("まとめは%d文字以内で入力してください", maxPremiumSummaryChars)
	case looksLikeContact(summary):
		fields["summary"] = "URL・メールアドレス・電話番号・SNS の ID など、連絡先らしきものは書けません"
	}
	sections := validatePremiumSections(fields, req.Sections, groups)
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
		RequestID: rr.ID, CoachID: &p.ID, Summary: &summary, DemoVideoPublicID: demo,
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
		if err := q.CreateReviewResponseSection(r.Context(), db.CreateReviewResponseSectionParams{RequestID: rr.ID, GroupPosition: sec.groupPosition, Body: sec.body}); err != nil {
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
		Title: "プレミアムの回答が届きました",
		Body:  "コーチからの回答を見てみましょう",
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
