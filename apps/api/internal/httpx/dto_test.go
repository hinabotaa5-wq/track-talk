package httpx

import (
	"strings"
	"testing"

	"github.com/hinata/track-talk/apps/api/internal/db"
)

func TestTextFeedExcerptWithAndWithoutPhoto(t *testing.T) {
	photo := "photos/runner"
	for _, publicID := range []*string{nil, &photo} {
		row := db.ListPostsNewRow{
			Kind: "text", Body: "今日の練習\n" + strings.Repeat("あ", 200),
			CloudinaryPublicID: publicID,
		}
		post := postFromList(row, false, true)
		if post.Excerpt != postExcerpt(row.Body) || post.Body != "" {
			t.Fatalf("photo=%v: excerpt=%q body=%q", publicID, post.Excerpt, post.Body)
		}
		if len([]rune(post.Excerpt)) > postExcerptRunes {
			t.Fatalf("excerpt is too long: %q", post.Excerpt)
		}
		if publicID != nil && post.ImagePublicID != photo {
			t.Fatalf("photo was lost: %q", post.ImagePublicID)
		}
	}
}
