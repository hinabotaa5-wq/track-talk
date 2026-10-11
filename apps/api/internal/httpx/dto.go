package httpx

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
)

type meta struct {
	Page    int   `json:"page"`
	PerPage int   `json:"per_page"`
	Total   int64 `json:"total"`
}

type postDTO struct {
	ID             uuid.UUID `json:"id"`
	Title          string    `json:"title"`
	Body           string    `json:"body,omitempty"`
	BodyPreview    string    `json:"body_preview,omitempty"`
	authorID       uuid.UUID
	Mine           bool      `json:"mine"`
	LikeCount      int32     `json:"like_count"`
	CommentCount   int32     `json:"comment_count"`
	LikedByMe      bool      `json:"liked_by_me"`
	SavedByMe      bool      `json:"saved_by_me"`
	Kind           string    `json:"kind"`
	ThumbnailURL   string    `json:"thumbnail_url,omitempty"`
	YouTubeVideoID string    `json:"youtube_video_id,omitempty"`
	VideoPublicID  string    `json:"video_public_id,omitempty"`
	ImagePublicID  string    `json:"image_public_id,omitempty"`
	PlaybackURL    string    `json:"playback_url,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func applyKind(p *postDTO, kind string, youtubeID, thumbnail, publicID *string) {
	if kind == "" {
		kind = "text"
	}
	p.Kind = kind
	if youtubeID != nil {
		p.YouTubeVideoID = *youtubeID
	}
	if publicID != nil && kind == "video" {
		p.VideoPublicID = *publicID
	}
	if publicID != nil && kind == "text" {
		p.ImagePublicID = *publicID
	}
	if kind == "youtube" && thumbnail != nil {
		p.ThumbnailURL = *thumbnail
	}
}

func preview(body string) string {
	body = strings.TrimSpace(body)
	if utf8.RuneCountInString(body) <= 140 {
		return body
	}
	r := []rune(body)
	return string(r[:140]) + "…"
}

func postFromList(row db.ListPostsNewRow, liked bool, list bool) postDTO {
	p := postDTO{
		ID:           row.ID,
		Title:        row.Title,
		authorID:     row.AuthorID,
		LikeCount:    row.LikeCount,
		CommentCount: row.CommentCount,
		LikedByMe:    liked,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
	applyKind(&p, row.Kind, row.YoutubeVideoID, row.YoutubeThumbnailUrl, row.CloudinaryPublicID)
	if list {
		p.BodyPreview = preview(row.Body)
	} else {
		p.Body = row.Body
	}
	return p
}

func postFromPopular(row db.ListPostsPopularRow, liked bool, list bool) postDTO {
	return postFromList(db.ListPostsNewRow{
		ID:                  row.ID,
		AuthorID:            row.AuthorID,
		Title:               row.Title,
		Body:                row.Body,
		Kind:                row.Kind,
		YoutubeVideoID:      row.YoutubeVideoID,
		YoutubeThumbnailUrl: row.YoutubeThumbnailUrl,
		CloudinaryPublicID:  row.CloudinaryPublicID,
		LikeCount:           row.LikeCount,
		CommentCount:        row.CommentCount,
		CreatedAt:           row.CreatedAt,
		UpdatedAt:           row.UpdatedAt,
	}, liked, list)
}

func postFromGet(row db.GetPostRow, liked bool) postDTO {
	return postFromList(db.ListPostsNewRow{
		ID:                  row.ID,
		AuthorID:            row.AuthorID,
		Title:               row.Title,
		Body:                row.Body,
		Kind:                row.Kind,
		YoutubeVideoID:      row.YoutubeVideoID,
		YoutubeThumbnailUrl: row.YoutubeThumbnailUrl,
		CloudinaryPublicID:  row.CloudinaryPublicID,
		LikeCount:           row.LikeCount,
		CommentCount:        row.CommentCount,
		CreatedAt:           row.CreatedAt,
		UpdatedAt:           row.UpdatedAt,
	}, liked, false)
}
