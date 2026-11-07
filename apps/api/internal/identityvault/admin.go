package identityvault

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type accessLog struct {
	ID        int64     `json:"id"`
	Viewer    uuid.UUID `json:"viewer"`
	Purpose   string    `json:"purpose"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Service) viewIdentity(w http.ResponseWriter, r *http.Request, viewer uuid.UUID) {
	coach, ok := idParam(w, r)
	if !ok {
		return
	}
	var input struct {
		Purpose string `json:"purpose"`
	}
	if !decode(w, r, &input) {
		return
	}
	purpose := strings.TrimSpace(input.Purpose)
	if !validText(purpose, 200) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}
	var out struct {
		Identity *identityFields `json:"identity"`
		Log      []accessLog     `json:"access_logs"`
	}
	ctx := r.Context()
	err := s.locked(ctx, coach, func(tx pgx.Tx) error {
		var err error
		out.Identity, err = s.loadIdentity(ctx, tx, coach)
		if err != nil {
			return err
		}
		if out.Identity == nil {
			return pgx.ErrNoRows
		}
		if _, err := tx.Exec(ctx, "INSERT INTO identity_vault.access_log (coach_id,viewer,purpose) VALUES ($1,$2,$3)", coach, viewer, purpose); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, "SELECT id,viewer,purpose,created_at FROM identity_vault.access_log WHERE coach_id=$1 ORDER BY id DESC LIMIT 50", coach)
		if err != nil {
			return err
		}
		out.Log, err = pgx.CollectRows(rows, pgx.RowToStructByPos[accessLog])
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if err != nil {
		unavailable(w)
		return
	}
	respond(w, out)
}

func (s *Service) forceRequired(w http.ResponseWriter, r *http.Request, _ uuid.UUID) {
	coach, ok := idParam(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	mutationResult(w, s.locked(ctx, coach, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO identity_vault.commerce (coach_id,required,forced) VALUES ($1,true,true)
			ON CONFLICT (coach_id) DO UPDATE SET required=true,forced=true,exempt_declared_at=NULL`, coach)
		if err != nil {
			return err
		}
		return clearDisclosures(ctx, tx, coach)
	}))
}

func (s *Service) revokeAll(w http.ResponseWriter, r *http.Request, _ uuid.UUID) {
	coach, ok := idParam(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	mutationResult(w, s.locked(ctx, coach, func(tx pgx.Tx) error {
		return clearDisclosures(ctx, tx, coach)
	}))
}
