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

func TestKyotoForbiddenSkippedBeforeGemini(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusForbidden) }))
	defer srv.Close()
	_, err := readMeets(context.Background(), competitions.Client{}, competitions.Source{Prefecture: "26", PageURL: srv.URL, PDFLinks: []string{"競技会日程"}})
	if !errors.Is(err, competitions.ErrSourceSkipped) {
		t.Fatalf("%v", err)
	}
}
