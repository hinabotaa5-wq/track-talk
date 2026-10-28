package httpx

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/cloudinary"
	"github.com/hinata/track-talk/apps/api/internal/competitions"
	"github.com/jackc/pgx/v5"
)

type storageCleanupResult struct {
	Notifications  int64 `json:"notifications_deleted"`
	Competitions   int64 `json:"competitions_deleted"`
	DomesticMajors int64 `json:"domestic_majors_deleted"`
	MajorHosts     int64 `json:"major_hosts_deleted"`
	CoachIdentity  int64 `json:"coach_identities_deleted"`
	Consultations  int   `json:"consultations_purged"`
	Messages       int   `json:"message_rooms_purged"`
	Assets         int   `json:"unused_assets_deleted"`
}

// 日次の専用処理。コーチングの新規受付が閉じていても保存期限の処理は続ける。
func (s *Server) storageCleanup(w http.ResponseWriter, r *http.Request) {
	secret := s.cfg.StorageCleanupSecret
	if secret == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Storage-Cleanup-Secret")), []byte(secret)) != 1 {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return
	}
	out, err := s.runStorageCleanup(r.Context())
	if err != nil {
		slog.Error("storage cleanup failed", "result", out, "err", err)
		MapDBError(w, err)
		return
	}
	slog.Info("storage cleanup completed", "result", out)
	JSON(w, http.StatusOK, out)
}

func (s *Server) runStorageCleanup(ctx context.Context) (storageCleanupResult, error) {
	var out storageCleanupResult
	var err error
	if out.Notifications, err = s.q.DeleteOldNotifications(ctx); err != nil {
		return out, err
	}
	now := time.Now()
	if s.cfg.Clock != nil {
		now = s.cfg.Clock()
	}
	season := competitions.SeasonYear(now)
	if out.Competitions, err = s.q.DeletePastCompetitionSeasons(ctx, int32(season)); err != nil {
		return out, err
	}
	start := time.Date(season, time.April, 1, 0, 0, 0, 0, time.UTC)
	for _, target := range []struct {
		table string
		count *int64
	}{{"domestic_majors", &out.DomesticMajors}, {"major_hosts", &out.MajorHosts}} {
		tag, err := s.pool.Exec(ctx, "DELETE FROM "+target.table+" WHERE ends_on < $1", start)
		if err != nil {
			return out, err
		}
		*target.count = tag.RowsAffected()
	}
	// コーチの特商法の登録情報は、最後の取引とコーチをやめた時の遅いほうから5年で消す。
	if out.CoachIdentity, err = s.q.DeleteExpiredCoachLegalIdentities(ctx, now.AddDate(-coachIdentityRetentionYears, 0, 0)); err != nil {
		return out, err
	}
	// 外部ストレージを消せない設定では文章も消さない。
	if !s.cloud.Configured() {
		return out, cloudinary.ErrNotConfigured
	}
	if out.Consultations, err = s.purgeConsultations(ctx); err != nil {
		return out, err
	}
	if out.Messages, err = s.purgeCoachingMessages(ctx); err != nil {
		return out, err
	}
	out.Assets, err = s.purgeUnusedStoredAssets(ctx, now.Add(-7*24*time.Hour))
	return out, err
}

// 生きている投稿は非表示・保存年数にかかわらず保護する。ケガ・レビュー・指導も残す。
const storedMediaInUse = `SELECT
 EXISTS(SELECT 1 FROM posts WHERE cloudinary_public_id=$1 AND (deleted_at IS NULL OR kind='injury'))
 OR EXISTS(SELECT 1 FROM recruitment_photos rp JOIN posts p ON p.id=rp.post_id WHERE rp.cloudinary_public_id=$1 AND p.deleted_at IS NULL)
 OR EXISTS(SELECT 1 FROM coach_profiles WHERE photo_public_id=$1)
 OR EXISTS(SELECT 1 FROM review_requests WHERE video_public_id=$1)
 OR EXISTS(SELECT 1 FROM review_responses WHERE demo_video_public_id=$1)
 OR EXISTS(SELECT 1 FROM coaching_messages WHERE video_public_id=$1)
 OR EXISTS(SELECT 1 FROM coaching_consultations WHERE video_public_id=$1 OR answer_video_public_id=$1)
 OR EXISTS(SELECT 1 FROM products WHERE strpos(replace(image_url,'%2F','/'),$1)>0)
 OR EXISTS(SELECT 1 FROM major_hosts WHERE strpos(replace(image_url,'%2F','/'),$1)>0)`

func (s *Server) retireUnusedAsset(ctx context.Context, id string) (bool, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	// 投稿の書き込みトリガーと同じロック。ロック後の新しいスナップショットで使用を確認する。
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('media-retire:' || $1,0))`, id); err != nil {
		return false, err
	}
	var used bool
	if err = tx.QueryRow(ctx, storedMediaInUse, id).Scan(&used); err != nil {
		return false, err
	}
	if used {
		return false, nil
	}
	if _, err = tx.Exec(ctx, `INSERT INTO cloudinary_retirements(public_id) VALUES($1) ON CONFLICT DO NOTHING`, id); err != nil {
		return false, err
	}
	// 削除に失敗しても再利用は禁止したままにし、翌日の一覧から削除を再試行する。
	return true, tx.Commit(ctx)
}

func (s *Server) purgeUnusedStoredAssets(ctx context.Context, cutoff time.Time) (int, error) {
	deleted := 0
	for _, kind := range []struct{ resource, delivery string }{{"image", "upload"}, {"video", "upload"}, {"video", cloudinary.CoachingVideoType}} {
		cursor := ""
		seen := map[string]bool{}
		for {
			assets, next, err := s.cloud.ListStoredAssets(ctx, kind.resource, kind.delivery, cursor)
			if err != nil {
				return deleted, err
			}
			for _, asset := range assets {
				valid := cloudinary.ValidPublicID(asset.PublicID)
				if kind.delivery == cloudinary.CoachingVideoType {
					valid = cloudinary.ValidCoachingPublicID(asset.PublicID)
				}
				if !valid || asset.CreatedAt.IsZero() || !asset.CreatedAt.Before(cutoff) {
					continue
				}
				retired, err := s.retireUnusedAsset(ctx, asset.PublicID)
				if err != nil {
					return deleted, err
				}
				if !retired {
					continue
				}
				if err = s.cloud.DeleteStoredAsset(ctx, kind.resource, kind.delivery, asset.PublicID); err != nil {
					return deleted, err
				}
				deleted++
				// 1回の応答時間を抑える。残りは次の日に続ける。
				if deleted >= 10 {
					return deleted, nil
				}
			}
			if next == "" {
				break
			}
			if seen[next] {
				return deleted, fmt.Errorf("cloudinary returned a repeated page cursor")
			}
			seen[next] = true
			cursor = next
		}
	}
	return deleted, nil
}
