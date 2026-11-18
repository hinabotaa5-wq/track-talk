package httpx

import (
	"context"
	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/migrate"
	"github.com/hinata/track-talk/apps/api/internal/push"
	"github.com/jackc/pgx/v5"
	"testing"
	"time"

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

// D13: test the stored values through the real common creation routes, including contract lookup.
func TestCoachingNotificationDatabaseRoles(t *testing.T) {
	ctx := context.Background()
	dsn := testDBDSN(t)
	if err := config.RequireLocalDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Skipf("database unavailable: %v", err)
	}
	defer conn.Close(ctx)
	if err = migrate.Up(dsn); err != nil {
		t.Fatal(err)
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`SET LOCAL session_replication_role=replica`)
	player, coach, contract, review := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	exec(`INSERT INTO profiles(id,nickname) VALUES($1,'player'),($2,'coach')`, player, coach)
	exec(`INSERT INTO coaching_contracts(id,plan_id,coach_id,requester_id,plan_kind,plan_title,duration_days,price_yen,event,goal,current_record,message,minor,respond_by,status)
 VALUES($1,gen_random_uuid(),$2,$3,'period','plan',30,10000,'100m','goal','12','hello',false,now()+interval '1 day','applied')`, contract, coach, player)
	s := &Server{q: db.New(tx)}
	for _, tc := range []struct {
		to        uuid.UUID
		url, role string
	}{
		{coach, "/coach", "coach"}, {player, "/coaching/support-consults/" + uuid.NewString(), "player"},
		{player, "/coaching/personal/" + contract.String(), "player"}, {coach, "/coaching/personal/" + contract.String(), "coach"},
		{player, "/admin", ""}, {player, "/coaching/personal/" + uuid.NewString(), ""},
	} {
		title := uuid.NewString()
		s.notify(ctx, tc.to, notifyCoaching, push.Message{Title: title, URL: tc.url}, time.Minute)
		var got string
		if err := tx.QueryRow(ctx, `SELECT COALESCE(coaching_role,'') FROM notifications WHERE title=$1`, title).Scan(&got); err != nil || got != tc.role {
			t.Fatalf("%s role=%q want %q: %v", tc.url, got, tc.role, err)
		}
	}
	for _, tc := range []struct {
		kind, role string
		to         uuid.UUID
	}{{"review_returned", "player", player}, {"review_offer", "coach", coach}} {
		id := uuid.New()
		if err := s.q.InsertReviewNotification(ctx, db.InsertReviewNotificationParams{ID: id, RecipientID: tc.to, Kind: tc.kind, ReviewRequestID: &review}); err != nil {
			t.Fatal(err)
		}
		var got string
		if err := tx.QueryRow(ctx, `SELECT coaching_role FROM notifications WHERE id=$1`, id).Scan(&got); err != nil || got != tc.role {
			t.Fatalf("review role %q %v", got, err)
		}
	}
}
