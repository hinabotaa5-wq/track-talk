package main

import (
	"context"
	"errors"
	"github.com/hinata/track-talk/apps/api/internal/competitions"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestImportResults(t *testing.T) {
	for _, tc := range []struct {
		name string
		errs []error
		fail bool
	}{
		{"none", nil, false},
		{"partial transient", []error{nil, competitions.ErrTransient}, false},
		{"all transient", []error{competitions.ErrTransient}, true},
		{"skipped with success", []error{nil, competitions.ErrSourceSkipped}, false},
		{"all skipped", []error{competitions.ErrSourceSkipped}, true},
		{"auth", []error{nil, competitions.ErrAuthentication}, true},
		{"rate limit", []error{nil, competitions.ErrRateLimited}, true},
		{"db error", []error{nil, errors.New("db")}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var r importResults
			for _, err := range tc.errs {
				r.record(err)
			}
			if r.failed() != tc.fail {
				t.Fatalf("%+v", r)
			}
		})
	}
}

// Issue #515: 取得元の 403 は skipped ではなく transient に倒し、翌日に回す。
// skipped だと次の 4/1 か 10/1 まで対象に戻らないため。
func TestKyotoForbiddenRetriedAsTransient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusForbidden) }))
	defer srv.Close()
	_, err := readMeets(context.Background(), competitions.Client{}, competitions.Source{Prefecture: "26", PageURL: srv.URL, PDFLinks: []string{"競技会日程"}}, nil)
	if !errors.Is(err, competitions.ErrTransient) {
		t.Fatalf("%v", err)
	}
	if got := classifyImportError(err); got != competitions.AttemptTransient {
		t.Fatalf("classify %q", got)
	}
}

func TestClassifyFailuresForSelection(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"rate limit", competitions.ErrRateLimited, competitions.AttemptRateLimited},
		{"transient", competitions.ErrTransient, competitions.AttemptTransient},
		{"skipped", competitions.ErrSourceSkipped, competitions.AttemptSkipped},
		{"deadline", context.DeadlineExceeded, competitions.AttemptTransient},
		{"canceled", context.Canceled, competitions.AttemptTransient},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyImportError(tc.err); got != tc.want {
				t.Fatalf("classify %v = %q, want %q", tc.err, got, tc.want)
			}
		})
	}
	if got := classifyImportError(errors.New("page https://example.com: 403 Forbidden")); got != competitions.AttemptTransient {
		t.Fatalf("403 page = %q", got)
	}
	if got := classifyImportError(errors.New("db down")); got != competitions.AttemptPermanent {
		t.Fatalf("db = %q", got)
	}
}
