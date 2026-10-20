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
		case "text":
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

// purgeMedia は使われなくなった写真・動画を裏で Cloudinary から消す。
// 失敗しても投稿の操作は成功のままにして、ログだけ残す。
func (s *Server) purgeMedia(m postMedia) {
	if m.empty() || !s.cloud.Configured() {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		s.deleteUnusedMedia(ctx, m)
	}()
}

// deleteUnusedMedia は、まだどこかで使っている ID を除いてから Cloudinary から消す。
func (s *Server) deleteUnusedMedia(ctx context.Context, m postMedia) {
	if m.empty() || !s.cloud.Configured() {
		return
	}
	inUse, err := s.q.ListMediaInUse(ctx, append(slices.Clone(m.Images), m.Videos...))
	if err != nil {
		slog.Warn("cloudinary cleanup skipped", "error", err)
		return
	}
	m = postMedia{Images: unused(m.Images, inUse), Videos: unused(m.Videos, inUse)}
	if err := s.cloud.Delete(ctx, "image", m.Images); err != nil {
		slog.Warn("cloudinary image delete failed", "public_ids", m.Images, "error", err)
	}
	if err := s.cloud.Delete(ctx, "video", m.Videos); err != nil {
		slog.Warn("cloudinary video delete failed", "public_ids", m.Videos, "error", err)
	}
}

// purgeCoachingVideos は、非公開の添削・やりとりの動画を裏で Cloudinary から消す。
func (s *Server) purgeCoachingVideos(ids []string) {
	if len(ids) == 0 || !s.cloud.Configured() {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.cloud.DeleteCoachingVideos(ctx, ids); err != nil {
			slog.Warn("cloudinary coaching video delete failed", "public_ids", ids, "error", err)
		}
	}()
}
