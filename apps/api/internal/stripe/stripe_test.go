package stripe

import (
	"encoding/hex"
	"fmt"
	"testing"
	"time"
)

func TestVerifyWebhook(t *testing.T) {
	payload := []byte(`{"id":"evt_1","type":"checkout.session.completed","data":{"object":{"id":"cs_1"}}}`)
	now := time.Unix(1_800_000_000, 0)
	sig := hex.EncodeToString(Sign(payload, "whsec_x", now.Unix()))
	header := fmt.Sprintf("t=%d,v1=deadbeef,v1=%s", now.Unix(), sig)

	ev, err := VerifyWebhook(payload, header, "whsec_x", now.Add(time.Minute))
	if err != nil || ev.Type != "checkout.session.completed" || ev.ID != "evt_1" {
		t.Fatalf("valid: %+v %v", ev, err)
	}
	if _, err := VerifyWebhook(payload, header, "whsec_other", now); err != ErrBadSignature {
		t.Fatalf("wrong secret: %v", err)
	}
	if _, err := VerifyWebhook(append(payload, ' '), header, "whsec_x", now); err != ErrBadSignature {
		t.Fatalf("tampered: %v", err)
	}
	if _, err := VerifyWebhook(payload, header, "whsec_x", now.Add(10*time.Minute)); err != ErrBadSignature {
		t.Fatalf("too old: %v", err)
	}
	if _, err := VerifyWebhook(payload, "", "whsec_x", now); err != ErrBadSignature {
		t.Fatalf("no header: %v", err)
	}
}
