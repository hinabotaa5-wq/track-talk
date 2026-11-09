package httpx

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/cloudinary"
)

// 縮めずに保存した動画は、配信用の派生ができたら縮めた版へ置き換える（本体 #497）。
// 元ファイルに残る撮影日時・位置情報・端末情報を Cloudinary に残さず、保存容量も抑えるため。
// 起点は「投稿・送信に使える」と確かめた時点（videoUploadStatus）で、取りこぼしは日次の掃除（storageCleanup）で拾う。

// originalShrinkDelays は、再生用の派生ができるのを待つ間隔。合計で約10分待ち、それでもまだなら日次の掃除に任せる。
var originalShrinkDelays = []time.Duration{
	5 * time.Second, 10 * time.Second, 20 * time.Second, 30 * time.Second,
	time.Minute, time.Minute, 2 * time.Minute, 2 * time.Minute, 3 * time.Minute,
}

// originalShrinkMaxPerRun は、日次の掃除で1回に置き換える上限（Admin API の回数制限を食い尽くさないため）。
const originalShrinkMaxPerRun = 50

var originalShrinking sync.Map

// shrinkOriginalSoon は、同じ動画を重ねて置き換えないようにしてから、裏で置き換える。投稿の操作は待たせない。
func (s *Server) shrinkOriginalSoon(publicID string) {
	if !s.cloud.Configured() {
		return
	}
	if _, busy := originalShrinking.LoadOrStore(publicID, true); busy {
		return
	}
	go func() {
		defer originalShrinking.Delete(publicID)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		s.shrinkOriginal(ctx, publicID, originalShrinkDelays)
	}()
}

// shrinkOriginal は、派生ができるまで delays の間隔で待ちながら置き換えを試みる。
func (s *Server) shrinkOriginal(ctx context.Context, publicID string, delays []time.Duration) {
	for attempt := 0; ; attempt++ {
		err := s.cloud.ShrinkOriginal(ctx, publicID)
		switch {
		case err == nil:
			slog.Info("cloudinary original replaced", "public_id", publicID)
			return
		case errors.Is(err, cloudinary.ErrNotOriginal), errors.Is(err, cloudinary.ErrNotFound):
			return
		case !errors.Is(err, cloudinary.ErrProcessing):
			slog.Warn("cloudinary original replace failed", "public_id", publicID, "error", err)
			return
		}
		if attempt >= len(delays) {
			slog.Info("cloudinary original still processing; left for daily cleanup", "public_id", publicID)
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(delays[attempt]):
		}
	}
}

// shrinkPendingOriginals は、まだ置き換えていない動画をまとめて置き換え、置き換えた数を返す。
// 派生がまだの動画や失敗した動画は次の回に回す。失敗しても日次の掃除そのものは止めない。
func (s *Server) shrinkPendingOriginals(ctx context.Context) int {
	if !s.cloud.Configured() {
		return 0
	}
	done, tried := 0, 0
	cursor := ""
	for {
		page, next, err := s.cloud.ListOriginals(ctx, cursor)
		if err != nil {
			slog.Warn("cloudinary originals list failed", "error", err)
			return done
		}
		for _, a := range page {
			if tried >= originalShrinkMaxPerRun || ctx.Err() != nil {
				return done
			}
			tried++
			if _, busy := originalShrinking.LoadOrStore(a.PublicID, true); busy {
				continue
			}
			err := s.cloud.ShrinkOriginal(ctx, a.PublicID)
			originalShrinking.Delete(a.PublicID)
			switch {
			case err == nil:
				done++
			case errors.Is(err, cloudinary.ErrProcessing), errors.Is(err, cloudinary.ErrNotOriginal), errors.Is(err, cloudinary.ErrNotFound):
			default:
				slog.Warn("cloudinary original replace failed", "public_id", a.PublicID, "error", err)
			}
		}
		if next == "" {
			return done
		}
		cursor = next
	}
}
