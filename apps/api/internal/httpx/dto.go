package httpx

import (
	"time"

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
	AuthorName     *string   `json:"author_name,omitempty"`
	authorID       uuid.UUID
	Mine           bool            `json:"mine"`
	LikeCount      int32           `json:"like_count"`
	CommentCount   int32           `json:"comment_count"`
	LikedByMe      bool            `json:"liked_by_me"`
	SavedByMe      bool            `json:"saved_by_me"`
	Kind           string          `json:"kind"`
	ThumbnailURL   string          `json:"thumbnail_url,omitempty"`
	YouTubeVideoID string          `json:"youtube_video_id,omitempty"`
	InstagramType  string          `json:"instagram_type,omitempty"`
	InstagramCode  string          `json:"instagram_shortcode,omitempty"`
	VideoPublicID  string          `json:"video_public_id,omitempty"`
	ImagePublicID  string          `json:"image_public_id,omitempty"`
	PlaybackURL    string          `json:"playback_url,omitempty"`
	SourceTitle    string          `json:"source_title,omitempty"`
	Recruitment    *recruitmentDTO `json:"recruitment,omitempty"`
	Practice       *practiceDTO    `json:"practice,omitempty"`
	Injury         *injuryDTO      `json:"injury,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

type recruitmentPhotoDTO struct {
	PublicID string `json:"public_id"`
	URL      string `json:"url"`
}

type recruitmentDTO struct {
	Prefectures []string              `json:"prefectures"`
	Venue       string                `json:"venue"`
	Schedule    string                `json:"schedule"`
	GenderRatio string                `json:"gender_ratio"`
	AverageAge  string                `json:"average_age"`
	RecruitAge  string                `json:"recruit_age"`
	Links       []string              `json:"links"`
	Photos      []recruitmentPhotoDTO `json:"photos"`
}

type practiceDTO struct {
	Venue           string                   `json:"venue"`
	StartsAt        time.Time                `json:"starts_at"`
	Event           string                   `json:"event"`
	EventOther      string                   `json:"event_other,omitempty"`
	EventLabel      string                   `json:"event_label"`
	Capacity        int32                    `json:"capacity"`
	DissolvedAt     *time.Time               `json:"dissolved_at,omitempty"`
	AcceptedCount   int32                    `json:"accepted_count"`
	Status          string                   `json:"status"`
	MyApplication   string                   `json:"my_application,omitempty"`
	MyApplicationID *uuid.UUID               `json:"my_application_id,omitempty"`
	RatedHost       bool                     `json:"rated_host,omitempty"`
	Applications    []practiceApplicationDTO `json:"applications,omitempty"`
	Reputation      *practiceReputationDTO   `json:"reputation,omitempty"`
}

type practiceApplicationDTO struct {
	ID        uuid.UUID `json:"id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	Rated     bool      `json:"rated,omitempty"`
}

type practiceReputationDTO struct {
	Good     int32                          `json:"good"`
	Ok       int32                          `json:"ok"`
	Bad      int32                          `json:"bad"`
	Comments []practiceReputationCommentDTO `json:"comments,omitempty"`
}

type practiceReputationCommentDTO struct {
	Score     string    `json:"score"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
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
	if kind == "youtube" && thumbnail != nil && YouTubeThumbnailURL(*thumbnail) == "" {
		p.ThumbnailURL = *thumbnail
	}
}

func applyInstagram(p *postDTO, instagramType, shortcode *string) {
	if p.Kind != "instagram" || instagramType == nil || shortcode == nil {
		return
	}
	if (*instagramType != "p" && *instagramType != "reel") || InstagramShortcode(*shortcode) != "" {
		return
	}
	p.InstagramType = *instagramType
	p.InstagramCode = *shortcode
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
	applyInstagram(&p, row.InstagramType, row.InstagramShortcode)
	if row.SourceTitle != nil && *row.SourceTitle != "" {
		p.SourceTitle = *row.SourceTitle
	}
	if !list {
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
		InstagramType:       row.InstagramType,
		InstagramShortcode:  row.InstagramShortcode,
		CloudinaryPublicID:  row.CloudinaryPublicID,
		SourceTitle:         row.SourceTitle,
		LikeCount:           row.LikeCount,
		CommentCount:        row.CommentCount,
		CreatedAt:           row.CreatedAt,
		UpdatedAt:           row.UpdatedAt,
	}, liked, list)
}

func postFromGet(row db.GetPostRow, liked bool) postDTO {
	p := postFromList(db.ListPostsNewRow{
		ID:                  row.ID,
		AuthorID:            row.AuthorID,
		Title:               row.Title,
		Body:                row.Body,
		Kind:                row.Kind,
		YoutubeVideoID:      row.YoutubeVideoID,
		YoutubeThumbnailUrl: row.YoutubeThumbnailUrl,
		InstagramType:       row.InstagramType,
		InstagramShortcode:  row.InstagramShortcode,
		CloudinaryPublicID:  row.CloudinaryPublicID,
		SourceTitle:         row.SourceTitle,
		LikeCount:           row.LikeCount,
		CommentCount:        row.CommentCount,
		CreatedAt:           row.CreatedAt,
		UpdatedAt:           row.UpdatedAt,
	}, liked, false)
	p.AuthorName = row.AuthorName
	return p
}
