package httpx

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/jst"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// SystemProfileID は実況スレッドの投稿者になる運営用固定プロフィールのID。
// マイグレーション 000065 で profiles に1行入れる。Supabase Auth のユーザーは無いのでログインできない。
var SystemProfileID = uuid.MustParse("f47ac10b-58cc-4372-a567-0e02b2c3d479")

type meetThreadDTO struct {
	MeetKey      string `json:"meet_key"`
	Name         string `json:"name"`
	DateLabel    string `json:"date_label"`
	PostID       string `json:"post_id"`
	CommentCount int    `json:"comment_count"`
	Href         string `json:"href"`
	// NowRunning はいま実施中の種目(例: 「女子100m予選」)。タイムテーブルが入っていない大会では空。
	NowRunning []string `json:"now_running"`
}

// meetTarget は今日開催中の対象大会。スレッドを1本ずつ用意する。
type meetTarget struct {
	key       string
	name      string
	dateLabel string
	title     string
	body      string
}

// featuredInternationalPatterns はホームの吹き出しに出す国際大会の絞り込み。
// 元は apps/web/lib/today-meets.ts にあった判定で、認可を API 側に持たせるためこちらへ移した。
var featuredInternationalPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^オリンピック`),
	regexp.MustCompile(`^世界陸上`),
	regexp.MustCompile(`^世界室内陸上`),
	regexp.MustCompile(`^U20世界陸上`),
	regexp.MustCompile(`ダイヤモンドリーグ`),
	regexp.MustCompile(`アジア(競技)?大会`),
	regexp.MustCompile(`(?i)^(Summer )?Olympic Games`),
	regexp.MustCompile(`(?i)^World Athletics (U20 |Indoor )?Championships`),
	regexp.MustCompile(`(?i)Diamond League`),
	regexp.MustCompile(`(?i)Asian Games`),
}

// todayMeetThreads は今日開催中の対象大会の実況スレッドを返し、無ければその場で1本だけ作る。
// 定期実行はせず、ホームを開いたリクエストのときに作る。ログインは不要(スレッドは誰でも読める)。
func (s *Server) todayMeetThreads(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	now := s.now()
	today := now.In(jst.Zone).Format("2006-01-02")
	start, end := seasonRange(currentSeasonYear(now))

	targets, err := s.meetTargets(ctx, start, end, func(starts, ends time.Time) bool { return meetIsOngoing(starts, ends, today) })
	if err != nil {
		MapDBError(w, err)
		return
	}
	if len(targets) == 0 {
		JSON(w, http.StatusOK, map[string]any{"meets": []meetThreadDTO{}})
		return
	}

	keys := make([]string, 0, len(targets))
	for _, t := range targets {
		if err := s.ensureMeetThread(ctx, t.key, t.title, t.body); err != nil {
			MapDBError(w, err)
			return
		}
		keys = append(keys, t.key)
	}

	rows, err := s.q.ListMeetThreads(ctx, keys)
	if err != nil {
		MapDBError(w, err)
		return
	}
	byKey := make(map[string]db.ListMeetThreadsRow, len(rows))
	for _, row := range rows {
		byKey[row.MeetKey] = row
	}

	sessions, err := s.q.ListMeetSessions(ctx, keys)
	if err != nil {
		MapDBError(w, err)
		return
	}
	running := nowRunningByMeet(sessions, now)

	meets := make([]meetThreadDTO, 0, len(targets))
	for _, t := range targets {
		row, ok := byKey[t.key]
		if !ok {
			continue
		}
		meets = append(meets, meetThreadDTO{
			MeetKey:      t.key,
			Name:         t.name,
			DateLabel:    t.dateLabel,
			PostID:       row.PostID.String(),
			CommentCount: int(row.CommentCount),
			Href:         "/posts/" + row.PostID.String(),
			NowRunning:   nonNilStrings(running[t.key]),
		})
	}
	JSON(w, http.StatusOK, map[string]any{"meets": meets})
}

// meetTargets は対象大会(国内主要大会はすべて、国際大会は featured だけ)のうち、開催期間が keep を満たすものを集める。
func (s *Server) meetTargets(ctx context.Context, start, end time.Time, keep func(starts, ends time.Time) bool) ([]meetTarget, error) {
	var targets []meetTarget

	majors, err := s.q.ListDomesticMajors(ctx, db.ListDomesticMajorsParams{SeasonStart: start, SeasonEnd: end})
	if err != nil {
		return nil, err
	}
	for _, m := range majors {
		if !keep(m.StartsOn, m.EndsOn) {
			continue
		}
		dateLabel := meetDateLabel(m.StartsOn)
		targets = append(targets, meetTarget{
			key:       fmt.Sprintf("m:%s:%d", m.Slug, m.Year),
			name:      m.Name,
			dateLabel: dateLabel,
			title:     meetTitle(m.Name, dateLabel),
			body:      meetBody(m.Venue, m.SiteUrl),
		})
	}

	comps, err := s.q.ListCompetitions(ctx, db.ListCompetitionsParams{Prefecture: "00", MonthStart: start, MonthEnd: end})
	if err != nil {
		return nil, err
	}
	for _, c := range comps {
		ends := c.StartsOn
		if c.EndsOn != nil {
			ends = *c.EndsOn
		}
		if !keep(c.StartsOn, ends) {
			continue
		}
		if !isFeaturedInternational(c.Name, c.OrganizerUrl, c.SourceUrl) {
			continue
		}
		dateLabel := meetDateLabel(c.StartsOn)
		link := firstNonEmpty(c.GuidelinesUrl, c.OrganizerUrl, c.SourceUrl)
		targets = append(targets, meetTarget{
			key:       fmt.Sprintf("c:%s:%d:%s:%s", c.Prefecture, c.SeasonYear, c.Name, c.StartsOn.Format("2006-01-02")),
			name:      c.Name,
			dateLabel: dateLabel,
			title:     meetTitle(c.Name, dateLabel),
			body:      meetBody(c.Location, link),
		})
	}
	return targets, nil
}

// ensureMeetThread は meet_key のスレッドを1本だけ用意する。既にあれば何もしない。
// 同時にリクエストが来ても、meet_key の主キーで1つに収束させる(負けた方はロールバックされ、投稿も残らない)。
func (s *Server) ensureMeetThread(ctx context.Context, key, title, body string) error {
	if _, err := s.q.GetMeetThreadPostID(ctx, key); err == nil {
		return nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	postID := uuid.New()
	err := s.tx(ctx, func(q *db.Queries) error {
		if err := q.CreateMeetPost(ctx, db.CreateMeetPostParams{
			ID:       postID,
			AuthorID: SystemProfileID,
			Title:    title,
			Body:     body,
		}); err != nil {
			return err
		}
		return q.InsertMeetThread(ctx, db.InsertMeetThreadParams{MeetKey: key, PostID: postID})
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return nil
	}
	return err
}

// meetIsOngoing は開催日(開始日〜終了日)のあいだかどうか。日付は JST の今日(YYYY-MM-DD)と比べる。
func meetIsOngoing(starts, ends time.Time, today string) bool {
	return starts.Format("2006-01-02") <= today && ends.Format("2006-01-02") >= today
}

func meetDateLabel(t time.Time) string {
	return fmt.Sprintf("%d年%d月%d日", t.Year(), int(t.Month()), t.Day())
}

// meetTitle は「<大会名>（<開催日>）実況」。posts.title は120文字までなので、長い大会名は詰める。
func meetTitle(name, dateLabel string) string {
	suffix := "（" + dateLabel + "）実況"
	budget := 120 - len([]rune(suffix))
	runes := []rune(name)
	if len(runes) > budget {
		if budget < 1 {
			budget = 1
		}
		name = string(runes[:budget-1]) + "…"
	}
	return name + suffix
}

func meetBody(venue, link string) string {
	var lines []string
	if v := strings.TrimSpace(venue); v != "" {
		lines = append(lines, "会場: "+v)
	}
	if l := strings.TrimSpace(link); l != "" {
		lines = append(lines, "公式サイト: "+l)
	}
	if len(lines) == 0 {
		return "大会当日の実況スレッドです。"
	}
	return strings.Join(lines, "\n")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func isFeaturedInternational(name, organizerURL, sourceURL string) bool {
	n := strings.TrimSpace(name)
	for _, pattern := range featuredInternationalPatterns {
		if pattern.MatchString(n) {
			return true
		}
	}
	return isDiamondLeagueSite(organizerURL) || isDiamondLeagueSite(sourceURL)
}

func isDiamondLeagueSite(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	host := u.Hostname()
	return host == "diamondleague.com" || strings.HasSuffix(host, ".diamondleague.com")
}

// nonNilStrings は、JSON に null ではなく [] を出すために、空のスライスを返す。
func nonNilStrings(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}
