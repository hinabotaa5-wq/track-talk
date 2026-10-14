package httpx

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
)

var (
	scrubEmail  = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	scrubURL    = regexp.MustCompile(`https?://\S+`)
	scrubHandle = regexp.MustCompile(`@[A-Za-z0-9_]{2,}`)
	// 電話番号や郵便番号のように、数字が区切り記号つきで長く続くもの。
	scrubDigits = regexp.MustCompile(`\d[\d\-－ー‐ ]{5,}\d`)
)

// scrubFreeText hides what most plainly points at a person: addresses, links, phone numbers, handles and
// the writer's own nickname. It cannot find names or places written in plain words, so archived rows
// stay review_status = 'pending' until someone has read them.
func scrubFreeText(text string, names ...string) string {
	out := scrubURL.ReplaceAllString(text, "[リンク]")
	out = scrubEmail.ReplaceAllString(out, "[メール]")
	out = scrubHandle.ReplaceAllString(out, "[ID]")
	out = scrubDigits.ReplaceAllString(out, "[番号]")
	for _, name := range names {
		if name = strings.TrimSpace(name); len([]rune(name)) >= 2 {
			out = strings.ReplaceAll(out, name, "[名前]")
		}
	}
	return out
}

type archiveStep struct {
	Stage string `json:"stage"`
	Days  int    `json:"days"`
}

type archiveUpdate struct {
	Stage string `json:"stage"`
	Body  string `json:"body"`
}

type archivePost struct {
	Stage   string          `json:"stage,omitempty"`
	Title   string          `json:"title"`
	Body    string          `json:"body"`
	Updates []archiveUpdate `json:"updates"`
}

func monthStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// archiveInjuryData copies a leaving member's injury records into injury_archives, one row per body part,
// without anything that links back to the account. It runs inside the withdrawal transaction, before the purge.
func archiveInjuryData(ctx context.Context, q *db.Queries, userID uuid.UUID, now time.Time) error {
	var names []string
	if nickname, err := q.GetProfileNickname(ctx, userID); err == nil && nickname != nil {
		names = append(names, *nickname)
	}
	members, err := q.ListMyInjuryGroupMembers(ctx, userID)
	if err != nil {
		return err
	}
	posts, err := q.ListMyInjuryPostsForArchive(ctx, userID)
	if err != nil {
		return err
	}

	type partRecord struct {
		injuredOn time.Time
		place     string
		situation string
		timeline  []archiveStep
		posts     []archivePost
	}
	records := map[string]*partRecord{}
	var order []string
	get := func(part string) *partRecord {
		if rec, ok := records[part]; ok {
			return rec
		}
		rec := &partRecord{timeline: []archiveStep{}, posts: []archivePost{}}
		records[part] = rec
		order = append(order, part)
		return rec
	}

	for _, m := range members {
		rec := get(m.BodyPart)
		rec.injuredOn = m.InjuredOn
		rec.place = scrubFreeText(m.Place, names...)
		rec.situation = scrubFreeText(m.Situation, names...)
		log, err := q.ListInjuryGroupStageLog(ctx, db.ListInjuryGroupStageLogParams{UserID: userID, BodyPart: m.BodyPart})
		if err != nil {
			return err
		}
		for _, entry := range log {
			days := int(entry.CreatedAt.Sub(log[0].CreatedAt).Hours() / 24)
			rec.timeline = append(rec.timeline, archiveStep{Stage: entry.Stage, Days: days})
		}
	}
	for _, p := range posts {
		rec := get(p.BodyPart)
		if rec.injuredOn.IsZero() || p.CreatedAt.Before(rec.injuredOn) {
			// 参加の記録がない古い投稿は、投稿した月を痛めた月の代わりにする。
			rec.injuredOn = p.CreatedAt
		}
		item := archivePost{
			Title:   scrubFreeText(p.Title, names...),
			Body:    scrubFreeText(p.Body, names...),
			Updates: []archiveUpdate{},
		}
		if p.GroupStage != nil {
			item.Stage = *p.GroupStage
		}
		updates, err := q.ListInjuryUpdates(ctx, p.ID)
		if err != nil {
			return err
		}
		for _, u := range updates {
			item.Updates = append(item.Updates, archiveUpdate{Stage: u.Stage, Body: scrubFreeText(u.Body, names...)})
		}
		rec.posts = append(rec.posts, item)
	}

	for _, part := range order {
		rec := records[part]
		timeline, err := json.Marshal(rec.timeline)
		if err != nil {
			return err
		}
		postsJSON, err := json.Marshal(rec.posts)
		if err != nil {
			return err
		}
		if err := q.InsertInjuryArchive(ctx, db.InsertInjuryArchiveParams{
			BodyPart:      part,
			InjuredMonth:  monthStart(rec.injuredOn),
			ArchivedMonth: monthStart(now),
			StageTimeline: timeline,
			Place:         rec.place,
			Situation:     rec.situation,
			Posts:         postsJSON,
		}); err != nil {
			return err
		}
	}
	return nil
}
