package httpx

import (
	"testing"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
)

func TestIsPostMaster(t *testing.T) {
	name := "ひなた"
	user := uuid.New()
	cases := []struct {
		label string
		row   db.GetPostRow
		want  bool
	}{
		{"30件でバッジ", db.GetPostRow{AuthorID: user, AuthorName: &name, AuthorLikedPostCount: 30}, true},
		{"29件ではまだ", db.GetPostRow{AuthorID: user, AuthorName: &name, AuthorLikedPostCount: 29}, false},
		{"ニックネーム非公開には出さない", db.GetPostRow{AuthorID: user, AuthorLikedPostCount: 100}, false},
		{"運営には出さない", db.GetPostRow{AuthorID: SystemProfileID, AuthorName: &name, AuthorLikedPostCount: 100}, false},
	}
	for _, c := range cases {
		if got := isPostMaster(c.row); got != c.want {
			t.Errorf("%s: got %v, want %v", c.label, got, c.want)
		}
	}
}
