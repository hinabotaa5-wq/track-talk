package httpx

import (
	"strings"

	"github.com/google/uuid"
)

// 特集モード（本体 #985）。設定 COACHING_FEATURED_COACH_ID が空でない間は、公開面に出すコーチを特集のコーチだけにする。
// 見た目の出し分けだけに頼らず、一覧・詳細・指名候補・割当ての API の応答で絞る。
// 値はカンマ区切りで複数のプロフィール ID を書ける（本体 #1063）。並び順が特集ページのスライドの順（先頭が初期表示）。

// featuredCoaches は、特集モードのときに特集するコーチの ID の並びと true を返す。
// 空の要素（"a,,b" や末尾のカンマ）は無視する。ID として読めない要素が1つでもあれば、
// 従来の「読めない単一 ID」と同じく特集モードのまま誰とも一致しない（空の並び）にする（安全側）。
func (s *Server) featuredCoaches() ([]uuid.UUID, bool) {
	if s.cfg.FeaturedCoachID == "" {
		return nil, false
	}
	var ids []uuid.UUID
	for _, part := range strings.Split(s.cfg.FeaturedCoachID, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := uuid.Parse(part)
		if err != nil {
			return nil, true
		}
		ids = append(ids, id)
	}
	return ids, true
}

// featuredCoach は、特集モードのときに先頭（初期表示）の特集コーチの ID と true を返す。
// コーチを選ばずに送った依頼の指名先に使う。特集コーチを読めないときは uuid.Nil（誰とも一致しない）を返す。
func (s *Server) featuredCoach() (uuid.UUID, bool) {
	ids, on := s.featuredCoaches()
	if !on || len(ids) == 0 {
		return uuid.Nil, on
	}
	return ids[0], true
}

// coachHiddenByFeature は、特集モードで公開面から外すコーチ（特集の並びに含まれないコーチ）なら true。
func (s *Server) coachHiddenByFeature(id uuid.UUID) bool {
	ids, on := s.featuredCoaches()
	if !on {
		return false
	}
	for _, featured := range ids {
		if id == featured {
			return false
		}
	}
	return true
}
