package httpx

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
)

// カーソルは公開ホーム専用。権限判定には使わず、全入力をSQLの引数として渡す。
type homeCursor struct {
	Version int       `json:"v"`
	Scope   string    `json:"s"`
	AsOf    time.Time `json:"a"`
	Time    time.Time `json:"t"`
	ID      uuid.UUID `json:"id"`
	Likes   int32     `json:"l"`
}

func homeCursorScope(sort, kind, q string) string {
	sum := sha256.Sum256([]byte(sort + "\x00" + kind + "\x00" + q))
	return hex.EncodeToString(sum[:])
}
func encodeHomeCursor(cursor homeCursor) string {
	data, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(data)
}
func decodeHomeCursor(raw, scope string, now time.Time) (homeCursor, bool) {
	var c homeCursor
	if len(raw) > 2048 {
		return c, false
	}
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || json.Unmarshal(data, &c) != nil {
		return c, false
	}
	return c, c.Version == 1 && c.Scope == scope && c.ID != uuid.Nil && c.Likes >= 0 && !c.Time.IsZero() && !c.Time.After(c.AsOf) && !c.AsOf.After(now.Add(time.Minute)) && c.AsOf.After(now.Add(-24*time.Hour))
}

func (s *Server) listHomePosts(w http.ResponseWriter, r *http.Request, kind *string) {
	page, per, _ := s.queryPage(r)
	sort := "new"
	if r.URL.Query().Get("sort") == "popular" {
		sort = "popular"
	}
	kindValue := ""
	if kind != nil {
		kindValue = *kind
	}
	scope := homeCursorScope(sort, kindValue, strings.TrimSpace(r.URL.Query().Get("q")))
	cursor := homeCursor{Version: 1, Scope: scope, AsOf: time.Now().UTC()}
	cursor.Time, cursor.ID, cursor.Likes = cursor.AsOf, uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff"), math.MaxInt32
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		var ok bool
		cursor, ok = decodeHomeCursor(raw, scope, time.Now())
		if !ok {
			Error(w, http.StatusBadRequest, "invalid_cursor", "一覧を更新して、もう一度お試しください")
			return
		}
	}
	keyword, codes := searchArgs(r.URL.Query().Get("q"))
	// 余分に1件だけ取得して続きを判定する。総件数は集計しない。
	rows := make([]db.ListPostsNewRow, 0, per+1)
	if sort == "popular" {
		found, err := s.q.ListHomePostsPopular(r.Context(), db.ListHomePostsPopularParams{Kind: kind, AsOf: cursor.AsOf, CursorTime: cursor.Time, CursorID: cursor.ID, CursorLikes: cursor.Likes, Keyword: keyword, PrefectureCodes: codes, LimitN: per + 1})
		if err != nil {
			MapDBError(w, err)
			return
		}
		for _, row := range found {
			rows = append(rows, db.ListPostsNewRow(row))
		}
	} else {
		found, err := s.q.ListHomePostsNew(r.Context(), db.ListHomePostsNewParams{Kind: kind, AsOf: cursor.AsOf, CursorTime: cursor.Time, CursorID: cursor.ID, Keyword: keyword, PrefectureCodes: codes, LimitN: per + 1})
		if err != nil {
			MapDBError(w, err)
			return
		}
		for _, row := range found {
			rows = append(rows, db.ListPostsNewRow(row))
		}
	}
	hasMore := len(rows) > int(per)
	if hasMore {
		rows = rows[:per]
	}
	ids := idsFromNew(rows)
	liked, saved := s.likedSet(r.Context(), ids), s.savedSet(r.Context(), ids)
	items := make([]postDTO, 0, len(rows))
	for _, row := range rows {
		item := withMine(r, s.present(postFromList(row, liked[row.ID], true)))
		item.SavedByMe = saved[row.ID]
		items = append(items, item)
	}
	if err := s.attachRecruitmentCovers(r.Context(), items); err != nil {
		MapDBError(w, err)
		return
	}
	if err := s.attachPracticeSummaries(r.Context(), items); err != nil {
		MapDBError(w, err)
		return
	}
	next := ""
	if hasMore {
		last := rows[len(rows)-1]
		cursor.Time, cursor.ID, cursor.Likes = last.CreatedAt, last.ID, last.LikeCount
		next = encodeHomeCursor(cursor)
	}
	JSON(w, http.StatusOK, map[string]any{"posts": items, "meta": map[string]any{"page": page, "per_page": per, "total": 0, "has_more": hasMore, "next_cursor": next}})
}
