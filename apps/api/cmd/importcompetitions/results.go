package main

import (
	"errors"
	"github.com/hinata/track-talk/apps/api/internal/competitions"
)

// スキップは成功件数に含めない。対象があるのに成功ゼロなら失敗。
type importResults struct {
	attempted, succeeded int
	permanent            bool
}

func (r *importResults) record(err error) {
	r.attempted++
	if err == nil {
		r.succeeded++
		return
	}
	if !errors.Is(err, competitions.ErrTransient) && !errors.Is(err, competitions.ErrSourceSkipped) {
		r.permanent = true
	}
}
func (r importResults) failed() bool { return r.permanent || (r.attempted > 0 && r.succeeded == 0) }
