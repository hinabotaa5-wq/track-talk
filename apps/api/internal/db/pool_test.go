package db

import (
	"context"
	"testing"
)

func TestNewPoolSetsApplicationName(t *testing.T) {
	pool, err := NewPool(context.Background(), "postgres://tracktalk:tracktalk@localhost:5432/tracktalk?sslmode=disable", "track-talk-api")
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if got := pool.Config().ConnConfig.RuntimeParams["application_name"]; got != "track-talk-api" {
		t.Fatalf("application_name = %q, want track-talk-api", got)
	}
}

func TestNewPoolRejectsBadURL(t *testing.T) {
	if _, err := NewPool(context.Background(), "://bad", "track-talk-api"); err == nil {
		t.Fatal("expected error for invalid DATABASE_URL")
	}
}
