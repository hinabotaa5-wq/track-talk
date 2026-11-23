package httpx

import "github.com/google/uuid"

// 特集モード（本体 #985）。設定 FEATURED_COACH_ID が空でない間は、公開面に出すコーチをその1人だけにする。
// 見た目の出し分けだけに頼らず、一覧・詳細・指名候補・割当ての API の応答で絞る。

// featuredCoach は、特集モードのときに特集するコーチの ID と true を返す。
// 値が ID として読めないときも特集モードとして扱い、uuid.Nil（誰とも一致しない）を返す。
func (s *Server) featuredCoach() (uuid.UUID, bool) {
	if s.cfg.FeaturedCoachID == "" {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(s.cfg.FeaturedCoachID)
	if err != nil {
		return uuid.Nil, true
	}
	return id, true
}

// coachHiddenByFeature は、特集モードで公開面から外すコーチなら true。
func (s *Server) coachHiddenByFeature(id uuid.UUID) bool {
	featured, on := s.featuredCoach()
	return on && id != featured
}
