package httpx

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"github.com/hinata/track-talk/apps/api/internal/db"
)

// 海外コーチの指導希望の集計（本体 #542）。コーチタブの独立カードから登録する。
// 1ユーザー1件（サーバー側で一意）。登録数は利用者には見せない。
// 本体 #581：登録は1回きりで確定し、登録後の変更・取消・再登録は受け付けない。
const (
	overseasInterestMonthly = "monthly"
	overseasInterestSingle  = "single"
	overseasInterestBoth    = "both"
)

type overseasInterestDTO struct {
	Kind string `json:"kind"`
}

func overseasInterestJSON(kind string) overseasInterestDTO {
	return overseasInterestDTO{Kind: kind}
}

func validOverseasInterestKind(kind string) bool {
	return kind == overseasInterestMonthly || kind == overseasInterestSingle || kind == overseasInterestBoth
}

func (s *Server) getOverseasInterest(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	row, err := s.q.GetOverseasInterest(r.Context(), me.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		JSON(w, http.StatusOK, map[string]any{"interest": nil})
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"interest": overseasInterestJSON(row.Kind)})
}

type overseasInterestReq struct {
	Kind string `json:"kind"`
}

// 新規登録だけを同じ PUT で受ける（本体 #581）。登録済みの再登録・変更は 409 で拒否し、
// 行には触れない。重複登録は作らない。
func (s *Server) putOverseasInterest(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	var req overseasInterestReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	if !validOverseasInterestKind(req.Kind) {
		FieldError(w, map[string]string{"kind": "受けたい形は3つの中から選んでください"})
		return
	}
	if _, err := s.q.GetOverseasInterest(r.Context(), me.ID); err == nil {
		Error(w, http.StatusConflict, "conflict", "登録済みのため、種別は変えられません")
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
		MapDBError(w, err)
		return
	}
	row, err := s.q.UpsertOverseasInterest(r.Context(), db.UpsertOverseasInterestParams{
		ProfileID: me.ID,
		Kind:      req.Kind,
	})
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"interest": overseasInterestJSON(row.Kind)})
}

// 登録の取り消しは受け付けない（本体 #581）。登録済みなら 409 で拒否し、行には触れない。
// 未登録の取り消しは従来どおり何もない旨（null）を返す。
func (s *Server) deleteOverseasInterest(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	if _, err := s.q.GetOverseasInterest(r.Context(), me.ID); err == nil {
		Error(w, http.StatusConflict, "conflict", "登録済みのため、取り消しはできません")
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"interest": nil})
}

type overseasInterestAdminItem struct {
	Kind      string `json:"kind"`
	Nickname  string `json:"nickname"`
	CreatedAt string `json:"created_at"`
}

// 管理側の一覧（本体 #542）。総数・種別内訳・登録者の一覧を返す。登録データは
// 開始時のお知らせとパイロットの個別案内に使い、コーチ側へ個人データは提供しない。
func (s *Server) adminOverseasInterests(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	stats, err := s.q.AdminOverseasInterestStats(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	rows, err := s.q.AdminOverseasInterestList(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	items := make([]overseasInterestAdminItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, overseasInterestAdminItem{
			Kind:      row.Kind,
			Nickname:  row.Nickname,
			CreatedAt: row.CreatedAt.Format("2006-01-02 15:04"),
		})
	}
	JSON(w, http.StatusOK, map[string]any{
		"total": stats.Total,
		"breakdown": map[string]any{
			"monthly": stats.Monthly,
			"single":  stats.Single,
			"both":    stats.Both,
		},
		"interests": items,
	})
}
