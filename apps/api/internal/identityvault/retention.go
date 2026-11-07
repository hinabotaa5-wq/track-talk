package identityvault

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Service) cleanup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := s.vault.Query(ctx, "SELECT coach_id FROM identity_vault.identities ORDER BY coach_id")
	if err != nil {
		unavailable(w)
		return
	}
	coaches, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		unavailable(w)
		return
	}
	cutoff := time.Now().In(jst).AddDate(-5, 0, 0)
	for _, coach := range coaches {
		if err := s.cleanupCoach(ctx, coach, cutoff); err != nil {
			unavailable(w)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

var jst = time.FixedZone("JST", 9*60*60)

func (s *Service) cleanupCoach(ctx context.Context, coach uuid.UUID, cutoff time.Time) error {
	return s.locked(ctx, coach, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, "SELECT plan_id,plan_epoch,coach_epoch FROM identity_vault.disclosures WHERE coach_id=$1", coach)
		if err != nil {
			return err
		}
		type grant struct{ Plan, PlanEpoch, CoachEpoch uuid.UUID }
		grants, err := pgx.CollectRows(rows, pgx.RowToStructByPos[grant])
		if err != nil {
			return err
		}
		for _, g := range grants {
			p, err := s.plan(ctx, g.Plan)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			if err == nil && p == (planEpoch{coach, g.PlanEpoch, g.CoachEpoch}) {
				continue
			}
			if _, err := tx.Exec(ctx, "DELETE FROM identity_vault.disclosures WHERE plan_id=$1 AND coach_id=$2", g.Plan, coach); err != nil {
				return err
			}
		}

		var ended time.Time
		var latest *time.Time
		err = s.main.QueryRow(ctx, `SELECT ended_at,GREATEST(last_transaction_at,identity_boundary.last_transaction(coach_id))
			FROM identity_boundary.retention WHERE coach_id=$1`, coach).Scan(&ended, &latest)
		if errors.Is(err, pgx.ErrNoRows) {
			_, err = tx.Exec(ctx, "UPDATE identity_vault.identities SET ended_at=NULL WHERE coach_id=$1 AND ended_at IS NOT NULL", coach)
			return err
		}
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE identity_vault.identities SET ended_at=$2,
			last_transaction_at=GREATEST(last_transaction_at,$3::timestamptz) WHERE coach_id=$1`, coach, ended, latest); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, "DELETE FROM identity_vault.identities WHERE coach_id=$1 AND GREATEST(ended_at,last_transaction_at)<$2", coach, cutoff)
		return err
	})
}
