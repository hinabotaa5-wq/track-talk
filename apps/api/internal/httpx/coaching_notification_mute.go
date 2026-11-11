package httpx

import (
	"context"

	"github.com/google/uuid"
)

// コーチング通知の一時ミュート（ops #73 の暫定リビジョン）。
// 土台は本番到達点の API コードで、main へはマージしない。migration は作らない。
// 決済・申込・引受・返却そのものの処理には触れず、通知の生成・送信だけを抑止する。

// coachingMutedCategory は、ミュート中も抑止の対象になる通知の種類か。
// フィード・募集・月次・手動アナウンスなどコーチング以外の種類は対象外。
func coachingMutedCategory(category string) bool {
	switch category {
	case notifyCoaching, notifyReview, notifyImportant:
		return true
	}
	return false
}

// coachingMutedFor は、その宛先へのコーチング通知を抑止するか。許可リスト内は通常どおり。
func (s *Server) coachingMutedFor(to uuid.UUID) bool {
	return s.cfg.CoachingNotificationSuppressed(to.String())
}

// insertCoachingReviewNotification は、添削のベルの記録を書く。ミュート中の対象外宛先には書かない。
func (s *Server) insertCoachingReviewNotification(ctx context.Context, to uuid.UUID, kind string, requestID uuid.UUID) error {
	if s.coachingMutedFor(to) {
		return nil
	}
	return insertReviewNotification(ctx, s.q, to, kind, requestID)
}

// insertCoachingReviewNotificationWithURL は、URL 付きの添削のベルの記録を書く。抑止の扱いは同じ。
func (s *Server) insertCoachingReviewNotificationWithURL(ctx context.Context, to uuid.UUID, kind string, requestID uuid.UUID, url *string) error {
	if s.coachingMutedFor(to) {
		return nil
	}
	return insertReviewNotificationWithURL(ctx, s.q, to, kind, requestID, url)
}

// coachingAllowlistedIDs は、ミュート中も通知する相手の ID 列。不正な値は捨てる。
func (s *Server) coachingAllowlistedIDs() []uuid.UUID {
	var out []uuid.UUID
	for raw := range s.cfg.CoachingNotificationAllowlist {
		if id, err := uuid.Parse(raw); err == nil {
			out = append(out, id)
		}
	}
	return out
}
