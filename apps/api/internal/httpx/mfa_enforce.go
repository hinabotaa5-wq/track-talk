package httpx

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// mfaEnrollTTL は、二段階認証を設定済みかどうかを覚えておく時間。毎回 DB に聞かないため。
// 設定した直後はそのログインが aal2 になるので、覚えている間の古さは困らない。
const mfaEnrollTTL = time.Minute

// mfaEnrollCacheMax を超えたら、覚えている分をいったん捨てる。
const mfaEnrollCacheMax = 10000

type mfaEnrollEntry struct {
	enrolled bool
	until    time.Time
}

// MFAEnrollment は、二段階認証を設定済みの人を Supabase の auth.mfa_factors で調べる。
// 実行ロールは auth スキーマに入れないため、postgres 所有の関数 public.mfa_verified を経由する（migration 000153）。
// 設定済みの人は、コードを入れる前のログイン(aal1)では API を使えないようにする。
// 画面で止めるだけだと、パスワードだけ知った人がトークンで直に API を呼べてしまうため。
type MFAEnrollment struct {
	pool *pgxpool.Pool

	mu       sync.Mutex
	checked  bool
	disabled bool
	cache    map[uuid.UUID]mfaEnrollEntry
	now      func() time.Time
}

func NewMFAEnrollment(pool *pgxpool.Pool) *MFAEnrollment {
	return &MFAEnrollment{pool: pool, cache: map[uuid.UUID]mfaEnrollEntry{}, now: time.Now}
}

// Enrolled は、この人が確認済みの二段階認証を持っていれば true。
// auth.mfa_factors が無い DB(手元やテストの素の Postgres)では、いつも false。
func (m *MFAEnrollment) Enrolled(ctx context.Context, id uuid.UUID) (bool, error) {
	if m == nil || m.pool == nil {
		return false, nil
	}
	now := m.now()
	m.mu.Lock()
	if m.disabled {
		m.mu.Unlock()
		return false, nil
	}
	if e, ok := m.cache[id]; ok && now.Before(e.until) {
		m.mu.Unlock()
		return e.enrolled, nil
	}
	checked := m.checked
	m.mu.Unlock()

	if !checked {
		if err := m.probe(ctx); err != nil {
			return false, err
		}
		m.mu.Lock()
		disabled := m.disabled
		m.mu.Unlock()
		if disabled {
			return false, nil
		}
	}

	var enrolled bool
	err := m.pool.QueryRow(ctx,
		`SELECT public.mfa_verified($1)`, id,
	).Scan(&enrolled)
	if err != nil {
		return false, err
	}
	m.mu.Lock()
	if len(m.cache) >= mfaEnrollCacheMax {
		m.cache = map[uuid.UUID]mfaEnrollEntry{}
	}
	m.cache[id] = mfaEnrollEntry{enrolled: enrolled, until: now.Add(mfaEnrollTTL)}
	m.mu.Unlock()
	return enrolled, nil
}

// probe は、auth.mfa_factors を読めるかを一度だけ確かめる。
// 表が無い・読む権限が無いときは調べるのをやめる(全員が使えなくなるのを避ける)。本番でこうなったらログで気づけるようにする。
// つながらないなどの一時的な失敗は、次の呼び出しで確かめ直す。
func (m *MFAEnrollment) probe(ctx context.Context) error {
	// 関数の中で auth.mfa_factors を読むので、表・スキーマが無い／読めないときは従来と同じエラーコードになる。
	// 公開 API の実行ロールが関数を実行できないときも 42501（権限なし）になる。
	_, err := m.pool.Exec(ctx, `SELECT public.mfa_verified('00000000-0000-0000-0000-000000000000'::uuid)`)
	var pgErr *pgconn.PgError
	if err != nil && !(errors.As(err, &pgErr) && (pgErr.Code == "42P01" || pgErr.Code == "3F000" || pgErr.Code == "42501")) {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.checked = true
	if err != nil {
		m.disabled = true
		slog.Warn("mfa enforcement disabled: auth.mfa_factors is not readable", "code", pgErr.Code)
		return nil
	}
	slog.Info("mfa enforcement enabled")
	return nil
}
