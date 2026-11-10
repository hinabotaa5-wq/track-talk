package competitions

import (
	"sort"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/jst"
)

// DailyRequestLimit は、1日1回の実行で使ってよい Gemini リクエスト数（論理数）の既定値。
// 無料枠の1日あたりの上限はプロジェクトごとで、AI Studio にしか出ないので、報告されている中でいちばん低い値（モデルごとに1日20回）より十分少なくする。
const DailyRequestLimit = 15

// MaxRealCallsPerDay は、リトライ込みの実 HTTP 呼び出し数の1日の上限。
// 無料枠の最低報告値（モデルごとに1日20回）を恒常的に超えないための枠で、
// 論理数の上限（DailyRequestLimit）とは別に実行時に数える。
const MaxRealCallsPerDay = 20

// ImportWindowStart は、年間の日程表を取り込む今の期間が始まった時刻。now より後ではない、いちばん新しい4月1日か10月1日（日本時間）。
// 県は春にその年の日程表を出し、秋に直す県もあるので、県ごとに期間ごとに1回だけ取り込み、あとはそのままにする。
func ImportWindowStart(now time.Time) time.Time {
	local := now.In(jst.Zone)
	year := local.Year()
	for _, month := range []time.Month{time.October, time.April} {
		if start := time.Date(year, month, 1, 0, 0, 0, 0, jst.Zone); !start.After(local) {
			return start
		}
	}
	return time.Date(year-1, time.October, 1, 0, 0, 0, 0, jst.Zone)
}

// InternationalRefreshAfter が短いのは、国際大会は Gemini を使わずに読むから。
const InternationalRefreshAfter = 7 * 24 * time.Hour

// Requests は、s の取り込みに使う Gemini のリクエスト数。リンクされた PDF はまとめて1回で送る。
func (s Source) Requests() int {
	if len(s.PDFLinks) > 0 {
		return 1 + len(s.ExtraURLs)
	}
	return len(s.URLs())
}

// 試行の結果。今の期間の記録だけを DueSourcesWithAttempts が見る。
// rate_limited は回数制限での打ち切りで、選定上は transient/permanent と同じ後回し。
// 旧 permanent に混ぜていたものを、読み手を誤らせないよう分けている。
const (
	AttemptSuccess     = "success"
	AttemptPartial     = "partial"
	AttemptTransient   = "transient"
	AttemptPermanent   = "permanent"
	AttemptSkipped     = "skipped"
	AttemptRateLimited = "rate_limited"
)

// Attempt は、今の期間のある県の取り込み試行の記録。失敗しても残るので、
// 翌日も同じ県が先頭に固定されて後続の県に予算が届かない、という詰まりを避けられる。
type Attempt struct {
	Outcome             string
	AttemptedAt         time.Time
	ConsecutiveFailures int
	FailedURLs          []string
	RequestsMade        int
}

// inWindow は、今の期間の試行だけを有効にする。期間が変われば試行の記録は仕切り直しになる。
func (a Attempt) inWindow(windowStart time.Time) bool {
	return !a.AttemptedAt.Before(windowStart)
}

// EffectiveCost は、その県の取り込みに使う Gemini のリクエスト数の見積もり。
// 直近の部分成功で残りの URL が分かっていれば、その分だけを見る。表がなくて Gemini を
// 呼ばなかった URL は、試行のたびに数え直さない（一部成功の残り見積もりにだけ反映する）。
func (s Source) EffectiveCost(attempt *Attempt, windowStart time.Time) int {
	base := s.Requests()
	if attempt == nil || !attempt.inWindow(windowStart) || attempt.Outcome != AttemptPartial {
		return base
	}
	if n := len(attempt.FailedURLs); n > 0 && n < base {
		return n
	}
	return base
}

// maxSingleSourceRequests は、1つの県が1日に使ってよい Gemini のリクエスト数の上限。
// 予算の過半を超える県（いまは福井の12回だけ）は後回しにして、後続の未試行の県を先に通す。
// 既存の小さな予算のテストの動きを変えないよう、8回が下限。
func maxSingleSourceRequests(limit int) int {
	if half := limit / 2; half > 8 {
		return half
	}
	return 8
}

// DueSources は、今の期間が始まってからまだ取り込んでいないソースを、古い順に、Gemini のリクエストが limit に収まるだけ返す。
// 収まらないソースは後日に回し、その日にはいちばん先に並ぶ。
func DueSources(sources []Source, updated map[string]time.Time, now time.Time, limit int) []Source {
	return DueSourcesWithAttempts(sources, updated, nil, now, limit)
}

// DueSourcesWithAttempts は、DueSources に今の期間の試行の記録を足したもの。
// 並びは「未試行→部分成功の残り→失敗の後回し」の順で、各段の中では DueSources と同じく古い順・コード順を保つ。
// 1県で予算の大半を使う県は、安い県を詰めたあとで1日1県だけ足す。期間判定の前提（4/1・10/1 ごとに1回）は変えない。
func DueSourcesWithAttempts(sources []Source, updated map[string]time.Time, attempts map[string]Attempt, now time.Time, limit int) []Source {
	windowStart := ImportWindowStart(now)
	type candidate struct {
		source Source
		tier   int
		cost   int
		// tier 内の並びのための鍵
		fresh  time.Time
		hasNew bool
		retry  time.Time
		fails  int
	}
	var cands []candidate
	for _, source := range sources {
		imported, fresh := freshTime(updated, source.Prefecture)
		att, ok := attempts[source.Prefecture]
		if !ok || !att.inWindow(windowStart) {
			ok = false
		}
		if fresh && !imported.Before(windowStart) {
			// 今の期間に取り込み済み。部分成功・失敗の記録がある県だけ、残りの分を取り直す。
			if !ok || (att.Outcome != AttemptPartial && att.Outcome != AttemptTransient && att.Outcome != AttemptPermanent && att.Outcome != AttemptRateLimited) {
				continue
			}
		} else {
			// まだ取り込んでいない。成功・スキップ済みは今の期間は終わり。
			if ok && (att.Outcome == AttemptSuccess || att.Outcome == AttemptSkipped) {
				continue
			}
		}
		cost := source.Requests()
		if ok {
			cost = source.EffectiveCost(&att, windowStart)
		}
		if cost == 0 {
			continue
		}
		c := candidate{source: source, cost: cost}
		switch {
		case !ok:
			c.tier = 0
			c.fresh = imported
			c.hasNew = fresh
		case att.Outcome == AttemptPartial:
			c.tier = 1
		default:
			c.tier = 2
			c.retry = att.AttemptedAt
			c.fails = att.ConsecutiveFailures
		}
		cands = append(cands, c)
	}
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].tier != cands[j].tier {
			return cands[i].tier < cands[j].tier
		}
		switch cands[i].tier {
		case 0:
			// DueSources と同じく、未取り込みを先に、取り込み済みは古い順に。どちらもコード順で公平に。
			if cands[i].hasNew != cands[j].hasNew {
				return !cands[i].hasNew
			}
			if cands[i].hasNew && !cands[i].fresh.Equal(cands[j].fresh) {
				return cands[i].fresh.Before(cands[j].fresh)
			}
			return cands[i].source.Prefecture < cands[j].source.Prefecture
		case 1:
			return cands[i].source.Prefecture < cands[j].source.Prefecture
		default:
			if cands[i].fails != cands[j].fails {
				return cands[i].fails < cands[j].fails
			}
			if !cands[i].retry.Equal(cands[j].retry) {
				return cands[i].retry.Before(cands[j].retry)
			}
			return cands[i].source.Prefecture < cands[j].source.Prefecture
		}
	})
	maxSingle := maxSingleSourceRequests(limit)
	var due []Source
	remaining := limit
	// 安い県から詰めて、後続の未試行の県に予算を届ける。
	for _, c := range cands {
		if c.cost > maxSingle {
			continue
		}
		if c.cost > remaining {
			continue
		}
		due = append(due, c.source)
		remaining -= c.cost
	}
	// 高い県は1日1県まで。残りの予算に収まるいちばん優先度の高い県だけ足す。
	for _, c := range cands {
		if c.cost <= maxSingle || c.cost > remaining {
			continue
		}
		due = append(due, c.source)
		break
	}
	return due
}

func freshTime(updated map[string]time.Time, prefecture string) (time.Time, bool) {
	imported, ok := updated[prefecture]
	if !ok || imported.IsZero() {
		return time.Time{}, false
	}
	return imported, true
}
