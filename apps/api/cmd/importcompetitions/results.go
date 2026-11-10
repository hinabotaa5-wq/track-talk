package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hinata/track-talk/apps/api/internal/competitions"
)

// スキップは成功件数に含めない。対象があるのに成功ゼロなら失敗。
type importResults struct {
	attempted, succeeded int
	partial              int
	skipped              int
	transientFailed      int
	rateLimited          int
	permanent            bool
	// geminiCalls は論理呼び出し数、httpCalls はリトライ込みの実呼び出し数。
	geminiCalls int
	httpCalls   int
	// 末尾サマリ用に県ごとの結果を残す。
	lines []string
}

func (r *importResults) record(err error) {
	r.recordSource("", err, 0, 0)
}

func (r *importResults) recordOutcome(prefecture, outcome string, err error, geminiCalls, httpCalls int) {
	r.attempted++
	r.geminiCalls += geminiCalls
	r.httpCalls += httpCalls
	label := outcome
	if err != nil {
		switch outcome {
		case competitions.AttemptSkipped:
			r.skipped++
		case competitions.AttemptTransient:
			r.transientFailed++
		case competitions.AttemptRateLimited:
			r.rateLimited++
		default:
			r.permanent = true
		}
		if label == "" {
			label = "failed"
		}
	} else {
		r.succeeded++
		if outcome == competitions.AttemptPartial {
			r.partial++
		}
		if label == "" {
			label = competitions.AttemptSuccess
		}
	}
	if prefecture != "" {
		detail := label
		if err != nil {
			detail += ": " + oneLine(err)
		}
		r.lines = append(r.lines, prefecture+": "+detail)
	}
}

func (r *importResults) recordSource(prefecture string, err error, geminiCalls, httpCalls int) {
	if err == nil {
		r.recordOutcome(prefecture, competitions.AttemptSuccess, nil, geminiCalls, httpCalls)
		return
	}
	r.recordOutcome(prefecture, classifyImportError(err), err, geminiCalls, httpCalls)
}

// classifyImportError は、取り込み失敗を試行の記録に残す区分に分ける。
// 回数制限は旧 permanent ではなく rate_limited に分け、選定上は
// transient/permanent と同じ後回しになる。期限切れは翌日に後回しできるよう
// transient に倒す。どちらも試行として記録し、未試行のまま先頭に戻さない。
func classifyImportError(err error) string {
	switch {
	case errors.Is(err, competitions.ErrSourceSkipped):
		return competitions.AttemptSkipped
	case errors.Is(err, competitions.ErrRateLimited):
		return competitions.AttemptRateLimited
	case errors.Is(err, competitions.ErrTransient):
		return competitions.AttemptTransient
	case errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled):
		return competitions.AttemptTransient
	case strings.Contains(err.Error(), "403"):
		// 協会サイトの bot 対策などの一時的拒否。skipped ではなく翌日に回す。
		return competitions.AttemptTransient
	default:
		return competitions.AttemptPermanent
	}
}

func (r importResults) failed() bool {
	return r.permanent || r.rateLimited > 0 || (r.attempted > 0 && r.succeeded == 0)
}

// summary は、その日にどの県を試してどうなったかを Actions のログの末尾に一覧する。
// gemini_calls は論理数、http_calls はリトライ込みの実数で、両方を出す。
func (r importResults) summary(targets int, limit int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "import summary: targets=%d success=%d partial=%d transient=%d skipped=%d rate_limited=%d gemini_calls=%d/%d http_calls=%d",
		targets, r.succeeded, r.partial, r.transientFailed, r.skipped, r.rateLimited, r.geminiCalls, limit, r.httpCalls)
	if r.permanent {
		b.WriteString(" permanent_failure=yes")
	}
	if r.rateLimited > 0 {
		b.WriteString(" rate_limited=yes")
	}
	for _, line := range r.lines {
		b.WriteString("\n  " + line)
	}
	return b.String()
}

func oneLine(err error) string {
	text := strings.Join(strings.Fields(err.Error()), " ")
	if len(text) > 160 {
		return text[:160]
	}
	return text
}
