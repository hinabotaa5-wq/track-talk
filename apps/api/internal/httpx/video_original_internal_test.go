package httpx

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/cloudinary"
)

// originalsCloud は、置き換え待ちの動画の一覧・詳細と、置き換えの受け付けを真似る。
// ready に入っている public_id は再生用の派生ができている。
type originalsCloud struct {
	mu       sync.Mutex
	tagged   map[string]bool
	ready    map[string]bool
	replaced []string
	details  int
}

func (o *originalsCloud) server() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		o.mu.Lock()
		defer o.mu.Unlock()
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/resources/video/tags/"):
			var items []string
			for id := range o.tagged {
				items = append(items, fmt.Sprintf(`{"public_id":%q,"type":"upload"}`, id))
			}
			fmt.Fprintf(w, `{"resources":[%s]}`, strings.Join(items, ","))
		case r.Method == http.MethodGet:
			o.details++
			id := r.URL.Path[strings.Index(r.URL.Path, "/upload/")+len("/upload/"):]
			if !o.tagged[id] {
				fmt.Fprint(w, `{"tags":[]}`)
				return
			}
			derived := ""
			if o.ready[id] {
				derived = `{"transformation":"c_limit,h_1280,q_auto,w_1280/mp4","format":"mp4"}`
			}
			fmt.Fprintf(w, `{"tags":["tt-original"],"derived":[%s]}`, derived)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/video/upload"):
			_ = r.ParseForm()
			o.replaced = append(o.replaced, r.PostForm.Get("public_id"))
			fmt.Fprint(w, `{}`)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/video/tags"):
			_ = r.ParseForm()
			delete(o.tagged, r.PostForm.Get("public_ids"))
			fmt.Fprint(w, `{}`)
		default:
			http.NotFound(w, r)
		}
	}))
}

func originalsServer(srv *httptest.Server) *Server {
	return &Server{cloud: cloudinary.Client{Cloud: "demo", Key: "key", Secret: "secret", APIBase: srv.URL}}
}

func TestShrinkOriginalWaitsForPlaybackThenReplaces(t *testing.T) {
	id := cloudinary.NewPublicID()
	o := &originalsCloud{tagged: map[string]bool{id: true}, ready: map[string]bool{}}
	srv := o.server()
	defer srv.Close()
	s := originalsServer(srv)
	delays := []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
	go func() {
		time.Sleep(20 * time.Millisecond)
		o.mu.Lock()
		o.ready[id] = true
		o.mu.Unlock()
	}()
	// 派生ができるまで待ち、できたら1回だけ置き換える。
	for i := 0; i < 50; i++ {
		s.shrinkOriginal(context.Background(), id, delays)
		o.mu.Lock()
		n := len(o.replaced)
		o.mu.Unlock()
		if n > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if len(o.replaced) != 1 || o.replaced[0] != id || o.tagged[id] {
		t.Fatalf("replaced %v tagged %v", o.replaced, o.tagged)
	}
	// 置き換え済みなら何もしない。
	s.shrinkOriginal(context.Background(), id, delays)
	if len(o.replaced) != 1 {
		t.Fatalf("replaced twice: %v", o.replaced)
	}
}

func TestShrinkOriginalGivesUpAfterDelays(t *testing.T) {
	id := cloudinary.NewPublicID()
	o := &originalsCloud{tagged: map[string]bool{id: true}, ready: map[string]bool{}}
	srv := o.server()
	defer srv.Close()
	originalsServer(srv).shrinkOriginal(context.Background(), id, []time.Duration{time.Millisecond, time.Millisecond})
	if len(o.replaced) != 0 || o.details != 3 || !o.tagged[id] {
		t.Fatalf("replaced %v details %d", o.replaced, o.details)
	}
}

func TestShrinkPendingOriginalsReplacesReadyAndKeepsTheRest(t *testing.T) {
	ready, waiting := cloudinary.NewPublicID(), cloudinary.NewPublicID()
	o := &originalsCloud{tagged: map[string]bool{ready: true, waiting: true}, ready: map[string]bool{ready: true}}
	srv := o.server()
	defer srv.Close()
	if got := originalsServer(srv).shrinkPendingOriginals(context.Background()); got != 1 {
		t.Fatalf("replaced count %d", got)
	}
	if len(o.replaced) != 1 || o.replaced[0] != ready || !o.tagged[waiting] {
		t.Fatalf("replaced %v tagged %v", o.replaced, o.tagged)
	}
}

func TestShrinkPendingOriginalsWithoutCloudinary(t *testing.T) {
	if got := (&Server{}).shrinkPendingOriginals(context.Background()); got != 0 {
		t.Fatalf("got %d", got)
	}
}
