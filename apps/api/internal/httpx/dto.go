package httpx

import (
	"strings"
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
	ID    uuid.UUID `json:"id"`
	Title string    `json:"title"`
	Body  string    `json:"body,omitempty"`
	// Excerpt は一覧カードに出す詳細の冒頭。一覧では本文を返さないので、カードに詳細を出す投稿に付ける。
	Excerpt    string  `json:"excerpt,omitempty"`
	AuthorName *string `json:"author_name,omitempty"`
	PostMaster bool    `json:"author_post_master,omitempty"`
	authorID   uuid.UUID
	// list は一覧用の項目か。一覧では小さな画像を配る。
	list           bool
	Mine           bool            `json:"mine"`
	LikeCount      int32           `json:"like_count"`
	CommentCount   int32           `json:"comment_count"`
	LikedByMe      bool            `json:"liked_by_me"`
	SavedByMe      bool            `json:"saved_by_me"`
	Kind           string          `json:"kind"`
	ThumbnailURL   string          `json:"thumbnail_url,omitempty"`
	YouTubeVideoID string          `json:"youtube_video_id,omitempty"`
	YouTubeShorts  bool            `json:"youtube_shorts,omitempty"`
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
	Venue         string     `json:"venue"`
	StartsAt      time.Time  `json:"starts_at"`
	Event         string     `json:"event"`
	EventOther    string     `json:"event_other,omitempty"`
	EventLabel    string     `json:"event_label"`
	Capacity      int32      `json:"capacity"`
	DissolvedAt   *time.Time `json:"dissolved_at,omitempty"`
	AcceptedCount int32      `json:"accepted_count"`
	Status        string     `json:"status"`
	// Ratable は、練習の日時を過ぎてから解散した（開かれた）練習で、評価できること。
	Ratable         bool                     `json:"ratable,omitempty"`
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
	// 過去に承認されて主催者から評価された回数と、そのうちドタキャンだった回数。
	Attendance practiceAttendanceDTO `json:"attendance"`
}

type practiceAttendanceDTO struct {
	Total  int32 `json:"total"`
	NoShow int32 `json:"no_show"`
}

type practiceReputationDTO struct {
	Good     int32                          `json:"good"`
	Ok       int32                          `json:"ok"`
	Bad      int32                          `json:"bad"`
	Comments []practiceReputationCommentDTO `json:"comments,omitempty"`
}

type practiceReputationCommentDTO struct {
	ID        uuid.UUID `json:"id"`
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
	if publicID != nil && (kind == "text" || kind == "instagram") {
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
		list:         list,
	}
	applyKind(&p, row.Kind, row.YoutubeVideoID, row.YoutubeThumbnailUrl, row.CloudinaryPublicID)
	p.YouTubeShorts = row.YoutubeShorts
	applyInstagram(&p, row.InstagramType, row.InstagramShortcode)
	if row.SourceTitle != nil && *row.SourceTitle != "" {
		p.SourceTitle = *row.SourceTitle
	}
	if !list {
		p.Body = row.Body
	} else if row.Kind == "text" || row.Kind == "video" || row.Kind == "youtube" || row.Kind == "instagram" {
		p.Excerpt = postExcerpt(row.Body)
	}
	return p
}

const postExcerptRunes = 120

// postExcerpt は、本文の空白や改行をつめた冒頭を返す。
func postExcerpt(body string) string {
	return clipRunes(strings.Join(strings.Fields(body), " "), postExcerptRunes)
}

func postFromPopular(row db.ListPostsPopularRow, liked bool, list bool) postDTO {
	return postFromList(db.ListPostsNewRow(row), liked, list)
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
		YoutubeShorts:       row.YoutubeShorts,
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
	p.PostMaster = isPostMaster(row)
	return p
}

// postMasterLikedPosts は、投稿マスターのバッジに要る「いいねが1つ以上ついた投稿」の数。
// 数だけで付けると連投が増えるので、いいねのついた投稿だけを数える。
const postMasterLikedPosts = 30

// isPostMaster は、投稿の詳細で投稿者に投稿マスターのバッジを出すかどうか。
// 匿名の投稿にバッジを出すと同じ人の投稿だと見分けられてしまうので、
// ニックネームを公開している人だけにする（本人がバッジを隠していれば数は0になる）。
// 運営（実況スレッド）には付けない。
func isPostMaster(row db.GetPostRow) bool {
	return row.AuthorName != nil &&
		row.AuthorID != SystemProfileID &&
		row.AuthorLikedPostCount >= postMasterLikedPosts
}
