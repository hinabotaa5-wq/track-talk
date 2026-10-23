package httpx

import (
	"context"
	"errors"
	"github.com/hinata/track-talk/apps/api/internal/cloudinary"
	"slices"
	"testing"
	"time"
)

func TestUnusedKeepsIDsStillInUse(t *testing.T) {
	got := unused([]string{"a", "b", "c"}, []string{"b"})
	if !slices.Equal(got, []string{"a", "c"}) {
		t.Fatalf("got %v", got)
	}
	if got := unused([]string{"a"}, []string{"a"}); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestRetryMediaCleanupRecovers(t *testing.T) {
	calls := 0
	err := retryMediaCleanup(context.Background(), time.Millisecond, func(context.Context) error {
		calls++
		if calls < 3 {
			return cloudinary.ErrUnavailable
		}
		return nil
	})
	if err != nil || calls != 3 {
		t.Fatalf("error=%v calls=%d", err, calls)
	}
}

func TestRetryMediaCleanupStopsAtLimit(t *testing.T) {
	calls := 0
	err := retryMediaCleanup(context.Background(), time.Millisecond, func(context.Context) error {
		calls++
		return cloudinary.ErrUnavailable
	})
	if !errors.Is(err, cloudinary.ErrUnavailable) || calls != mediaCleanupAttempts {
		t.Fatalf("error=%v calls=%d", err, calls)
	}
}

func TestRetryMediaCleanupDoesNotRepeatPermanentErrors(t *testing.T) {
	for _, failure := range []error{cloudinary.ErrInvalid, cloudinary.ErrNotConfigured} {
		calls := 0
		err := retryMediaCleanup(context.Background(), time.Millisecond, func(context.Context) error { calls++; return failure })
		if !errors.Is(err, failure) || calls != 1 {
			t.Fatalf("error=%v calls=%d", err, calls)
		}
	}
}

func TestRetryMediaCleanupCancellationStopsWaiting(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	timer := time.AfterFunc(10*time.Millisecond, cancel)
	defer timer.Stop()
	err := retryMediaCleanup(ctx, time.Hour, func(context.Context) error { calls++; return cloudinary.ErrUnavailable })
	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("error=%v calls=%d", err, calls)
	}
}

func TestRetryMediaCleanupDoesNotStartWhenCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	err := retryMediaCleanup(ctx, time.Millisecond, func(context.Context) error { calls++; return nil })
	if !errors.Is(err, context.Canceled) || calls != 0 {
		t.Fatalf("error=%v calls=%d", err, calls)
	}
}
