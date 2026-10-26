package httpx_test

import (
	"net/http"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

func TestSenseSummaryBudgetCountsFailuresAndSkipsInvalidRequests(t *testing.T) {
	ai := &fakeGemini{reply: "invalid JSON"}
	endpoint := ai.serve(t)
	h := setupConfig(t, func(c *config.Config) {
		c.SenseSummaryEnabled = true
		c.GeminiAPIKey = "test"
		c.GeminiEndpoint = endpoint
		c.SenseSummaryDailyLimit = 2
	})
	me := token(t, uuid.New())
	if rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes/summary", me, nil); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("empty request: %d %s", rec.Code, rec.Body.String())
	}
	writeSenseNotes(t, h, me, 0, 6)
	for i := 0; i < 2; i++ {
		if rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes/summary", me, nil); rec.Code != http.StatusBadGateway {
			t.Fatalf("failure %d: %d %s", i, rec.Code, rec.Body.String())
		}
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes/summary", me, nil); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("exhausted: %d", rec.Code)
	}
	ai.mu.Lock()
	defer ai.mu.Unlock()
	if len(ai.prompts) != 2 {
		t.Fatalf("calls=%d", len(ai.prompts))
	}
}

func TestSenseSummaryBudgetMinuteAndRollingDay(t *testing.T) {
	ai := &fakeGemini{reply: `{"overview":"ok"}`}
	endpoint := ai.serve(t)
	h := setupConfig(t, func(c *config.Config) {
		c.SenseSummaryEnabled = true
		c.GeminiAPIKey = "test"
		c.GeminiEndpoint = endpoint
		c.SenseSummaryDailyLimit = 2
		c.SenseSummaryMinuteLimit = 1
	})
	me := token(t, uuid.New())
	writeSenseNotes(t, h, me, 0, 18)
	call := func(want int) {
		t.Helper()
		rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes/summary", me, nil)
		if rec.Code != want {
			t.Fatalf("want %d got %d %s", want, rec.Code, rec.Body.String())
		}
	}
	call(http.StatusCreated)
	call(http.StatusServiceUnavailable) // 1分の枠を超える
	execSQLArgs(t, `UPDATE sense_summary_attempts SET attempted_at=clock_timestamp()-interval '2 minutes'`)
	call(http.StatusCreated)
	execSQLArgs(t, `UPDATE sense_summary_attempts SET attempted_at=clock_timestamp()-interval '23 hours'`)
	call(http.StatusServiceUnavailable) // 日付をまたいでいても24時間以内は数える
	execSQLArgs(t, `UPDATE sense_summary_attempts SET attempted_at=clock_timestamp()-interval '25 hours'`)
	call(http.StatusCreated)
}

func TestSenseSummaryBudgetSerializesAcrossServers(t *testing.T) {
	ai := &fakeGemini{reply: `{"overview":"ok"}`}
	endpoint := ai.serve(t)
	configure := func(c *config.Config) {
		c.SenseSummaryEnabled = true
		c.GeminiAPIKey = "test"
		c.GeminiEndpoint = endpoint
		c.SenseSummaryDailyLimit = 2
	}
	first, second := setupConfig(t, configure), setupConfig(t, configure)
	var tokens []string
	for i := 0; i < 6; i++ {
		tok := token(t, uuid.New())
		writeSenseNotes(t, first, tok, 0, 6)
		tokens = append(tokens, tok)
	}
	codes := make(chan int, len(tokens))
	var wg sync.WaitGroup
	for i, tok := range tokens {
		h := first
		if i%2 == 1 {
			h = second
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes <- doJSON(t, h, http.MethodPost, "/v1/me/sense-notes/summary", tok, nil).Code
		}()
	}
	wg.Wait()
	close(codes)
	created := 0
	for code := range codes {
		if code == http.StatusCreated {
			created++
		} else if code != http.StatusServiceUnavailable {
			t.Errorf("unexpected %d", code)
		}
	}
	ai.mu.Lock()
	defer ai.mu.Unlock()
	if created != 2 || len(ai.prompts) != 2 {
		t.Fatalf("created=%d calls=%d", created, len(ai.prompts))
	}
}
