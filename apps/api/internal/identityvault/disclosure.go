package identityvault

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const checklistVersion = 1

var checklist = []string{
	"有料の指導を、TrackTalk の外で事業として続けていない",
	"TrackTalk のプランも、繰り返し収入を得ることを目的にしていない",
	"ホームページや SNS で、有料の指導を宣伝・募集していない",
}

type commerce struct {
	Required bool       `json:"required"`
	Forced   bool       `json:"forced"`
	Declared *time.Time `json:"exempt_declared_at"`
}

func settings(ctx context.Context, tx pgx.Tx, coach uuid.UUID) (commerce, error) {
	out := commerce{Required: true}
	err := tx.QueryRow(ctx, "SELECT required, forced, exempt_declared_at FROM identity_vault.commerce WHERE coach_id=$1", coach).Scan(&out.Required, &out.Forced, &out.Declared)
	if errors.Is(err, pgx.ErrNoRows) {
		err = nil
	}
	return out, err
}

func (s *Service) coachView(w http.ResponseWriter, r *http.Request, coach uuid.UUID) {
	var out struct {
		commerce
		Version  int             `json:"checklist_version"`
		List     []string        `json:"checklist"`
		Identity *identityFields `json:"identity"`
		Plans    []eligiblePlan  `json:"plans"`
		Revision *uuid.UUID      `json:"identity_revision"`
	}
	ctx := r.Context()
	err := s.locked(ctx, coach, func(tx pgx.Tx) error {
		var err error
		out.commerce, err = settings(ctx, tx, coach)
		if err != nil {
			return err
		}
		out.Identity, err = s.loadIdentity(ctx, tx, coach)
		if err != nil {
			return err
		}
		if out.Identity != nil {
			if err := tx.QueryRow(ctx, "SELECT revision FROM identity_vault.identities WHERE coach_id=$1", coach).Scan(&out.Revision); err != nil {
				return err
			}
		}
		out.Plans, err = s.eligiblePlans(ctx, tx, coach)
		return err
	})
	if err != nil {
		unavailable(w)
		return
	}
	out.Version = checklistVersion
	out.List = checklist
	respond(w, out)
}

type eligiblePlan struct {
	ID         uuid.UUID `json:"id"`
	Title      string    `json:"title"`
	Authorized bool      `json:"authorized"`
}

func (s *Service) eligiblePlans(ctx context.Context, tx pgx.Tx, coach uuid.UUID) ([]eligiblePlan, error) {
	rows, err := s.main.Query(ctx, "SELECT id,title,plan_epoch,coach_epoch FROM identity_boundary.published_plans WHERE coach_id=$1 ORDER BY id", coach)
	if err != nil {
		return nil, err
	}
	type row struct {
		ID                    uuid.UUID
		Title                 string
		PlanEpoch, CoachEpoch uuid.UUID
	}
	plans, err := pgx.CollectRows(rows, pgx.RowToStructByPos[row])
	if err != nil {
		return nil, err
	}
	out := make([]eligiblePlan, 0, len(plans))
	for _, p := range plans {
		var approved bool
		err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM identity_vault.disclosures WHERE plan_id=$1
			AND coach_id=$2 AND plan_epoch=$3 AND coach_epoch=$4)`, p.ID, coach, p.PlanEpoch, p.CoachEpoch).Scan(&approved)
		if err != nil {
			return nil, err
		}
		out = append(out, eligiblePlan{ID: p.ID, Title: p.Title, Authorized: approved})
	}
	return out, nil
}

var errConflict = errors.New("identity state conflict")

func mutationResult(w http.ResponseWriter, err error) {
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, errConflict):
		w.WriteHeader(http.StatusConflict)
	default:
		unavailable(w)
	}
}

func (s *Service) exemption(w http.ResponseWriter, r *http.Request, coach uuid.UUID) {
	var input struct {
		Exempt  *bool  `json:"exempt"`
		Version int    `json:"checklist_version"`
		Checked []bool `json:"checked"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.Exempt == nil || (*input.Exempt && (input.Version != checklistVersion || len(input.Checked) != len(checklist))) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}
	if *input.Exempt {
		for _, checked := range input.Checked {
			if !checked {
				w.WriteHeader(http.StatusUnprocessableEntity)
				return
			}
		}
	}
	ctx := r.Context()
	mutationResult(w, s.locked(ctx, coach, func(tx pgx.Tx) error {
		state, err := settings(ctx, tx, coach)
		if err != nil {
			return err
		}
		if state.Forced && *input.Exempt {
			return errConflict
		}
		if state.Required == !*input.Exempt {
			return nil
		}
		_, err = tx.Exec(ctx, `INSERT INTO identity_vault.commerce (coach_id,required,exempt_declared_at)
			VALUES ($1,$2,CASE WHEN $2 THEN NULL ELSE now() END) ON CONFLICT(coach_id) DO UPDATE
			SET required=EXCLUDED.required, exempt_declared_at=EXCLUDED.exempt_declared_at`, coach, !*input.Exempt)
		if err != nil {
			return err
		}
		return clearDisclosures(ctx, tx, coach)
	}))
}

type planEpoch struct {
	coach, planVersion, coachVersion uuid.UUID
}

func (s *Service) plan(ctx context.Context, id uuid.UUID) (planEpoch, error) {
	var p planEpoch
	err := s.main.QueryRow(ctx, "SELECT coach_id,plan_epoch,coach_epoch FROM identity_boundary.published_plans WHERE id=$1", id).Scan(&p.coach, &p.planVersion, &p.coachVersion)
	return p, err
}

func (s *Service) publish(w http.ResponseWriter, r *http.Request, coach uuid.UUID) {
	id, ok := idParam(w, r)
	if !ok {
		return
	}
	// 公開する値は、確認した版（identity_revision）の登録内容そのもの。
	// 電話番号もその版に含まれ、ここでは受け取らない。
	var input struct {
		Confirm  bool      `json:"confirm"`
		Revision uuid.UUID `json:"identity_revision"`
	}
	if !decode(w, r, &input) {
		return
	}
	if !input.Confirm {
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}
	ctx := r.Context()
	mutationResult(w, s.locked(ctx, coach, func(tx pgx.Tx) error {
		p, err := s.plan(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && p.coach != coach) {
			return errConflict
		}
		if err != nil {
			return err
		}
		st, err := settings(ctx, tx, coach)
		if err != nil {
			return err
		}
		var currentRevision uuid.UUID
		var hasPhone bool
		err = tx.QueryRow(ctx, "SELECT revision, phone_enc IS NOT NULL FROM identity_vault.identities WHERE coach_id=$1", coach).Scan(&currentRevision, &hasPhone)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && currentRevision != input.Revision) {
			return errConflict
		}
		if err != nil {
			return err
		}
		// 移行前の登録（電話番号なし）は、登録し直すまで表記要の公開を認めない。
		if st.Required && !hasPhone {
			return errConflict
		}
		// The second check is at read time. A main DB change concurrent with
		// this approval can at most leave an approval for an obsolete epoch.
		// disclosures.phone は旧方式の平文の列。書き込まず、上書きのたびに消す。
		_, err = tx.Exec(ctx, `INSERT INTO identity_vault.disclosures (plan_id,coach_id,plan_epoch,coach_epoch)
			VALUES ($1,$2,$3,$4) ON CONFLICT (plan_id) DO UPDATE
			SET coach_id=EXCLUDED.coach_id,plan_epoch=EXCLUDED.plan_epoch,coach_epoch=EXCLUDED.coach_epoch,phone=NULL`,
			id, coach, p.planVersion, p.coachVersion)
		return err
	}))
}

func (s *Service) unpublish(w http.ResponseWriter, r *http.Request, coach uuid.UUID) {
	id, ok := idParam(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	mutationResult(w, s.locked(ctx, coach, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, "DELETE FROM identity_vault.disclosures WHERE plan_id=$1 AND coach_id=$2", id, coach)
		return err
	}))
}

func (s *Service) registration(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r)
	if !ok {
		return
	}
	var approved bool
	if err := s.main.QueryRow(r.Context(), "SELECT EXISTS (SELECT 1 FROM identity_boundary.approved_coaches WHERE profile_id=$1)", id).Scan(&approved); err != nil {
		unavailable(w)
		return
	}
	if !approved {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	var registered bool
	if err := s.vault.QueryRow(r.Context(), "SELECT EXISTS (SELECT 1 FROM identity_vault.identities WHERE coach_id=$1)", id).Scan(&registered); err != nil {
		unavailable(w)
		return
	}
	if !registered {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type PublicDisclosure struct {
	Required   bool   `json:"required"`
	Registered bool   `json:"registered,omitempty"`
	LegalName  string `json:"legal_name,omitempty"`
	Address    string `json:"address,omitempty"`
	Phone      string `json:"phone,omitempty"`
}

func (s *Service) disclosure(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	p, err := s.plan(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if err != nil {
		unavailable(w)
		return
	}
	var result PublicDisclosure
	err = s.locked(ctx, p.coach, func(tx pgx.Tx) error {
		current, err := s.plan(ctx, id)
		if err != nil {
			return err
		}
		if current != p {
			return pgx.ErrNoRows
		}
		// 本人が確認した版の公開許可がなければ、表記要・不要を問わず何も返さない。
		var approved bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM identity_vault.disclosures
			WHERE plan_id=$1 AND coach_id=$2 AND plan_epoch=$3 AND coach_epoch=$4)`,
			id, p.coach, p.planVersion, p.coachVersion).Scan(&approved)
		if err != nil {
			return err
		}
		if !approved {
			return pgx.ErrNoRows
		}
		st, err := settings(ctx, tx, p.coach)
		if err != nil {
			return err
		}
		result.Required = st.Required
		// 表記不要のコーチは、氏名・住所・電話番号のいずれも復号しない。
		if !st.Required {
			return nil
		}
		identity, err := s.loadIdentity(ctx, tx, p.coach)
		if err != nil {
			return err
		}
		if identity == nil || identity.Phone == "" {
			return pgx.ErrNoRows
		}
		result.Registered = true
		result.LegalName = identity.LegalName
		result.Address = identity.Address
		result.Phone = identity.Phone
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if err != nil {
		unavailable(w)
		return
	}
	respond(w, result)
}

func (s *Service) approvedPlans(w http.ResponseWriter, r *http.Request) {
	requested := strings.Split(r.URL.Query().Get("ids"), ",")
	if len(requested) > 100 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	ids := make([]uuid.UUID, 0, len(requested))
	for _, raw := range requested {
		id, err := uuid.Parse(raw)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		ids = append(ids, id)
	}
	ctx := r.Context()
	rows, err := s.main.Query(ctx, "SELECT id,coach_id,plan_epoch,coach_epoch FROM identity_boundary.published_plans WHERE id=ANY($1)", ids)
	if err != nil {
		unavailable(w)
		return
	}
	type eligible struct{ ID, Coach, PlanEpoch, CoachEpoch uuid.UUID }
	plans, err := pgx.CollectRows(rows, pgx.RowToStructByPos[eligible])
	if err != nil {
		unavailable(w)
		return
	}
	planIDs := make([]uuid.UUID, 0, len(plans))
	coaches := make([]uuid.UUID, 0, len(plans))
	planEpochs := make([]uuid.UUID, 0, len(plans))
	coachEpochs := make([]uuid.UUID, 0, len(plans))
	for _, p := range plans {
		planIDs = append(planIDs, p.ID)
		coaches = append(coaches, p.Coach)
		planEpochs = append(planEpochs, p.PlanEpoch)
		coachEpochs = append(coachEpochs, p.CoachEpoch)
	}
	rows, err = s.vault.Query(ctx, `SELECT d.plan_id FROM identity_vault.disclosures d
		JOIN unnest($1::uuid[],$2::uuid[],$3::uuid[],$4::uuid[]) AS p(plan_id,coach_id,plan_epoch,coach_epoch)
		USING (plan_id,coach_id,plan_epoch,coach_epoch)
		JOIN identity_vault.identities i ON i.coach_id=d.coach_id
		LEFT JOIN identity_vault.commerce c ON c.coach_id=d.coach_id
		WHERE c.required=false OR i.phone_enc IS NOT NULL`, planIDs, coaches, planEpochs, coachEpochs)
	if err != nil {
		unavailable(w)
		return
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		unavailable(w)
		return
	}
	respond(w, out)
}
