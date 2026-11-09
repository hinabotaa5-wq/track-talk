package httpx

import (
	"crypto/sha256"
	"net/http"
	"slices"
	"sort"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/jst"
	"time"
)

// サポートプランの入口のコーチ一覧（本体 #486）。動画添削の指名一覧とは別のリストで、
// 「大会まで」（無料相談の受付設定がある）か「対面」（対面プランを公開している）のコーチだけを出す。
// 人気中立：★・件数・売上・契約件数・残り枠は返さない。並び順は日替わりの公平なローテーション。
// 大会までの受け入れ状況はコーチの手動入力（受付設定）だけで決め、カレンダーとは連動しない。
// 対面はカレンダーから自動で判定する。カレンダーは日ごとの「空き・相談・不可」しか持たないので、
// 今後21日に「空き」の日があれば受付中とする（プランの所要時間ぶんの連続した空きでの判定は、カレンダーに時間帯が無いため未実装）。

// supportRotationKey は、日本時間の日付とコーチの ID から決まる並びの鍵。日ごとに順番が入れ替わり、特定のコーチが上に固定されない。
func supportRotationKey(day string, id uuid.UUID) [32]byte {
	return sha256.Sum256(append([]byte(day), id[:]...))
}

func rotateCoaches(ids []uuid.UUID, now time.Time) []uuid.UUID {
	day := now.In(jst.Zone).Format(time.DateOnly)
	out := slices.Clone(ids)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := supportRotationKey(day, out[i]), supportRotationKey(day, out[j])
		return string(a[:]) < string(b[:])
	})
	return out
}

func (s *Server) listSupportPlanCoaches(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := s.q.ListPublicCoaches(ctx)
	if err != nil {
		MapDBError(w, err)
		return
	}
	plans, err := s.q.ListPublicCoachingPlans(ctx, coachPledgeListedVersion)
	if err != nil {
		MapDBError(w, err)
		return
	}
	planIDs := make([]uuid.UUID, 0, len(plans))
	for _, p := range plans {
		planIDs = append(planIDs, p.ID)
	}
	approved, err := s.identity.approvedPlans(ctx, planIDs)
	if err != nil {
		approved = map[uuid.UUID]bool{}
	}
	inPerson := map[uuid.UUID][]map[string]any{}
	minPrice := map[uuid.UUID]int32{}
	areas := map[uuid.UUID][]string{}
	for _, p := range plans {
		if !approved[p.ID] || p.ServiceType == nil || *p.ServiceType != "in_person" {
			continue
		}
		area := ""
		if p.VenueArea != nil {
			area = *p.VenueArea
		}
		inPerson[p.CoachID] = append(inPerson[p.CoachID], map[string]any{"id": p.ID, "title": p.Title, "price_yen": p.PriceYen, "session_minutes": p.SessionMinutes, "venue_area": area})
		if m, ok := minPrice[p.CoachID]; !ok || p.PriceYen < m {
			minPrice[p.CoachID] = p.PriceYen
		}
		if area != "" && !slices.Contains(areas[p.CoachID], area) {
			areas[p.CoachID] = append(areas[p.CoachID], area)
		}
	}
	consults, err := s.publicConsultEntries(ctx, nil)
	if err != nil {
		consults = map[uuid.UUID]map[string]any{}
	}
	byID := map[uuid.UUID]db.ListPublicCoachesRow{}
	ids := []uuid.UUID{}
	for _, c := range rows {
		if consults[c.ProfileID] == nil && len(inPerson[c.ProfileID]) == 0 {
			continue
		}
		byID[c.ProfileID] = c
		ids = append(ids, c.ProfileID)
	}
	now := time.Now()
	today := availabilityToday(now)
	out := make([]map[string]any, 0, len(ids))
	for _, id := range rotateCoaches(ids, now) {
		c := byID[id]
		kinds := []string{}
		entry := map[string]any{
			"id": id, "nickname": c.Nickname, "photo_url": s.coachPhotoURL(c.PhotoPublicID), "events": c.Events,
		}
		if cs := consults[id]; cs != nil {
			kinds = append(kinds, "competition_support")
			entry["competition_support"] = cs
		}
		if ps := inPerson[id]; len(ps) > 0 {
			kinds = append(kinds, "in_person")
			days, err := s.q.ListCoachAvailability(ctx, db.ListCoachAvailabilityParams{CoachID: id, Today: today})
			if err != nil {
				MapDBError(w, err)
				return
			}
			open := slices.ContainsFunc(days, func(d db.CoachAvailability) bool { return d.Status == "available" })
			status := "paused"
			if open {
				status = "open"
			}
			entry["in_person"] = map[string]any{"plans": ps, "min_price_yen": minPrice[id], "areas": nonNilStrings(areas[id]), "reception": map[string]any{"status": status}}
		}
		entry["kinds"] = kinds
		out = append(out, entry)
	}
	JSON(w, http.StatusOK, map[string]any{"coaches": out})
}
