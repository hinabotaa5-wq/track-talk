package identityvault

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	main, vault  *pgxpool.Pool
	key          cipher.AEAD
	verifier     *auth.Verifier
	checkSession func(context.Context, string) error
	machineToken string
	cleanupToken string
	origin       string
}

func New(main, vault *pgxpool.Pool, verifier *auth.Verifier, authClient auth.Admin, key, machineToken, cleanupToken, origin string) (*Service, error) {
	raw, err := base64.StdEncoding.DecodeString(key)
	if err != nil || len(raw) != 32 || len(machineToken) < 32 || len(cleanupToken) < 32 || machineToken == cleanupToken || origin == "" || !authClient.Enabled() || verifier == nil {
		return nil, errors.New("invalid identity service configuration")
	}
	block, err := aes.NewCipher(raw)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Service{main: main, vault: vault, key: aead, verifier: verifier, checkSession: authClient.CheckSession, machineToken: machineToken, cleanupToken: cleanupToken, origin: origin}, nil
}

func (s *Service) Handler() http.Handler {
	r := http.NewServeMux()
	r.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	r.HandleFunc("GET /v1/coach", s.coach(s.coachView))
	r.HandleFunc("PUT /v1/coach/identity", s.coach(s.saveIdentity))
	r.HandleFunc("PUT /v1/coach/exemption", s.coach(s.exemption))
	r.HandleFunc("PUT /v1/coach/disclosures/{id}", s.coach(s.publish))
	r.HandleFunc("DELETE /v1/coach/disclosures/{id}", s.coach(s.unpublish))
	r.HandleFunc("POST /v1/admin/coaches/{id}/identity", s.admin(s.viewIdentity))
	r.HandleFunc("POST /v1/admin/coaches/{id}/require-disclosure", s.admin(s.forceRequired))
	r.HandleFunc("DELETE /v1/admin/coaches/{id}/disclosures", s.admin(s.revokeAll))
	r.HandleFunc("GET /internal/registration/{id}", s.machine(s.machineToken, s.registration))
	r.HandleFunc("GET /internal/disclosures", s.machine(s.machineToken, s.approvedPlans))
	r.HandleFunc("GET /internal/disclosures/{id}", s.machine(s.machineToken, s.disclosure))
	r.HandleFunc("POST /internal/cleanup", s.machine(s.cleanupToken, s.cleanup))
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'")
		w.Header().Set("Vary", "Origin")
		timeout := 15 * time.Second
		if req.URL.Path == "/internal/cleanup" {
			timeout = 3 * time.Minute
		}
		ctx, cancel := context.WithTimeout(req.Context(), timeout)
		defer cancel()
		req = req.WithContext(ctx)
		if origin := req.Header.Get("Origin"); origin != "" {
			if origin != s.origin || !strings.HasPrefix(req.URL.Path, "/v1/") {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, PUT, POST, DELETE")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			if req.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		r.ServeHTTP(w, req)
	})
}

func token(r *http.Request) string {
	value, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok {
		return ""
	}
	return value
}

func (s *Service) machine(expected string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(token(r)), []byte(expected)) != 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

type memberHandler func(http.ResponseWriter, *http.Request, uuid.UUID)

func (s *Service) coach(next memberHandler) http.HandlerFunc {
	return s.member(false, next)
}

func (s *Service) admin(next memberHandler) http.HandlerFunc {
	return s.member(true, next)
}

func (s *Service) member(admin bool, next memberHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, err := s.verifier.ParseIdentity(token(r))
		if err != nil || identity.SessionID == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if identity.AAL != "aal2" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if err := s.checkSession(r.Context(), token(r)); err != nil {
			if errors.Is(err, auth.ErrSessionInactive) {
				w.WriteHeader(http.StatusUnauthorized)
			} else {
				unavailable(w)
			}
			return
		}
		var coachID *uuid.UUID
		err = s.vault.QueryRow(r.Context(), "SELECT coach_id FROM identity_vault.members WHERE subject=$1 AND is_admin=$2", identity.UserID, admin).Scan(&coachID)
		if errors.Is(err, pgx.ErrNoRows) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if err != nil {
			unavailable(w)
			return
		}
		if admin {
			next(w, r, identity.UserID)
			return
		}
		var approved bool
		err = s.main.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM identity_boundary.approved_coaches WHERE profile_id=$1)", *coachID).Scan(&approved)
		if err != nil {
			unavailable(w)
			return
		}
		if !approved {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		next(w, r, *coachID)
	}
}

func unavailable(w http.ResponseWriter) {
	w.WriteHeader(http.StatusServiceUnavailable)
}

func respond(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil || d.Decode(&struct{}{}) != io.EOF {
		w.WriteHeader(http.StatusBadRequest)
		return false
	}
	return true
}

func idParam(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
	}
	return id, err == nil
}

func (s *Service) locked(ctx context.Context, coach uuid.UUID, apply func(pgx.Tx) error) error {
	tx, err := s.vault.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", coach.String()); err != nil {
		return err
	}
	if err := apply(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type identityFields struct {
	LegalName string `json:"legal_name"`
	Address   string `json:"address"`
}

func (s *Service) seal(coach uuid.UUID, field, value string) ([]byte, error) {
	nonce := make([]byte, s.key.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return s.key.Seal(nonce, nonce, []byte(value), []byte(coach.String()+"/"+field)), nil
}

func (s *Service) open(coach uuid.UUID, field string, sealed []byte) (string, error) {
	n := s.key.NonceSize()
	if len(sealed) < n {
		return "", errors.New("invalid ciphertext")
	}
	plain, err := s.key.Open(nil, sealed[:n], sealed[n:], []byte(coach.String()+"/"+field))
	return string(plain), err
}

func (s *Service) loadIdentity(ctx context.Context, tx pgx.Tx, coach uuid.UUID) (*identityFields, error) {
	var name, address []byte
	var keyID string
	err := tx.QueryRow(ctx, "SELECT key_id, legal_name_enc, address_enc FROM identity_vault.identities WHERE coach_id=$1", coach).Scan(&keyID, &name, &address)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if keyID != "v1" {
		return nil, errors.New("unsupported identity key")
	}
	legalName, err := s.open(coach, "legal_name", name)
	if err != nil {
		return nil, err
	}
	street, err := s.open(coach, "address", address)
	if err != nil {
		return nil, err
	}
	return &identityFields{LegalName: legalName, Address: street}, nil
}

func validText(value string, max int) bool {
	return value != "" && !strings.ContainsAny(value, "\x00\r\n") && utf8.RuneCountInString(value) <= max
}

var phonePattern = regexp.MustCompile(`^[0-9+()\- ]{10,20}$`)

func (s *Service) saveIdentity(w http.ResponseWriter, r *http.Request, coach uuid.UUID) {
	var input identityFields
	if !decode(w, r, &input) {
		return
	}
	input.LegalName = strings.TrimSpace(input.LegalName)
	input.Address = strings.TrimSpace(input.Address)
	if !validText(input.LegalName, 80) || !validText(input.Address, 200) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}
	ctx := r.Context()
	err := s.locked(ctx, coach, func(tx pgx.Tx) error {
		name, err := s.seal(coach, "legal_name", input.LegalName)
		if err != nil {
			return err
		}
		address, err := s.seal(coach, "address", input.Address)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO identity_vault.identities (coach_id,key_id,legal_name_enc,address_enc)
			VALUES ($1,'v1',$2,$3) ON CONFLICT (coach_id) DO UPDATE
			SET key_id=EXCLUDED.key_id,revision=gen_random_uuid(),legal_name_enc=EXCLUDED.legal_name_enc,address_enc=EXCLUDED.address_enc,ended_at=NULL`, coach, name, address); err != nil {
			return err
		}
		// Previously approved values do not authorize publication of replacements.
		return clearDisclosures(ctx, tx, coach)
	})
	if err != nil {
		unavailable(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func clearDisclosures(ctx context.Context, tx pgx.Tx, coach uuid.UUID) error {
	_, err := tx.Exec(ctx, "DELETE FROM identity_vault.disclosures WHERE coach_id=$1", coach)
	return err
}
