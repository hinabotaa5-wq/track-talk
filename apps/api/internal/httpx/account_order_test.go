package httpx

import (
	"errors"
	"testing"
)

func TestFinishWithdrawSkipsAuthWhenCommitFails(t *testing.T) {
	called := false
	deleted, err := finishWithdraw(func() error {
		return errors.New("commit")
	}, func() error {
		called = true
		return nil
	})
	if err == nil || called || deleted {
		t.Fatalf("deleted=%v called=%v err=%v", deleted, called, err)
	}
	if errors.Is(err, errAuthDelete) {
		t.Fatal("commit failure reported as auth failure")
	}
}

func TestFinishWithdrawKeepsCommitWhenAuthDeleteFails(t *testing.T) {
	deleted, err := finishWithdraw(func() error {
		return nil
	}, func() error {
		return errors.New("supabase")
	})
	if deleted || !errors.Is(err, errAuthDelete) {
		t.Fatalf("deleted=%v err=%v", deleted, err)
	}
}
