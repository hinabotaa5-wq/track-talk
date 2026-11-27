package httpx

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/cloudinary"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/push"
	"github.com/jackc/pgx/v5"
)

// コーチの添削項目（追加項目）と見本動画の変更申請（本体 #1086、運営者決定 2026-11-27）。
// 対象はスタンダードだけ。固定の4項目（回答の型 #1067）は全コーチ共通でコーチは減らせず、
// コーチが申請で増減できるのは追加項目（最大5件・各30字）と、コーチ詳細に常設する見本動画（最大3本）だけ。
// 申請・承認・切り替えは価格申請（coach_price.go）と同じ：承認は全件（却下なし。承認は確認・記録の場）、
// 承認の時点で対応中だった依頼がすべて終わってから有効になる（ApplyDueCoachOffers）。
// 有効な追加項目は依頼の作成時に review_requests.offer_items に固定し、回答の欄はそれに従う（各300字・必須）。

const (
	maxCoachOfferItems     = 5
	maxCoachOfferItemChars = 30
	maxCoachDemoVideos     = 3
)

// fixedOfferTitles は、全コーチの回答に入っている固定の4項目の正式名称（並びは回答の型どおり）。
var fixedOfferTitles = []string{"良い点", "改善点", "おすすめのドリル", "コーチなら明日から何をするか"}

// reservedOfferTitles は、追加項目のタイトルにできない文字列。固定4項目の正式名称と、
// 固定4項目目の言い換え（旧入力例。Issue 補足2 T2）。
var reservedOfferTitles = append(append([]string{}, fixedOfferTitles...), "コーチだったら明日から何をするか")

// coachOfferItemsError は、追加項目のタイトルを確かめる。前後の空白を落としたタイトルと、問題があればその説明を返す。
func coachOfferItemsError(items []string) ([]string, string) {
	if len(items) > maxCoachOfferItems {
		return nil, "追加項目は5件までです"
	}
	out := make([]string, 0, len(items))
	seen := map[string]bool{}
	for _, raw := range items {
		title := strings.TrimSpace(raw)
		switch {
		case title == "":
			return nil, "追加項目のタイトルを入力してください"
		case utf8.RuneCountInString(title) > maxCoachOfferItemChars:
			return nil, "追加項目のタイトルは30文字以内で入力してください"
		case oneOf(title, reservedOfferTitles):
			return nil, "「" + title + "」はすべての回答に入っている項目なので、追加項目にはできません"
		case seen[title]:
			return nil, "同じタイトルの追加項目が重なっています"
		case looksLikeContact(title):
			return nil, "URL・メールアドレス・電話番号・SNS の ID など、連絡先らしきものは書けません"
		}
		seen[title] = true
		out = append(out, title)
	}
	return out, ""
}

// coachOfferSet は、コーチのいま有効な追加項目と見本動画。行がないコーチは空。
type coachOfferSet struct {
	Items         []string
	Videos        []string
	ApplicationID *uuid.UUID
}

// applyDueCoachOffers は、切り替えの条件を満たした承認済みの申請を有効にし、コーチに知らせる。
// coachIDs が空なら全コーチ（tick）。同じ申請は1回しか切り替わらないので、知らせも1回だけ届く。
func (s *Server) applyDueCoachOffers(ctx context.Context, coachIDs ...uuid.UUID) (int, error) {
	if coachIDs == nil {
		coachIDs = []uuid.UUID{}
	}
	rows, err := s.q.ApplyDueCoachOffers(ctx, coachIDs)
	if err != nil {
		return 0, err
	}
	for _, row := range rows {
		s.notifyCoachOfferApplied(ctx, row.CoachID)
	}
	return len(rows), nil
}

// effectiveCoachOffers は、コーチたちのいま有効な追加項目と見本動画を返す。読む前に、切り替えの条件を満たした申請を有効にする。
// 切り替えに失敗しても、いまの有効な内容で答える（表示や申込みを止めない。tick が拾い直す）。
func (s *Server) effectiveCoachOffers(ctx context.Context, q *db.Queries, coachIDs []uuid.UUID) (map[uuid.UUID]coachOfferSet, error) {
	out := make(map[uuid.UUID]coachOfferSet, len(coachIDs))
	if len(coachIDs) == 0 {
		return out, nil
	}
	if q == s.q {
		if _, err := s.applyDueCoachOffers(ctx, coachIDs...); err != nil {
			slog.Warn("apply due coach offers", "err", err)
		}
	}
	for _, id := range coachIDs {
		out[id] = coachOfferSet{Items: []string{}, Videos: []string{}}
	}
	rows, err := q.ListCoachOffers(ctx, coachIDs)
	if err != nil {
		return nil, err
	}
	appIDs := make([]uuid.UUID, 0, len(rows))
	byApp := map[uuid.UUID]uuid.UUID{}
	for _, row := range rows {
		appID := row.ApplicationID
		set := out[row.CoachID]
		set.Items = nonNilStrings(row.Items)
		set.ApplicationID = &appID
		out[row.CoachID] = set
		appIDs = append(appIDs, appID)
		byApp[appID] = row.CoachID
	}
	videos, err := coachOfferVideos(ctx, q, appIDs)
	if err != nil {
		return nil, err
	}
	for appID, coachID := range byApp {
		set := out[coachID]
		set.Videos = videos[appID]
		out[coachID] = set
	}
	return out, nil
}

// coachOfferVideos は、申請ごとの見本動画（提出した順）。
func coachOfferVideos(ctx context.Context, q *db.Queries, appIDs []uuid.UUID) (map[uuid.UUID][]string, error) {
	out := make(map[uuid.UUID][]string, len(appIDs))
	for _, id := range appIDs {
		out[id] = []string{}
	}
	if len(appIDs) == 0 {
		return out, nil
	}
	rows, err := q.ListCoachOfferApplicationVideos(ctx, appIDs)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ApplicationID] = append(out[row.ApplicationID], row.VideoPublicID)
	}
	return out, nil
}

// coachOfferItemsFor は、依頼に固定する追加項目（指名したコーチのいま有効なタイトル）。コーチが決まっていなければ空。
func (s *Server) coachOfferItemsFor(ctx context.Context, coachID *uuid.UUID) ([]string, error) {
	if coachID == nil {
		return []string{}, nil
	}
	sets, err := s.effectiveCoachOffers(ctx, s.q, []uuid.UUID{*coachID})
	if err != nil {
		return nil, err
	}
	return sets[*coachID].Items, nil
}

// demoVideosJSON は、見本動画の再生用の形。withID はコーチ本人と運営にだけ public_id を見せる（申請の出し直しに使う）。
func (s *Server) demoVideosJSON(ids []string, withID bool) []map[string]any {
	out := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		item := map[string]any{"url": s.cloud.CoachingPlaybackURL(id), "thumbnail_url": s.cloud.CoachingThumbnailURL(id)}
		if withID {
			item["public_id"] = id
		}
		out = append(out, item)
	}
	return out
}

func (s *Server) coachOfferApplicationJSON(a db.CoachOfferApplication, videos []string) map[string]any {
	return map[string]any{
		"id": a.ID, "items": nonNilStrings(a.Items), "demo_videos": s.demoVideosJSON(videos, true), "status": a.Status,
		"created_at": a.CreatedAt, "updated_at": a.UpdatedAt, "approved_at": a.ApprovedAt, "applied_at": a.AppliedAt,
	}
}

// myCoachOffers は、コーチ本人の添削項目・見本動画（いま有効な内容と、承認待ち・切り替え待ちの申請）。
func (s *Server) myCoachOffers(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireApprovedCoach(w, r)
	if !ok {
		return
	}
	s.writeMyCoachOffers(w, r, p.ID)
}

func (s *Server) writeMyCoachOffers(w http.ResponseWriter, r *http.Request, coachID uuid.UUID) {
	ctx := r.Context()
	sets, err := s.effectiveCoachOffers(ctx, s.q, []uuid.UUID{coachID})
	if err != nil {
		MapDBError(w, err)
		return
	}
	apps, err := s.q.ListCoachOfferApplicationsFor(ctx, coachID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	ids := make([]uuid.UUID, 0, len(apps))
	for _, a := range apps {
		ids = append(ids, a.ID)
	}
	videos, err := coachOfferVideos(ctx, s.q, ids)
	if err != nil {
		MapDBError(w, err)
		return
	}
	var pending, approved any
	for _, a := range apps {
		switch a.Status {
		case "pending":
			pending = s.coachOfferApplicationJSON(a, videos[a.ID])
		case "approved":
			approved = s.coachOfferApplicationJSON(a, videos[a.ID])
		}
	}
	set := sets[coachID]
	JSON(w, http.StatusOK, map[string]any{
		"items": set.Items, "demo_videos": s.demoVideosJSON(set.Videos, true),
		"pending": pending, "approved": approved,
		"max_items": maxCoachOfferItems, "max_item_chars": maxCoachOfferItemChars, "max_demo_videos": maxCoachDemoVideos,
	})
}

type coachOfferReq struct {
	Items              []string `json:"items"`
	DemoVideoPublicIDs []string `json:"demo_video_public_ids"`
}

// applyCoachOffer は、追加項目と見本動画の希望セット（全件）を申請する。承認待ちがあればセットを書き換える（上書き申請）。
// 価格の申請とは別のレコードで、別々に出せる。
func (s *Server) applyCoachOffer(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireApprovedCoach(w, r)
	if !ok {
		return
	}
	var req coachOfferReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	fields := map[string]string{}
	items, e := coachOfferItemsError(req.Items)
	if e != "" {
		fields["items"] = e
	}
	videos := make([]string, 0, len(req.DemoVideoPublicIDs))
	seen := map[string]bool{}
	switch {
	case len(req.DemoVideoPublicIDs) > maxCoachDemoVideos:
		fields["demo_video_public_ids"] = "見本動画は3本までです"
	default:
		for _, id := range req.DemoVideoPublicIDs {
			if !cloudinary.ValidCoachingPublicID(id) {
				fields["demo_video_public_ids"] = "見本動画を選び直してください"
				break
			}
			if seen[id] {
				fields["demo_video_public_ids"] = "同じ見本動画が重なっています"
				break
			}
			seen[id] = true
			videos = append(videos, id)
		}
	}
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	ctx := r.Context()
	// 見本動画は、コーチ本人がアップロードしたものだけ（他人の動画の ID を知っていても登録できない）。
	// 回答の見本動画と同じ動画も使える（回答ごとの見本動画とは別の保持。本体 #1086 補足 S6）。
	for _, id := range videos {
		if !s.uploadIssuedToMe(w, r, p.ID, id, "demo_video_public_ids", "見本動画") {
			return
		}
		if err := s.cloud.CoachingVideoReady(ctx, id); err != nil {
			writeAssetReadyError(w, err, "見本動画")
			return
		}
	}
	err := s.tx(ctx, func(q *db.Queries) error {
		if err := q.LockCoachOffers(ctx, p.ID); err != nil {
			return err
		}
		a, err := q.UpsertPendingCoachOffer(ctx, db.UpsertPendingCoachOfferParams{ID: uuid.New(), CoachID: p.ID, Items: items})
		if err != nil {
			return err
		}
		if err := q.DeleteCoachOfferApplicationVideos(ctx, a.ID); err != nil {
			return err
		}
		for i, id := range videos {
			if err := q.InsertCoachOfferApplicationVideo(ctx, db.InsertCoachOfferApplicationVideoParams{
				ApplicationID: a.ID, Position: int16(i), VideoPublicID: id,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		// 回収が決まった動画（000100 の守り）は MapDBError が 422 で返す。
		MapDBError(w, err)
		return
	}
	s.writeMyCoachOffers(w, r, p.ID)
}

// withdrawCoachOffer は、承認待ちの申請を取り下げる。承認済みのものは取り下げられない。
func (s *Server) withdrawCoachOffer(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireApprovedCoach(w, r)
	if !ok {
		return
	}
	_, err := s.q.WithdrawPendingCoachOffer(r.Context(), p.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusConflict, "conflict", "取り下げられる申請がありません（承認済みの申請は取り下げられません）")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	s.writeMyCoachOffers(w, r, p.ID)
}

// adminCoachOffers は、運営の添削項目・見本動画の申請の一覧（承認待ちを先に）。
// 承認の前に中身を確かめられるよう、追加項目と見本動画を「いま有効な内容（変更前）」と並べて返す（補足 S3）。
func (s *Server) adminCoachOffers(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	ctx := r.Context()
	if _, err := s.applyDueCoachOffers(ctx); err != nil {
		slog.Warn("apply due coach offers", "err", err)
	}
	rows, err := s.q.ListAdminCoachOfferApplications(ctx)
	if err != nil {
		MapDBError(w, err)
		return
	}
	coachIDs := make([]uuid.UUID, 0, len(rows))
	appIDs := make([]uuid.UUID, 0, len(rows))
	seen := map[uuid.UUID]bool{}
	for _, row := range rows {
		appIDs = append(appIDs, row.ID)
		if !seen[row.CoachID] {
			seen[row.CoachID] = true
			coachIDs = append(coachIDs, row.CoachID)
		}
	}
	current, err := s.effectiveCoachOffers(ctx, s.q, coachIDs)
	if err != nil {
		MapDBError(w, err)
		return
	}
	videos, err := coachOfferVideos(ctx, s.q, appIDs)
	if err != nil {
		MapDBError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		set := current[row.CoachID]
		out = append(out, map[string]any{
			"id": row.ID, "coach_id": row.CoachID, "coach_name": row.CoachName, "coach_status": row.CoachStatus,
			"items": nonNilStrings(row.Items), "demo_videos": s.demoVideosJSON(videos[row.ID], false),
			"current_items": set.Items, "current_demo_videos": s.demoVideosJSON(set.Videos, false),
			"status": row.Status, "created_at": row.CreatedAt, "updated_at": row.UpdatedAt,
			"approved_at": row.ApprovedAt, "approved_by_nickname": row.ApprovedByNickname, "applied_at": row.AppliedAt,
		})
	}
	JSON(w, http.StatusOK, map[string]any{"applications": out, "fixed_items": fixedOfferTitles})
}

// adminApproveCoachOffer は、申請を承認する（承認した人・日時を残す）。即時には切り替えず、
// 承認の時点で対応中の依頼がなければ、このあとすぐに有効になる。
func (s *Server) adminApproveCoachOffer(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	var approved db.CoachOfferApplication
	err := s.tx(ctx, func(q *db.Queries) error {
		a, err := q.GetCoachOfferApplication(ctx, id)
		if err != nil {
			return err
		}
		if err := q.LockCoachOffers(ctx, a.CoachID); err != nil {
			return err
		}
		// 切り替え前の古い承認は、新しい承認で置き換える。
		if a.Status == "pending" {
			if err := q.SupersedeApprovedCoachOffer(ctx, a.CoachID); err != nil {
				return err
			}
		}
		approved, err = q.ApproveCoachOffer(ctx, db.ApproveCoachOfferParams{ID: id, ApprovedBy: &admin.ID})
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		if _, getErr := s.q.GetCoachOfferApplication(ctx, id); errors.Is(getErr, pgx.ErrNoRows) {
			Error(w, http.StatusNotFound, "not_found", "申請が見つかりません")
			return
		}
		Error(w, http.StatusConflict, "conflict", "この申請は承認待ちではありません（取り下げ済みか、承認済みです）")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	s.notifyCoachOfferApproved(ctx, approved.CoachID)
	if _, err := s.applyDueCoachOffers(ctx, approved.CoachID); err != nil {
		slog.Warn("apply due coach offers", "err", err)
	}
	if latest, err := s.q.GetCoachOfferApplication(ctx, approved.ID); err == nil {
		approved = latest
	}
	videos, err := coachOfferVideos(ctx, s.q, []uuid.UUID{approved.ID})
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"application": s.coachOfferApplicationJSON(approved, videos[approved.ID])})
}

func (s *Server) notifyCoachOfferApproved(ctx context.Context, coachID uuid.UUID) {
	s.notify(ctx, coachID, notifyCoaching, push.Message{
		Title: "添削項目・見本動画の申請が承認されました",
		Body:  "対応中の依頼がすべて終わると、新しい添削項目・見本動画に切り替わります",
		URL:   "/coach/price",
		Tag:   "coach-offer",
	}, 24*time.Hour)
}

func (s *Server) notifyCoachOfferApplied(ctx context.Context, coachID uuid.UUID) {
	s.notify(ctx, coachID, notifyCoaching, push.Message{
		Title: "新しい添削項目・見本動画に切り替わりました",
		Body:  "これからの申込みは新しい添削項目で受け付けます",
		URL:   "/coach/price",
		Tag:   "coach-offer",
	}, 24*time.Hour)
}

// responseItemsJSON は、回答の追加項目（タイトル＋本文、依頼に固定した順）。
func responseItemsJSON(items []db.ReviewResponseItem) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		out = append(out, map[string]any{"title": it.Title, "body": it.Body})
	}
	return out
}

// reviewResponseItemsByRequest は、依頼ごとの回答の追加項目。
func reviewResponseItemsByRequest(ctx context.Context, q *db.Queries, ids []uuid.UUID) (map[uuid.UUID][]db.ReviewResponseItem, error) {
	out := make(map[uuid.UUID][]db.ReviewResponseItem, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := q.ListReviewResponseItems(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.RequestID] = append(out[row.RequestID], row)
	}
	return out, nil
}

// offerItemField は、回答の追加項目の欄のエラーのキー（offer_items.0 など）。
func offerItemField(i int) string { return "offer_items." + strconv.Itoa(i) }
