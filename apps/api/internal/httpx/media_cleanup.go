package httpx

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/cloudinary"
)

// postMedia は投稿に付いている Cloudinary の写真と動画。
type postMedia struct {
	Images []string
	Videos []string
}

func (m postMedia) empty() bool {
	return len(m.Images) == 0 && len(m.Videos) == 0
}

// postMediaOf は投稿を消す前に、その投稿が使っている写真・動画を集める。
func (s *Server) postMediaOf(ctx context.Context, id uuid.UUID) postMedia {
	var m postMedia
	// 運営が非表示にした投稿でも、投稿者が消すときは写真・動画を消すため、非表示を除かずに読む。
	post, err := s.q.GetPostMedia(ctx, id)
	if err != nil {
		return m
	}
	if post.CloudinaryPublicID != nil && *post.CloudinaryPublicID != "" {
		switch post.Kind {
		case "video":
			m.Videos = append(m.Videos, *post.CloudinaryPublicID)
		case "text", "instagram":
			m.Images = append(m.Images, *post.CloudinaryPublicID)
		}
	}
	if post.Kind == "recruitment" {
		photos, err := s.q.ListRecruitmentPhotos(ctx, id)
		if err == nil {
			for _, photo := range photos {
				m.Images = append(m.Images, photo.CloudinaryPublicID)
			}
		}
	}
	return m
}

// purgeRejectedUpload は、形式・大きさ・長さの条件を満たさず投稿を断った写真・動画を、Cloudinary から消す。
// 断っても残ると、条件に合わないファイルが溜まり続ける。他で使っている ID は deleteUnusedMedia が除く。
func (s *Server) purgeRejectedUpload(err error, resource, id string) {
	if !errors.Is(err, cloudinary.ErrRejected) && !errors.Is(err, cloudinary.ErrTooLarge) && !errors.Is(err, cloudinary.ErrTooLong) {
		return
	}
	if resource == "video" {
		s.purgeMedia(postMedia{Videos: []string{id}})
		return
	}
	s.purgeMedia(postMedia{Images: []string{id}})
}

// unused は、まだ残っている投稿から参照されていない ID だけを返す。
// 他人の写真の ID を自分の投稿に付けてから消す、といった手で他人の写真を消せないようにする。
func unused(ids, inUse []string) []string {
	var out []string
	for _, id := range ids {
		if !slices.Contains(inUse, id) {
			out = append(out, id)
		}
	}
	return out
}

const (
	mediaCleanupAttempts   = 4
	mediaCleanupRetryDelay = time.Second
	mediaCleanupTimeout    = 90 * time.Second
)

// retryMediaCleanup は失敗した削除を最大3回、1・2・4秒あけて再試行する。
// 呼び出し元の終了・期限切れや、不正なID・未設定のような再試行で直らない失敗では止める。
func retryMediaCleanup(ctx context.Context, delay time.Duration, cleanup func(context.Context) error) error {
	var err error
	for attempt := 0; attempt < mediaCleanupAttempts; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if attempt > 0 {
			timer := time.NewTimer(delay << (attempt - 1))
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err = cleanup(ctx)
		if err == nil || errors.Is(err, cloudinary.ErrInvalid) || errors.Is(err, cloudinary.ErrNotConfigured) {
			return err
		}
	}
	return err
}

// purgeMedia は使われなくなった写真・動画を裏で Cloudinary から消す。
// 失敗時は再試行し、上限に達したときはログを残す。投稿の操作は待たせない。
func (s *Server) purgeMedia(m postMedia) {
	if m.empty() || !s.cloud.Configured() {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), mediaCleanupTimeout)
		defer cancel()
		s.deleteUnusedMedia(ctx, m)
	}()
}

// deleteUnusedMedia は再試行ごとに使用状況を調べ直し、削除に失敗した分だけ送り直す。
func (s *Server) deleteUnusedMedia(ctx context.Context, m postMedia) {
	if m.empty() || !s.cloud.Configured() {
		return
	}
	pending := m
	if err := retryMediaCleanup(ctx, mediaCleanupRetryDelay, func(ctx context.Context) error {
		var err error
		pending, err = s.deleteUnusedMediaOnce(ctx, pending)
		return err
	}); err != nil {
		slog.Warn("cloudinary cleanup failed after retries", "images", pending.Images, "videos", pending.Videos, "error", err)
	}
}

func (s *Server) deleteUnusedMediaOnce(ctx context.Context, m postMedia) (postMedia, error) {
	if m.empty() {
		return postMedia{}, nil
	}
	inUse, err := s.q.ListMediaInUse(ctx, append(slices.Clone(m.Images), m.Videos...))
	if err != nil {
		return m, err
	}
	m = postMedia{Images: unused(m.Images, inUse), Videos: unused(m.Videos, inUse)}
	var pending postMedia
	var failures []error
	if err := s.cloud.Delete(ctx, "image", m.Images); err != nil {
		pending.Images = m.Images
		failures = append(failures, err)
	}
	if err := s.cloud.Delete(ctx, "video", m.Videos); err != nil {
		pending.Videos = m.Videos
		failures = append(failures, err)
	}
	return pending, errors.Join(failures...)
}

// purgeCoachingVideos は非公開の動画も同じ回数・間隔で再試行して消す。
func (s *Server) purgeCoachingVideos(ids []string) {
	if len(ids) == 0 || !s.cloud.Configured() {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), mediaCleanupTimeout)
		defer cancel()
		if err := retryMediaCleanup(ctx, mediaCleanupRetryDelay, func(ctx context.Context) error {
			return s.cloud.DeleteCoachingVideos(ctx, ids)
		}); err != nil {
			slog.Warn("cloudinary coaching video delete failed after retries", "public_ids", ids, "error", err)
		}
	}()
}
