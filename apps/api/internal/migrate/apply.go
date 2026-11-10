package migrate

import (
	"errors"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/hinata/track-talk/apps/api/migrations"
)

// Report は、本番の migration 実行ジョブ（cmd/dbmigrate）が出す結果。
// 版数・dirty・適用件数だけを持ち、接続先や SQL・エラーの原文は持たない。
type Report struct {
	Expected      uint `json:"expected_version"`
	BeforeVersion uint `json:"before_version"`
	BeforeDirty   bool `json:"before_dirty"`
	AfterVersion  uint `json:"after_version"`
	AfterDirty    bool `json:"after_dirty"`
	Applied       int  `json:"applied"`
}

// ErrDirty は、実行前の版が dirty のため何もしなかったことを表す。
// dirty の解消（force）は人の判断が要るため、このジョブは自動では行わない。
var ErrDirty = errors.New("database is dirty")

// ErrAhead は、本番の版がこのイメージの知る最新版より新しいことを表す（古いイメージでの実行）。
var ErrAhead = errors.New("database is ahead of this image")

// ExpectedVersion は、埋め込みの migration の最新版。
func ExpectedVersion() (uint, error) {
	versions, err := embeddedVersions()
	if err != nil {
		return 0, err
	}
	if len(versions) == 0 {
		return 0, errors.New("no migrations")
	}
	return versions[len(versions)-1], nil
}

func embeddedVersions() ([]uint, error) {
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return nil, err
	}
	seen := map[uint]bool{}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		head, _, ok := strings.Cut(name, "_")
		if !ok {
			continue
		}
		v, err := strconv.ParseUint(head, 10, 32)
		if err != nil {
			continue
		}
		seen[uint(v)] = true
	}
	out := make([]uint, 0, len(seen))
	for v := range seen {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

// Apply は未適用の migration を当て、前後の版を返す。適用済みなら何もしない（冪等）。
// 実行前が dirty のとき、または本番がイメージより新しいときは何も当てずにエラーを返す。
func Apply(databaseURL string) (Report, error) {
	var r Report
	expected, err := ExpectedVersion()
	if err != nil {
		return r, err
	}
	r.Expected = expected
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return r, err
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, withAppName(databaseURL))
	if err != nil {
		return r, err
	}
	defer m.Close()
	before, dirty, err := version(m)
	if err != nil {
		return r, err
	}
	r.BeforeVersion, r.BeforeDirty = before, dirty
	r.AfterVersion, r.AfterDirty = before, dirty
	if dirty {
		return r, ErrDirty
	}
	if before > expected {
		return r, ErrAhead
	}
	upErr := m.Up()
	after, afterDirty, verr := version(m)
	if verr == nil {
		r.AfterVersion, r.AfterDirty = after, afterDirty
	}
	if upErr != nil && !errors.Is(upErr, migrate.ErrNoChange) {
		return r, upErr
	}
	if verr != nil {
		return r, verr
	}
	versions, err := embeddedVersions()
	if err != nil {
		return r, err
	}
	for _, v := range versions {
		if v > before && v <= after {
			r.Applied++
		}
	}
	return r, nil
}

func version(m *migrate.Migrate) (uint, bool, error) {
	v, dirty, err := m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		return 0, false, nil
	}
	return v, dirty, err
}

// Status は何も当てずに、現在の版と未適用の件数だけを返す（読み取りのみ）。
func Status(databaseURL string) (Report, int, error) {
	var r Report
	expected, err := ExpectedVersion()
	if err != nil {
		return r, 0, err
	}
	r.Expected = expected
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return r, 0, err
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, withAppName(databaseURL))
	if err != nil {
		return r, 0, err
	}
	defer m.Close()
	v, dirty, err := version(m)
	if err != nil {
		return r, 0, err
	}
	r.BeforeVersion, r.BeforeDirty, r.AfterVersion, r.AfterDirty = v, dirty, v, dirty
	versions, err := embeddedVersions()
	if err != nil {
		return r, 0, err
	}
	pending := 0
	for _, ver := range versions {
		if ver > v {
			pending++
		}
	}
	return r, pending, nil
}
