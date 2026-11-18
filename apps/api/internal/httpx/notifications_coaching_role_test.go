package httpx

import (
	"testing"

	"github.com/google/uuid"
)

// D13: path boundaries must not turn unrelated or administrative messages into coaching notices.
func TestCoachingNotificationDestination(t *testing.T) {
	id := uuid.New()
	for _, tc := range []struct {
		url, role string
		contract  uuid.UUID
	}{
		{"/coach", "coach", uuid.Nil},
		{"/coach/apply", "coach", uuid.Nil},
		{"/coach/contracts", "coach", uuid.Nil},
		{"/coaching/support-consults/" + id.String(), "player", uuid.Nil},
		{"/coaching/requests/" + id.String(), "player", uuid.Nil},
		{"/me/coaching", "player", uuid.Nil},
		{"/me/coaching/tickets", "player", uuid.Nil},
		{"/coaching/personal/" + id.String(), "", id},
		{"/coaching/personal/not-a-uuid", "", uuid.Nil},
		{"/coaching/personal/" + id.String() + "/extra", "", uuid.Nil},
		{"/admin", "", uuid.Nil},
		{"/admin/coaching", "", uuid.Nil},
		{"/coach-other", "", uuid.Nil},
		{"/me/coaching-other", "", uuid.Nil},
		{"/me", "", uuid.Nil},
		{"/", "", uuid.Nil},
	} {
		t.Run(tc.url, func(t *testing.T) {
			role, contract := coachingNotificationDestination(tc.url)
			if role != tc.role || contract != tc.contract {
				t.Fatalf("got (%q, %v), want (%q, %v)", role, contract, tc.role, tc.contract)
			}
		})
	}
}
