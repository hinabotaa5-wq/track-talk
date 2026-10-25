package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const target = "projects/p/locations/asia-northeast1/services/track-talk-api"

func pushBody(t *testing.T, cost, budget float64) io.Reader {
	t.Helper()
	data, _ := json.Marshal(map[string]any{"costAmount": cost, "budgetAmount": budget, "currencyCode": "JPY"})
	body, _ := json.Marshal(map[string]any{
		"message":      map[string]any{"data": base64.StdEncoding.EncodeToString(data)},
		"subscription": "projects/p/subscriptions/budget-stop",
	})
	return strings.NewReader(string(body))
}

type runAPI struct {
	calls  int
	method string
	path   string
	query  string
	auth   string
	body   map[string]string
	status int
}

func newRunAPI(t *testing.T, api *runAPI) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api.calls++
		api.method = r.Method
		api.path = r.URL.Path
		api.query = r.URL.RawQuery
		api.auth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&api.body)
		if api.status != 0 {
			w.WriteHeader(api.status)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newStopper(srv *httptest.Server) stopper {
	return stopper{
		service:   target,
		authorize: func(*http.Request) error { return nil },
		runAPI:    srv.URL + "/v2/",
		token:     func(context.Context) (string, error) { return "tok", nil },
		client:    srv.Client(),
	}
}

func TestWithinBudgetDoesNothing(t *testing.T) {
	api := &runAPI{}
	s := newStopper(newRunAPI(t, api))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", pushBody(t, 99, 100)))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
	if api.calls != 0 {
		t.Fatalf("cloud run api called %d times", api.calls)
	}
}

func TestOverBudgetClosesIngress(t *testing.T) {
	api := &runAPI{}
	s := newStopper(newRunAPI(t, api))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", pushBody(t, 100, 100)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if api.method != http.MethodPatch || api.path != "/v2/"+target || api.query != "updateMask=ingress" {
		t.Fatalf("request = %s %s?%s", api.method, api.path, api.query)
	}
	if api.auth != "Bearer tok" {
		t.Fatalf("auth = %q", api.auth)
	}
	if api.body["ingress"] != internalOnly {
		t.Fatalf("body = %v", api.body)
	}
}

func TestFailureAsksForRetry(t *testing.T) {
	api := &runAPI{status: http.StatusForbidden}
	s := newStopper(newRunAPI(t, api))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", pushBody(t, 150, 100)))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}

	s.token = func(context.Context) (string, error) { return "", errors.New("no metadata") }
	rec = httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", pushBody(t, 150, 100)))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestMalformedNoticeIsDropped(t *testing.T) {
	api := &runAPI{}
	s := newStopper(newRunAPI(t, api))
	for _, body := range []string{"not json", `{"message":{"data":"bm90IGpzb24="}}`} {
		rec := httptest.NewRecorder()
		s.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))
		if rec.Code != http.StatusNoContent {
			t.Fatalf("%q: status = %d", body, rec.Code)
		}
	}
	if api.calls != 0 {
		t.Fatalf("cloud run api called %d times", api.calls)
	}
}

func TestUnauthenticatedNoticeCannotStopService(t *testing.T) {
	for _, configured := range []bool{false, true} {
		api := &runAPI{}
		s := newStopper(newRunAPI(t, api))
		s.authorize = nil
		if configured {
			s.authorize = func(*http.Request) error { return errors.New("invalid identity") }
		}
		rec := httptest.NewRecorder()
		s.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", pushBody(t, 100, 100)))
		if rec.Code != http.StatusUnauthorized || api.calls != 0 {
			t.Fatalf("status=%d cloud run calls=%d", rec.Code, api.calls)
		}
	}
}
