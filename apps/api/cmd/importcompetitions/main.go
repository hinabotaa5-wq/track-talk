package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/competitions"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	prefecture := flag.String("prefecture", "", "取り込む都道府県コード。00 は国際大会。空なら国際大会と、Gemini の1日の上限（GEMINI_DAILY_REQUESTS、既定15回）に収まる県まで")
	dryRun := flag.Bool("dry-run", false, "DB に書かず、読み取れた大会を表示する。-prefecture が必要")
	news := flag.Bool("news", false, "県陸協の新着から要項を補う。-prefecture があればその県だけ。-dry-run なら DB を書き換えず、補える要項を表示する")
	pageURL := flag.String("url", "", "-dry-run のとき、未登録の県でも試せる日程ページの URL")
	flag.Parse()

	if *dryRun && !*news {
		if err := runDryRun(*prefecture, *pageURL); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *pageURL != "" {
		log.Fatal("-url は -dry-run と一緒に使います")
	}

	_ = godotenv.Load()
	_ = godotenv.Load("../../.env")
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	if !(*news && *dryRun) {
		removed, err := db.New(pool).DeletePastCompetitionSeasons(ctx, int32(competitions.SeasonYear(time.Now())))
		if err != nil {
			log.Fatal(err)
		}
		if removed > 0 {
			fmt.Fprintf(os.Stdout, "dropped %d competitions from previous seasons\n", removed)
		}
	}

	if *news {
		if isInternational(*prefecture) {
			log.Fatal("-news は都道府県だけに使えます")
		}
		key := os.Getenv("GEMINI_API_KEY")
		if key == "" {
			log.Fatal("GEMINI_API_KEY is required")
		}
		if *prefecture == "" {
			log.Fatal("-news には -prefecture が必要です")
		}
		sources, err := newsTargets(ctx, pool, *prefecture, -1)
		if err != nil {
			log.Fatal(err)
		}
		if importNews(ctx, pool, competitions.Client{Key: key}, sources, -1, *dryRun) {
			os.Exit(1)
		}
		return
	}

	if isInternational(*prefecture) {
		if err := importInternational(ctx, pool); err != nil {
			log.Fatal(err)
		}
		return
	}

	var failed bool
	importedInternational := false
	if *prefecture == "" {
		updated, err := importTimes(ctx, pool)
		if err != nil {
			log.Fatal(err)
		}
		if internationalDue(updated, time.Now()) {
			if err := importInternational(ctx, pool); err != nil {
				log.Printf("%s: %v", competitions.InternationalPrefecture, err)
				failed = true
			} else {
				importedInternational = true
			}
		}
	}

	targets, err := selectTargets(ctx, pool, *prefecture)
	if err != nil {
		log.Fatal(err)
	}
	// Annual schedules come first. News gets only the Gemini requests they leave over, so on the days in April
	// and October when prefectures are being imported, almost nothing is left for it.
	newsBudget := 0
	if *prefecture == "" {
		limit, err := dailyRequestLimit()
		if err != nil {
			log.Fatal(err)
		}
		newsBudget = limit
		for _, source := range targets {
			newsBudget -= source.Requests()
		}
	}
	var newsSources []competitions.Source
	if newsBudget > 0 {
		newsSources, err = newsTargets(ctx, pool, "", newsBudget)
		if err != nil {
			log.Fatal(err)
		}
	}
	if len(targets) == 0 && len(newsSources) == 0 {
		if failed {
			os.Exit(1)
		}
		if *prefecture == "" && !importedInternational {
			fmt.Fprintln(os.Stdout, "今日の取り込み対象はありません")
		}
		return
	}

	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		log.Fatal("GEMINI_API_KEY is required")
	}
	client := competitions.Client{Key: key}
	for i, source := range targets {
		if i > 0 {
			time.Sleep(15 * time.Second)
		}
		if err := importSource(ctx, pool, client, source); err != nil {
			log.Printf("%s: %v", source.Prefecture, err)
			if errors.Is(err, competitions.ErrRateLimited) {
				os.Exit(1)
			}
			failed = true
		}
	}
	if len(newsSources) > 0 {
		// Same pacing as between prefectures, so the free tier's per-minute limit is not hit.
		if len(targets) > 0 {
			time.Sleep(15 * time.Second)
		}
		if importNews(ctx, pool, client, newsSources, newsBudget, false) {
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}

func selectTargets(ctx context.Context, pool *pgxpool.Pool, prefecture string) ([]competitions.Source, error) {
	if prefecture != "" {
		code := prefecture
		if len(code) == 1 {
			code = "0" + code
		}
		for _, source := range competitions.Sources {
			if source.Prefecture == code {
				return []competitions.Source{source}, nil
			}
		}
		return nil, fmt.Errorf("未登録の都道府県です: %s", prefecture)
	}
	updated, err := importTimes(ctx, pool)
	if err != nil {
		return nil, err
	}
	limit, err := dailyRequestLimit()
	if err != nil {
		return nil, err
	}
	for _, source := range competitions.Sources {
		if source.Requests() > limit {
			log.Printf("%s: Gemini を %d 回使うので、1日 %d 回の上限では取り込めません", source.Prefecture, source.Requests(), limit)
		}
	}
	return competitions.DueSources(competitions.Sources, updated, time.Now(), limit), nil
}

// dailyRequestLimit lets GEMINI_DAILY_REQUESTS follow the project's requests-per-day shown in AI Studio.
func dailyRequestLimit() (int, error) {
	raw := strings.TrimSpace(os.Getenv("GEMINI_DAILY_REQUESTS"))
	if raw == "" {
		return competitions.DailyRequestLimit, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 1 {
		return 0, fmt.Errorf("GEMINI_DAILY_REQUESTS は1以上の整数にしてください: %q", raw)
	}
	return limit, nil
}

func importTimes(ctx context.Context, pool *pgxpool.Pool) (map[string]time.Time, error) {
	rows, err := pool.Query(ctx, `SELECT prefecture, MAX(updated_at) FROM competitions GROUP BY prefecture`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	updated := map[string]time.Time{}
	for rows.Next() {
		var prefecture string
		var at time.Time
		if err := rows.Scan(&prefecture, &at); err != nil {
			return nil, err
		}
		updated[prefecture] = at
	}
	return updated, rows.Err()
}

func readMeets(ctx context.Context, client competitions.Client, source competitions.Source) ([]competitions.Meet, error) {
	urls, err := source.ResolveURLs(ctx)
	if err != nil {
		return nil, err
	}
	if len(urls) == 0 {
		return nil, errors.New("日程ページのURLがありません")
	}
	// Consecutive PDFs go to Gemini together so a meet listed in several of them is returned once.
	var pages []competitions.Page
	var pageURLs []string
	for _, u := range urls {
		page, err := competitions.FetchPage(ctx, u)
		if err != nil {
			// A month that is not published yet must not stop the other pages of the same prefecture.
			if len(urls) > 1 {
				log.Printf("%s: %s を飛ばします: %v", source.Prefecture, u, err)
				continue
			}
			return nil, err
		}
		if n := len(pages); n > 0 && len(page.PDFs) > 0 && len(pages[n-1].PDFs) > 0 {
			pages[n-1].PDFs = append(pages[n-1].PDFs, page.PDFs...)
			continue
		}
		pages = append(pages, page)
		pageURLs = append(pageURLs, u)
	}
	var allMeets []competitions.Meet
	seen := map[string]bool{}
	for i, page := range pages {
		u := pageURLs[i]
		if i > 0 {
			time.Sleep(5 * time.Second)
		}
		drafts, err := client.Extract(ctx, page)
		if err != nil {
			return nil, err
		}
		meets, err := competitions.BuildMeets(page, drafts, source.Prefecture, u)
		if err != nil {
			// If one of multiple URLs has no meets, continue to others unless all fail
			if len(pages) > 1 && strings.Contains(err.Error(), "大会がありません") {
				continue
			}
			return nil, err
		}
		for _, meet := range meets {
			key := meet.Name + "|" + meet.StartsOn.Format("2006-01-02")
			if !seen[key] {
				seen[key] = true
				allMeets = append(allMeets, meet)
			}
		}
	}
	if len(allMeets) == 0 {
		return nil, fmt.Errorf("大会がありません")
	}
	sort.SliceStable(allMeets, func(i, j int) bool {
		return allMeets[i].StartsOn.Before(allMeets[j].StartsOn)
	})
	return allMeets, nil
}

// runDryRun reads one prefecture page and prints what would be imported.
// It never touches the database, so a new URL can be checked before it is registered.
func runDryRun(prefecture, pageURL string) error {
	code := strings.TrimSpace(prefecture)
	if code == "" {
		return errors.New("-dry-run には -prefecture が必要です")
	}
	if len(code) == 1 {
		code = "0" + code
	}
	if isInternational(code) {
		return errors.New("-dry-run は都道府県の日程ページだけに使えます")
	}
	source := competitions.Source{Prefecture: code, PageURL: pageURL}
	if pageURL == "" {
		found := false
		for _, registered := range competitions.Sources {
			if registered.Prefecture == code {
				source, found = registered, true
				break
			}
		}
		if !found {
			return fmt.Errorf("未登録の都道府県です。-url で日程ページを指定してください: %s", code)
		}
	}
	_ = godotenv.Load()
	_ = godotenv.Load("../../.env")
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		return errors.New("GEMINI_API_KEY is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	meets, err := readMeets(ctx, competitions.Client{Key: key}, source)
	if err != nil {
		return err
	}
	printMeets(os.Stdout, meets, source)
	return nil
}

func printMeets(w io.Writer, meets []competitions.Meet, source competitions.Source) {
	fmt.Fprintf(w, "%d competitions for prefecture %s season %d (%s)\n", len(meets), source.Prefecture, meets[0].SeasonYear, strings.Join(source.URLs(), ", "))
	withLink := 0
	for _, meet := range meets {
		date := meet.StartsOn.Format("2006-01-02")
		if meet.EndsOn != nil {
			date += "~" + meet.EndsOn.Format("01-02")
		}
		link := "-"
		if meet.OrganizerURL != meet.SourceURL {
			withLink++
			link = meet.LinkLabel
		}
		fmt.Fprintf(w, "  %s  %s  @%s  [%s]\n", date, meet.Name, meet.Location, link)
	}
	fmt.Fprintf(w, "%d of %d have their own guidelines/district link\n", withLink, len(meets))
}

func importSource(ctx context.Context, pool *pgxpool.Pool, client competitions.Client, source competitions.Source) error {
	meets, err := readMeets(ctx, client, source)
	if err != nil {
		return err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)
	season := int32(meets[0].SeasonYear)
	if err := q.DeleteCompetitionsBeforeSeason(ctx, db.DeleteCompetitionsBeforeSeasonParams{
		Prefecture: source.Prefecture,
		SeasonYear: season,
	}); err != nil {
		return err
	}
	// Meets that are still in the table are updated in place, so the guidelines found from news survive the re-import.
	// Only meets that left the table are removed.
	keys := make([]string, 0, len(meets))
	for _, meet := range meets {
		keys = append(keys, meet.Name+"|"+meet.StartsOn.Format("2006-01-02"))
	}
	if err := q.DeleteCompetitionsBySeasonExcept(ctx, db.DeleteCompetitionsBySeasonExceptParams{
		Prefecture: source.Prefecture,
		SeasonYear: season,
		Keys:       keys,
	}); err != nil {
		return err
	}
	for _, meet := range meets {
		if err := q.UpsertCompetition(ctx, db.UpsertCompetitionParams{
			ID:           uuid.New(),
			Name:         meet.Name,
			StartsOn:     meet.StartsOn,
			EndsOn:       meet.EndsOn,
			Location:     meet.Location,
			Prefecture:   meet.Prefecture,
			SeasonYear:   int32(meet.SeasonYear),
			OrganizerUrl: meet.OrganizerURL,
			SourceUrl:    meet.SourceURL,
			LinkLabel:    meet.LinkLabel,
		}); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "imported %d competitions for prefecture %s season %d\n", len(meets), source.Prefecture, meets[0].SeasonYear)
	return nil
}

func isInternational(prefecture string) bool {
	switch strings.TrimSpace(prefecture) {
	case "0", "00", "international":
		return true
	default:
		return false
	}
}

func internationalDue(updated map[string]time.Time, now time.Time) bool {
	imported, ok := updated[competitions.InternationalPrefecture]
	return !ok || imported.IsZero() || now.Sub(imported) >= competitions.InternationalRefreshAfter
}

func importInternational(ctx context.Context, pool *pgxpool.Pool) error {
	now := time.Now()
	meets, err := competitions.FetchInternational(ctx, now)
	if err != nil {
		return err
	}
	_, _, seasons := competitions.InternationalWindow(now)
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)
	if len(seasons) > 0 {
		if err := q.DeleteCompetitionsBeforeSeason(ctx, db.DeleteCompetitionsBeforeSeasonParams{
			Prefecture: competitions.InternationalPrefecture,
			SeasonYear: int32(seasons[0]),
		}); err != nil {
			return err
		}
	}
	for _, season := range seasons {
		if err := q.DeleteCompetitionsBySeason(ctx, db.DeleteCompetitionsBySeasonParams{
			Prefecture: competitions.InternationalPrefecture,
			SeasonYear: int32(season),
		}); err != nil {
			return err
		}
	}
	for _, meet := range meets {
		if err := q.UpsertCompetition(ctx, db.UpsertCompetitionParams{
			ID:           uuid.New(),
			Name:         meet.Name,
			StartsOn:     meet.StartsOn,
			EndsOn:       meet.EndsOn,
			Location:     meet.Location,
			Prefecture:   meet.Prefecture,
			SeasonYear:   int32(meet.SeasonYear),
			OrganizerUrl: meet.OrganizerURL,
			SourceUrl:    meet.SourceURL,
			LinkLabel:    meet.LinkLabel,
		}); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "imported %d international competitions\n", len(meets))
	return nil
}
