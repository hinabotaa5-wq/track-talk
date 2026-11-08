package main

import (
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
	permanent            bool
	geminiCalls          int
	// 末尾サマリ用に県ごとの結果を残す。
	lines []string
}

func (r *importResults) record(err error) {
	r.recordSource("", err, 0)
}

func (r *importResults) recordOutcome(prefecture, outcome string, err error, geminiCalls int) {
	r.attempted++
	r.geminiCalls += geminiCalls
	label := outcome
	if err != nil {
		switch outcome {
		case competitions.AttemptSkipped:
			r.skipped++
		case competitions.AttemptTransient:
			r.transientFailed++
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

func (r *importResults) recordSource(prefecture string, err error, geminiCalls int) {
	if err == nil {
		r.recordOutcome(prefecture, competitions.AttemptSuccess, nil, geminiCalls)
		return
	}
	r.recordOutcome(prefecture, classifyImportError(err), err, geminiCalls)
}

// classifyImportError は、取り込み失敗を試行の記録に残す区分に分ける。
func classifyImportError(err error) string {
	switch {
	case errors.Is(err, competitions.ErrSourceSkipped):
		return competitions.AttemptSkipped
	case errors.Is(err, competitions.ErrTransient):
		return competitions.AttemptTransient
	default:
		return competitions.AttemptPermanent
	}
}

func (r importResults) failed() bool { return r.permanent || (r.attempted > 0 && r.succeeded == 0) }

// summary は、その日にどの県を試してどうなったかを Actions のログの末尾に一覧する。
func (r importResults) summary(targets int, limit int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "import summary: targets=%d success=%d partial=%d transient=%d skipped=%d gemini_calls~%d/%d",
		targets, r.succeeded, r.partial, r.transientFailed, r.skipped, r.geminiCalls, limit)
	if r.permanent {
		b.WriteString(" permanent_failure=yes")
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
