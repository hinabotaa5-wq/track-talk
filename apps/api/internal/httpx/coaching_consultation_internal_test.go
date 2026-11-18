package httpx

import (
	"testing"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/db"
)

func TestConsultationDeliveryIndependentOfDeadlineGeneration(t *testing.T) {
	delivered := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	for _, window := range []time.Duration{48 * time.Hour, 24 * time.Hour} {
		for _, funding := range []string{"cash", "ticket"} {
			for _, elapsed := range []time.Duration{time.Hour, window + time.Minute} {
				respond, closed := delivered.Add(window), delivered.Add(elapsed)
				c := db.CoachingConsultation{Status: "expired", Funding: funding, RespondBy: &respond, ClosedAt: &closed}
				if funding == "cash" {
					c.PaidAt = &delivered
				}
				before := c
				// The one-hour case is early expiry after coach suspension, before the response deadline.
				oldResult := closed.After(respond.Add(-window))
				if got := reviewConsultationAttempted(c); !got || got != oldResult || !consultationReachedCoach(c) {
					t.Errorf("window=%v funding=%s elapsed=%v: attempt=%v old=%v", window, funding, elapsed, got, oldResult)
				}
				if c != before {
					t.Fatal("saved consultation changed")
				}
			}
		}
		respond, closed := delivered.Add(window), delivered
		late := db.CoachingConsultation{Status: "expired", Funding: "cash", RespondBy: &respond, ClosedAt: &closed, PaidAt: &delivered}
		if reviewConsultationAttempted(late) || consultationReachedCoach(late) {
			t.Errorf("late payment counted or visible with window %v", window)
		}
	}
	for _, c := range []db.CoachingConsultation{
		{Status: "expired", Funding: "cash"},
		{Status: "cancelled", Funding: "cash"},
		{Status: "draft", Funding: "ticket"},
		{Status: "awaiting_payment", Funding: "cash"},
	} {
		if reviewConsultationAttempted(c) {
			t.Errorf("unsent consultation counted: %+v", c)
		}
	}
}

func TestConsultationWorkOrderTwelveHoursAndSavedDeadline(t *testing.T) {
	now := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	c := db.CoachingConsultation{PriceYen: 500, SharePercent: 70}
	if got := consultationWorkOrderTerms(c, now).DeliverBy; !got.Equal(now.Add(12 * time.Hour)) {
		t.Fatalf("preview deadline %v", got)
	}
	legacy := now.Add(72 * time.Hour)
	c.AnswerBy = &legacy
	if got := consultationWorkOrderTerms(c, now).DeliverBy; !got.Equal(legacy) {
		t.Fatalf("saved legacy deadline changed: %v", got)
	}
}
