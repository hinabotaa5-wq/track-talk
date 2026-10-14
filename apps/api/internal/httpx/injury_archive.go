package httpx

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
)

type archiveStep struct {
	Stage string `json:"stage"`
	Days  int    `json:"days"`
}

func monthStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// archiveInjuryData copies a leaving member's group records into injury_archives, one row per body part,
// as numbers and choices only: the body part, the month of the injury and the stages with the days between them.
// Nothing written in words (the place, the situation, posts, reports) is kept, and nothing links back to the account.
// It runs inside the withdrawal transaction, before the purge.
func archiveInjuryData(ctx context.Context, q *db.Queries, userID uuid.UUID, now time.Time) error {
	members, err := q.ListMyInjuryGroupMembers(ctx, userID)
	if err != nil {
		return err
	}
	for _, m := range members {
		log, err := q.ListInjuryGroupStageLog(ctx, db.ListInjuryGroupStageLogParams{UserID: userID, BodyPart: m.BodyPart})
		if err != nil {
			return err
		}
		timeline := make([]archiveStep, 0, len(log))
		for _, entry := range log {
			timeline = append(timeline, archiveStep{Stage: entry.Stage, Days: int(entry.CreatedAt.Sub(log[0].CreatedAt).Hours() / 24)})
		}
		encoded, err := json.Marshal(timeline)
		if err != nil {
			return err
		}
		if err := q.InsertInjuryArchive(ctx, db.InsertInjuryArchiveParams{
			BodyPart:      m.BodyPart,
			InjuredMonth:  monthStart(m.InjuredOn),
			ArchivedMonth: monthStart(now),
			StageTimeline: encoded,
		}); err != nil {
			return err
		}
	}
	return nil
}
