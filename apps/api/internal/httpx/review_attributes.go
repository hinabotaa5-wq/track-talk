package httpx

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// レビューを書いた人の属性(どれも任意)。商品の一覧とレビューの絞り込みに使う。
// 足のサイズは mm で持つが、画面には 1cm 幅(「25.0〜25.5cm」)でしか出さない。

var reviewLevels = []struct{ Code, Label string }{
	{"junior-high", "中学"},
	{"high-school", "高校"},
	{"university", "大学"},
	{"adult", "社会人"},
	{"masters", "マスターズ"},
}

var reviewFootWidths = []struct{ Code, Label string }{
	{"narrow", "細め"},
	{"regular", "ふつう"},
	{"wide", "広め"},
}

var reviewUsages = []struct{ Code, Label string }{
	{"practice", "練習用"},
	{"race", "試合用"},
	{"both", "練習・試合"},
}

const (
	minFootSize = 220
	maxFootSize = 300
)

func reviewOptionLabel(options []struct{ Code, Label string }, code string) (string, bool) {
	for _, item := range options {
		if item.Code == code {
			return item.Label, true
		}
	}
	return "", false
}

// reviewAttrs are the optional attributes of a review, as stored.
type reviewAttrs struct {
	Event     *string
	Level     *string
	FootSize  *int16
	FootWidth *string
	Usage     *string
}

type reviewAttrsReq struct {
	Event     *string  `json:"event"`
	Level     *string  `json:"level"`
	FootSize  *float64 `json:"foot_size"`
	FootWidth *string  `json:"foot_width"`
	Usage     *string  `json:"usage"`
}

func optionalCode(raw *string) *string {
	if raw == nil {
		return nil
	}
	v := strings.TrimSpace(*raw)
	if v == "" {
		return nil
	}
	return &v
}

// normalizeReviewAttrs checks the optional attributes; an empty value means "not given".
func normalizeReviewAttrs(req reviewAttrsReq, fields map[string]string) reviewAttrs {
	var out reviewAttrs
	if v := optionalCode(req.Event); v != nil {
		if validPracticeEvent(*v) {
			out.Event = v
		} else {
			fields["event"] = "種目を選び直してください"
		}
	}
	if v := optionalCode(req.Level); v != nil {
		if _, ok := reviewOptionLabel(reviewLevels, *v); ok {
			out.Level = v
		} else {
			fields["level"] = "レベルを選び直してください"
		}
	}
	if req.FootSize != nil {
		mm := int(*req.FootSize*10 + 0.5)
		if mm < minFootSize || mm > maxFootSize || mm%5 != 0 {
			fields["foot_size"] = "足のサイズは22.0〜30.0cmの0.5cm刻みで選んでください"
		} else {
			v := int16(mm)
			out.FootSize = &v
		}
	}
	if v := optionalCode(req.FootWidth); v != nil {
		if _, ok := reviewOptionLabel(reviewFootWidths, *v); ok {
			out.FootWidth = v
		} else {
			fields["foot_width"] = "足幅を選び直してください"
		}
	}
	if v := optionalCode(req.Usage); v != nil {
		if _, ok := reviewOptionLabel(reviewUsages, *v); ok {
			out.Usage = v
		} else {
			fields["usage"] = "使い方を選び直してください"
		}
	}
	return out
}

// reviewFilter narrows products and reviews by the reviewers' attributes.
type reviewFilter struct {
	Event       *string
	Level       *string
	FootSizeMin *int16
	FootSizeMax *int16
	FootWidth   *string
	Usage       *string
}

func (f reviewFilter) active() bool {
	return f.Event != nil || f.Level != nil || f.FootSizeMin != nil || f.FootSizeMax != nil || f.FootWidth != nil || f.Usage != nil
}

func parseFootSizeParam(raw string) (*int16, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, true
	}
	cm, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, false
	}
	mm := int(cm*10 + 0.5)
	if mm < minFootSize || mm > maxFootSize {
		return nil, false
	}
	v := int16(mm)
	return &v, true
}

// parseReviewFilter reads the filter from the query; an unknown value is a field error.
func parseReviewFilter(q url.Values, fields map[string]string) reviewFilter {
	attrs := normalizeReviewAttrs(reviewAttrsReq{
		Event:     ptrIfSet(q, "event"),
		Level:     ptrIfSet(q, "level"),
		FootWidth: ptrIfSet(q, "foot_width"),
		Usage:     ptrIfSet(q, "usage"),
	}, fields)
	out := reviewFilter{Event: attrs.Event, Level: attrs.Level, FootWidth: attrs.FootWidth, Usage: attrs.Usage}
	var ok bool
	if out.FootSizeMin, ok = parseFootSizeParam(q.Get("foot_size_min")); !ok {
		fields["foot_size_min"] = "足のサイズを選び直してください"
	}
	if out.FootSizeMax, ok = parseFootSizeParam(q.Get("foot_size_max")); !ok {
		fields["foot_size_max"] = "足のサイズを選び直してください"
	}
	return out
}

func ptrIfSet(q url.Values, key string) *string {
	if !q.Has(key) {
		return nil
	}
	v := q.Get(key)
	return &v
}

// footSizeRangeLabel shows a size as the 1cm band it falls in, not the exact size.
func footSizeRangeLabel(mm int16) string {
	low := int(mm) / 10
	return fmt.Sprintf("%d.0〜%d.5cm", low, low)
}

// reviewAttrsJSON adds the attributes, with labels, to a review's JSON.
func reviewAttrsJSON(out map[string]any, a reviewAttrs) {
	var eventLabel, levelLabel, sizeLabel, widthLabel, usageLabel *string
	if a.Event != nil {
		v := practiceLabel(*a.Event, "その他")
		eventLabel = &v
	}
	if a.Level != nil {
		if v, ok := reviewOptionLabel(reviewLevels, *a.Level); ok {
			levelLabel = &v
		}
	}
	if a.FootSize != nil {
		v := footSizeRangeLabel(*a.FootSize)
		sizeLabel = &v
	}
	if a.FootWidth != nil {
		if v, ok := reviewOptionLabel(reviewFootWidths, *a.FootWidth); ok {
			widthLabel = &v
		}
	}
	if a.Usage != nil {
		if v, ok := reviewOptionLabel(reviewUsages, *a.Usage); ok {
			usageLabel = &v
		}
	}
	out["event"] = a.Event
	out["event_label"] = eventLabel
	out["level"] = a.Level
	out["level_label"] = levelLabel
	out["foot_size_label"] = sizeLabel
	out["foot_width"] = a.FootWidth
	out["foot_width_label"] = widthLabel
	out["usage"] = a.Usage
	out["usage_label"] = usageLabel
}
