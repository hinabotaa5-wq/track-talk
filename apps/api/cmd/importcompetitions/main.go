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
	pool, err := db.NewPool(ctx, databaseURL, db.AppNameImport)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	// 試行の記録表がなくても取り込みは続ける（読み書きの失敗は中のログだけ）。
	ensureImportAttemptsTable(ctx, pool)

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

	sel, err := selectTargets(ctx, pool, *prefecture)
	if err != nil {
		log.Fatal(err)
	}
	targets := sel.targets
	if len(sel.deferred) > 0 {
		log.Printf("budget deferred: %s", strings.Join(sel.deferred, ", "))
	}
	// 年間の日程表を先に読む。ニュースには、その残りの Gemini リクエストだけを回すので、県の取り込みがある4月と10月の日は、ニュースにはほとんど残らない。
	newsBudget := 0
	if *prefecture == "" {
		newsBudget = sel.limit - sel.used
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
	var results importResults
	windowStart := competitions.ImportWindowStart(time.Now())
	for i, source := range targets {
		if i > 0 {
			if err := competitions.Wait(ctx, 15*time.Second); err != nil {
				break
			}
		}
		// 部分成功の残りは、その URL だけを読み直す。見積もりも実消費も残り分だけになる。
		var retry []string
		if prev := prevAttempt(sel.attempts, source.Prefecture, windowStart); prev != nil && prev.Outcome == competitions.AttemptPartial {
			retry = prev.FailedURLs
		}
		outcome, err := importSource(ctx, pool, client, source, retry)
		if err != nil {
			log.Printf("%s: %v", source.Prefecture, err)
			results.recordOutcome(source.Prefecture, outcome.result, err, outcome.geminiCalls)
			recordImportAttempt(ctx, pool, source.Prefecture, nextAttempt(prevAttempt(sel.attempts, source.Prefecture, windowStart), windowStart, outcome.result, outcome.failed, outcome.geminiCalls))
			if errors.Is(err, competitions.ErrRateLimited) || errors.Is(err, competitions.ErrAuthentication) {
				fmt.Fprintln(os.Stdout, results.summary(len(targets), sel.limit))
				os.Exit(1)
			}
		} else {
			results.recordOutcome(source.Prefecture, outcome.result, nil, outcome.geminiCalls)
			recordImportAttempt(ctx, pool, source.Prefecture, nextAttempt(prevAttempt(sel.attempts, source.Prefecture, windowStart), windowStart, outcome.result, outcome.failed, outcome.geminiCalls))
		}
	}
	failed = failed || results.failed() || ctx.Err() != nil
	if len(newsSources) > 0 {
		// 県と県のあいだと同じ間隔をあけ、無料枠の1分あたりの上限に当たらないようにする。
		if len(targets) > 0 {
			if err := competitions.Wait(ctx, 15*time.Second); err != nil {
				os.Exit(1)
			}
		}
		if importNews(ctx, pool, client, newsSources, newsBudget, false, &results) {
			failed = true
		}
	}
	if len(targets) > 0 || len(newsSources) > 0 {
		fmt.Fprintln(os.Stdout, results.summary(len(targets), sel.limit))
	}
	if failed {
		os.Exit(1)
	}
}

// prevAttempt は、今の期間のその県の試行の記録だけを返す。
func prevAttempt(attempts map[string]competitions.Attempt, prefecture string, windowStart time.Time) *competitions.Attempt {
	if att, ok := attempts[prefecture]; ok && !att.AttemptedAt.Before(windowStart) {
		return &att
	}
	return nil
}

// selection は、その日の取り込み対象と、予算の使い道の内訳。
type selection struct {
	targets  []competitions.Source
	deferred []string
	attempts map[string]competitions.Attempt
	used     int
	limit    int
}

func selectTargets(ctx context.Context, pool *pgxpool.Pool, prefecture string) (selection, error) {
	var sel selection
	if prefecture != "" {
		code := prefecture
		if len(code) == 1 {
			code = "0" + code
		}
		for _, source := range competitions.Sources {
			if source.Prefecture == code {
				limit, err := dailyRequestLimit()
				if err != nil {
					return sel, err
				}
				sel.limit = limit
				sel.used = source.Requests()
				sel.attempts = loadImportAttempts(ctx, pool)
				sel.targets = append(sel.targets, source)
				return sel, nil
			}
		}
		return sel, fmt.Errorf("未登録の都道府県です: %s", prefecture)
	}
	updated, err := importTimes(ctx, pool)
	if err != nil {
		return sel, err
	}
	limit, err := dailyRequestLimit()
	if err != nil {
		return sel, err
	}
	sel.limit = limit
	for _, source := range competitions.Sources {
		if source.Requests() > limit {
			log.Printf("%s: Gemini を %d 回使うので、1日 %d 回の上限では取り込めません", source.Prefecture, source.Requests(), limit)
		}
	}
	sel.attempts = loadImportAttempts(ctx, pool)
	now := time.Now()
	windowStart := competitions.ImportWindowStart(now)
	costOf := func(source competitions.Source) int {
		var att *competitions.Attempt
		if a, ok := sel.attempts[source.Prefecture]; ok {
			att = &a
		}
		return source.EffectiveCost(att, windowStart)
	}
	sel.targets = competitions.DueSourcesWithAttempts(competitions.Sources, updated, sel.attempts, now, limit)
	for _, target := range sel.targets {
		sel.used += costOf(target)
	}
	// 予算に収まらず見送った県もログに残して、後続の県が届いていることを確かめられるようにする。
	selected := map[string]bool{}
	for _, target := range sel.targets {
		selected[target.Prefecture] = true
	}
	for _, source := range competitions.DueSourcesWithAttempts(competitions.Sources, updated, sel.attempts, now, 1<<30) {
		if !selected[source.Prefecture] {
			sel.deferred = append(sel.deferred, fmt.Sprintf("%s(%d)", source.Prefecture, costOf(source)))
		}
	}
	return sel, nil
}

// dailyRequestLimit は、GEMINI_DAILY_REQUESTS で AI Studio に出ているプロジェクトの1日あたりのリクエスト数に合わせられるようにする。
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

// readOutcome は、1つの県の読み取り結果。複数 URL の県は、成功した URL 分だけでも取り込む。
type readOutcome struct {
	meets []competitions.Meet
	// succeeded は大会が1件以上取れた読み取り単位のキー（最初の URL）。削除の範囲もここに絞る。
	succeeded []string
	// failed は Gemini を呼んだあとに失敗した読み取り単位のキー。翌日の残り見積もりになる。
	failed []string
	// skipped は表が見つからないなどで Gemini を呼ぶ前に飛ばした URL。
	skipped     []string
	geminiCalls int
}

// scheduleEmpty は、ページは読めたが日程表がないとき。表が出るまで待つものではなく、
// その URL には取り込むものがないので、翌日の残り見積もりには数えない。
func scheduleEmpty(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "競技日程の表が見つかりません") ||
		strings.Contains(msg, "競技年度が見つかりません")
}

// readMeets は1つの県の日程を読み取る。onlyURLs があればその URL だけを読み直す
// （部分成功の翌日の残り分）。空なら ResolveURLs で全 URL を読む。
func readMeets(ctx context.Context, client competitions.Client, source competitions.Source, onlyURLs []string) (readOutcome, error) {
	var out readOutcome
	override := len(onlyURLs) > 0
	urls := onlyURLs
	if !override {
		var err error
		urls, err = source.ResolveURLs(ctx)
		if err != nil {
			if source.Prefecture == "26" && strings.Contains(err.Error(), "403 Forbidden") {
				return out, fmt.Errorf("%w: 26 京都陸協が取得元へのアクセスを403で拒否。既存データを維持: %v", competitions.ErrSourceSkipped, err)
			}
			return out, err
		}
		if len(urls) == 0 {
			return out, errors.New("日程ページのURLがありません")
		}
	}
	// 続けて並んだ PDF はまとめて Gemini に送り、複数の PDF に載っている大会が1回だけ返るようにする。
	var pages []competitions.Page
	var pageURLs []string
	var firstErr error
	for _, u := range urls {
		page, err := competitions.FetchPage(ctx, u)
		if err != nil {
			// まだ公開されていない月があっても、同じ県のほかのページは止めない。
			if override || len(urls) > 1 {
				// ページは読めたが日程表がないものは、取り込むものがないので残りに数えない。
				// 読めなかったものは翌日読み直す。
				if scheduleEmpty(err) {
					log.Printf("%s: %s を飛ばします: %v", source.Prefecture, u, err)
					out.skipped = append(out.skipped, u)
				} else {
					log.Printf("%s: %s の読み取りに失敗しましたが、ほかの URL は続けます: %v", source.Prefecture, u, err)
					out.failed = append(out.failed, u)
					if firstErr == nil {
						firstErr = err
					}
				}
				continue
			}
			return out, err
		}
		if n := len(pages); n > 0 && len(page.PDFs) > 0 && len(pages[n-1].PDFs) > 0 {
			pages[n-1].PDFs = append(pages[n-1].PDFs, page.PDFs...)
			continue
		}
		pages = append(pages, page)
		pageURLs = append(pageURLs, u)
	}
	// 残りの読み直しでは1 URL でも切り離す。通常の単一 URL は従来どおり即失敗にする。
	multi := override || len(pageURLs) > 1
	var allMeets []competitions.Meet
	seen := map[string]bool{}
	for i, page := range pages {
		u := pageURLs[i]
		if i > 0 {
			// Tier 0 の Flash は5回/分。県内の複数ページ間にも県のあいだと同じ余裕を残す。
			if err := competitions.Wait(ctx, 15*time.Second); err != nil {
				return out, err
			}
		}
		drafts, err := client.Extract(ctx, page)
		out.geminiCalls++
		if err != nil {
			// 1つの URL の一時失敗（503 high demand など）で県全体を不成立にしない。
			if multi {
				log.Printf("%s: %s の読み取りに失敗しましたが、ほかの URL は続けます: %v", source.Prefecture, u, err)
				out.failed = append(out.failed, u)
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			return out, err
		}
		meets, err := competitions.BuildMeets(page, drafts, source.Prefecture, u)
		if err != nil {
			// 複数の URL のうち1つに大会がなくても、すべて失敗しないかぎりほかを続ける。
			if multi && strings.Contains(err.Error(), "大会がありません") {
				out.failed = append(out.failed, u)
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			return out, err
		}
		out.succeeded = append(out.succeeded, u)
		for _, meet := range meets {
			key := meet.Name + "|" + meet.StartsOn.Format("2006-01-02")
			if !seen[key] {
				seen[key] = true
				allMeets = append(allMeets, meet)
			}
		}
	}
	if len(allMeets) == 0 {
		// 読み直しで表のある URL が1つもなければ、失敗ではなく「今の期間は終わり」にする。
		if override && len(out.failed) == 0 {
			return out, fmt.Errorf("%w: 残りの URL に取り込める大会がありません。既存データを維持", competitions.ErrSourceSkipped)
		}
		if firstErr != nil {
			return out, firstErr
		}
		return out, fmt.Errorf("大会がありません")
	}
	sort.SliceStable(allMeets, func(i, j int) bool {
		return allMeets[i].StartsOn.Before(allMeets[j].StartsOn)
	})
	out.meets = allMeets
	return out, nil
}

// runDryRun は県のページを1つ読み、取り込まれる内容を表示する。
// DB には触らないので、新しい URL を登録する前に確かめられる。
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
	outcome, err := readMeets(ctx, competitions.Client{Key: key}, source, nil)
	if err != nil {
		return err
	}
	printMeets(os.Stdout, outcome.meets, source)
	if len(outcome.failed) > 0 || len(outcome.skipped) > 0 {
		fmt.Fprintf(os.Stdout, "note: %d urls failed, %d urls skipped before gemini\n", len(outcome.failed), len(outcome.skipped))
	}
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

// importOutcome は、1つの県の取り込み結果。
type importOutcome struct {
	// result は success・partial・skipped・transient・permanent のどれか。
	result string
	// failed は取り込めなかった読み取り単位のキー。翌日の残り見積もりと試行の記録になる。
	failed      []string
	geminiCalls int
}

// deleteBySeasonExceptURLs は、成功した URL 由来の行だけを消す。失敗した URL の既存データは残す。
const deleteBySeasonExceptURLs = `DELETE FROM competitions c
WHERE c.prefecture = $1
  AND c.season_year = $2
  AND c.source_url = ANY($3)
  AND (c.name || '|' || to_char(c.starts_on, 'YYYY-MM-DD')) <> ALL($4)`

func importSource(ctx context.Context, pool *pgxpool.Pool, client competitions.Client, source competitions.Source, retryURLs []string) (importOutcome, error) {
	var outcome importOutcome
	read, err := readMeets(ctx, client, source, retryURLs)
	outcome.geminiCalls = read.geminiCalls
	outcome.failed = read.failed
	if err != nil {
		outcome.result = classifyImportError(err)
		return outcome, err
	}
	meets := read.meets
	tx, err := pool.Begin(ctx)
	if err != nil {
		outcome.result = competitions.AttemptPermanent
		return outcome, err
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)
	season := int32(meets[0].SeasonYear)
	// 古いシーズンの大会を消すのは、いまのシーズンより前の分だけにする。ページに翌年度の表記があって
	// 翌シーズンと判定されても、いまのシーズンの大会まで消さないようにする。
	before := min(season, int32(competitions.SeasonYear(time.Now())))
	if err := q.DeleteCompetitionsBeforeSeason(ctx, db.DeleteCompetitionsBeforeSeasonParams{
		Prefecture: source.Prefecture,
		SeasonYear: before,
	}); err != nil {
		outcome.result = competitions.AttemptPermanent
		return outcome, err
	}
	// 日程表にまだある大会はその場で更新し、ニュースから見つけた要項が取り込み直しで消えないようにする。
	// 日程表から消えた大会だけを削除する。
	keys := make([]string, 0, len(meets))
	for _, meet := range meets {
		keys = append(keys, meet.Name+"|"+meet.StartsOn.Format("2006-01-02"))
	}
	// 読み直し（retryURLs）では成功した URL の分だけ消す。全 URL を読んで失敗も取り残しも
	// なければ、従来どおり日程表から消えた大会を消す。
	if len(retryURLs) == 0 && len(read.failed) == 0 && len(read.skipped) == 0 {
		if err := q.DeleteCompetitionsBySeasonExcept(ctx, db.DeleteCompetitionsBySeasonExceptParams{
			Prefecture: source.Prefecture,
			SeasonYear: season,
			Keys:       keys,
		}); err != nil {
			outcome.failed = nil
			outcome.result = competitions.AttemptPermanent
			return outcome, err
		}
	} else {
		// 一部失敗のときは、失敗した URL の既存データを残す。成功した URL の分だけ取り込む。
		if _, err := tx.Exec(ctx, deleteBySeasonExceptURLs, source.Prefecture, season, read.succeeded, keys); err != nil {
			outcome.result = competitions.AttemptPermanent
			return outcome, err
		}
	}
	if len(outcome.failed) == 0 {
		outcome.result = competitions.AttemptSuccess
	} else {
		outcome.result = competitions.AttemptPartial
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
			outcome.result = competitions.AttemptPermanent
			return outcome, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		outcome.result = competitions.AttemptPermanent
		return outcome, err
	}
	if outcome.result == competitions.AttemptPartial {
		fmt.Fprintf(os.Stdout, "imported %d competitions for prefecture %s season %d (partial: %d urls failed, %d urls skipped)\n",
			len(meets), source.Prefecture, meets[0].SeasonYear, len(read.failed), len(read.skipped))
	} else {
		fmt.Fprintf(os.Stdout, "imported %d competitions for prefecture %s season %d\n", len(meets), source.Prefecture, meets[0].SeasonYear)
	}
	return outcome, nil
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
