package httpx

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/hinata/track-talk/apps/api/internal/db"
)

// パーソナル指導のやりとりの保存期間。期間が終わってから1年は読め、その後は文章と動画を消す（2026-10-17 決定）。
// 未解決の通報がある契約は、運営が対応を終えるまで消さない。契約・支払い・報酬の記録は残す。

// purgeCoachingMessages は、保存期間を過ぎた契約のやりとりを消す（tick から）。
// 動画を Cloudinary から消せなかった契約は、文章も残して次の tick でやり直す。
func (s *Server) purgeCoachingMessages(ctx context.Context) (int, error) {
	ids, err := s.q.ListContractsToPurge(ctx)
	if err != nil {
		return 0, err
	}
	purged := 0
	for _, id := range ids {
		done := false
		err = s.tx(ctx, func(q *db.Queries) error {
			if err := q.LockConsultationRoot(ctx, id); err != nil {
				return err
			}
			if _, err := q.LockConsultationContract(ctx, id); err != nil {
				return err
			}
			eligible, err := q.ListContractsToPurge(ctx)
			if err != nil {
				return err
			}
			found := false
			for _, candidate := range eligible {
				if candidate == id {
					found = true
					break
				}
			}
			if !found {
				return nil
			}
			videos, err := q.ListContractMessageVideos(ctx, id)
			if err != nil {
				return err
			}
			if err := s.cloud.DeleteCoachingVideos(ctx, videos); err != nil {
				return err
			}
			if _, err := q.DeleteContractMessages(ctx, id); err != nil {
				return err
			}
			if err := q.PurgeSupportLinks(ctx, id); err != nil {
				return err
			}
			if err := q.MarkContractMessagesPurged(ctx, id); err != nil {
				return err
			}
			done = true
			return nil
		})
		if err != nil {
			slog.Warn("delete coaching message videos", "contract", id, "err", err)
			continue
		}
		if done {
			purged++
		}
	}
	return purged, nil
}

// closeReport は、運営が通報の対応を終える。やりとり欄の通報は、終えるまで保存期間を過ぎても会話を消さない。
func (s *Server) closeReport(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	rp, err := s.q.CloseReport(r.Context(), id)
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"report": rp})
}
