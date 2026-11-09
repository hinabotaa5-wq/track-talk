package httpx

import (
	"context"
	"log/slog"
	"net/http"
	"slices"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
)

// 動画ファイルの保持（運営者決定 2026-11-10、本体 #506）。
// 動画添削はスレッドの終わりから7日、サポートプランは契約の終わりから30日で、動画ファイル（Cloudinary の元ファイルと派生）を消す。
// 添削のあとの追加相談は相談の終わりから7日、契約の追加相談は契約と相談の遅い方の終わりから30日。
// 通報・紛争の対応中（未対応の通報、異議の申立ての再審査待ち、運営の保全ホールド）は消さない。条件は queries/video_retention.sql。
// 文章・メニュー・取引の記録は消さない（それぞれの保持期間は従来どおり）。行は残し、動画を消した日時を書く。
// 動画の ID は残す（再生 URL に出る ID の流用を防ぐ、CoachingVideoInUse の確認のため）。

type videoRetentionResult struct {
	Reviews       int `json:"review_videos_purged"`
	Contracts     int `json:"contract_videos_purged"`
	Consultations int `json:"consultation_videos_purged"`
}

// purgeExpiredVideos は、保持期間を過ぎた動画ファイルを消す（日次の storage cleanup から）。
// Cloudinary から消せなかったものは印を付けず、翌日にやり直す。
func (s *Server) purgeExpiredVideos(ctx context.Context) (videoRetentionResult, error) {
	var out videoRetentionResult
	reviews, err := s.q.ListReviewVideosToPurge(ctx)
	if err != nil {
		return out, err
	}
	for _, id := range reviews {
		done, err := s.purgeVideosOf(ctx, id, id, func(q *db.Queries) ([]uuid.UUID, error) { return q.ListReviewVideosToPurge(ctx) },
			func(q *db.Queries) ([]string, error) { return q.ListReviewVideoIDs(ctx, id) },
			func(q *db.Queries) error { return q.MarkReviewVideosPurged(ctx, id) })
		if err != nil {
			return out, err
		}
		if done {
			out.Reviews++
		}
	}
	contracts, err := s.q.ListContractVideosToPurge(ctx)
	if err != nil {
		return out, err
	}
	for _, id := range contracts {
		done, err := s.purgeVideosOf(ctx, id, id, func(q *db.Queries) ([]uuid.UUID, error) { return q.ListContractVideosToPurge(ctx) },
			func(q *db.Queries) ([]string, error) { return q.ListContractMessageVideos(ctx, id) },
			func(q *db.Queries) error { return q.MarkContractVideosPurged(ctx, id) })
		if err != nil {
			return out, err
		}
		if done {
			out.Contracts++
		}
	}
	consultations, err := s.q.ListConsultationVideosToPurge(ctx)
	if err != nil {
		return out, err
	}
	for _, id := range consultations {
		c, err := s.q.GetConsultation(ctx, id)
		if err != nil {
			return out, err
		}
		root, _ := consultationParent(c)
		done, err := s.purgeVideosOf(ctx, id, root, func(q *db.Queries) ([]uuid.UUID, error) { return q.ListConsultationVideosToPurge(ctx) },
			func(q *db.Queries) ([]string, error) {
				c, err := q.GetConsultation(ctx, id)
				if err != nil {
					return nil, err
				}
				var ids []string
				for _, v := range []*string{c.VideoPublicID, c.AnswerVideoPublicID} {
					if v != nil {
						ids = append(ids, *v)
					}
				}
				return ids, nil
			},
			func(q *db.Queries) error { return q.MarkConsultationVideosPurged(ctx, id) })
		if err != nil {
			return out, err
		}
		if done {
			out.Consultations++
		}
	}
	return out, nil
}

// purgeVideosOf は、1件分の動画を消して印を付ける。追加相談の操作と同じ鍵（添削・契約の ID）で順番にし、
// 鍵を取ったあとに対象のままか確かめ直す（その間に通報や保全ホールドが入ったら消さない）。
// Cloudinary から消せなかったときは印を付けずに次へ進み、翌日にやり直す。
func (s *Server) purgeVideosOf(ctx context.Context, id, root uuid.UUID, eligible func(*db.Queries) ([]uuid.UUID, error), videos func(*db.Queries) ([]string, error), mark func(*db.Queries) error) (bool, error) {
	done := false
	err := s.tx(ctx, func(q *db.Queries) error {
		if err := q.LockConsultationRoot(ctx, root); err != nil {
			return err
		}
		ids, err := eligible(q)
		if err != nil {
			return err
		}
		if !slices.Contains(ids, id) {
			return nil
		}
		list, err := videos(q)
		if err != nil {
			return err
		}
		if err := s.cloud.DeleteCoachingVideos(ctx, list); err != nil {
			slog.Warn("delete expired coaching videos", "id", id, "err", err)
			return nil
		}
		if err := mark(q); err != nil {
			return err
		}
		done = true
		return nil
	})
	return done, err
}

type retentionHoldReq struct {
	Target string    `json:"target"`
	ID     uuid.UUID `json:"id"`
	Hold   bool      `json:"hold"`
}

// adminSetRetentionHold は、運営の保全ホールドを立てる・外す。通報・紛争（チャージバックなど）の対応中に立て、
// 対応を終えたら外す。立っている間は動画を消さず、サポートプランのやりとり欄は終わった後も閉じない。
// target は review（動画添削の依頼の ID）か contract（契約の ID）。
func (s *Server) adminSetRetentionHold(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var req retentionHoldReq
	if err := DecodeJSON(r, &req); err != nil || req.ID == uuid.Nil {
		Error(w, http.StatusBadRequest, "validation_error", "対象の ID を指定してください")
		return
	}
	switch req.Target {
	case "review":
		row, err := s.q.SetReviewRetentionHold(r.Context(), db.SetReviewRetentionHoldParams{ID: req.ID, Hold: req.Hold})
		if err != nil {
			MapDBError(w, err)
			return
		}
		JSON(w, http.StatusOK, map[string]any{"target": req.Target, "id": row.ID, "retention_hold_at": row.RetentionHoldAt, "videos_purged_at": row.VideosPurgedAt})
	case "contract":
		row, err := s.q.SetContractRetentionHold(r.Context(), db.SetContractRetentionHoldParams{ID: req.ID, Hold: req.Hold})
		if err != nil {
			MapDBError(w, err)
			return
		}
		JSON(w, http.StatusOK, map[string]any{"target": req.Target, "id": row.ID, "retention_hold_at": row.RetentionHoldAt, "videos_purged_at": row.VideosPurgedAt})
	default:
		Error(w, http.StatusBadRequest, "validation_error", "対象は review か contract です")
	}
}
