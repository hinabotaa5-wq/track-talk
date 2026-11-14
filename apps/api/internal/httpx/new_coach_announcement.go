package httpx

import "github.com/hinata/track-talk/apps/api/internal/db"

const retiredNewCoachAnnouncementTitle = "新しいコーチが加わりました"

// 旧自動告知（本体 #438）は固定の見出し、送信者なし、コーチID由来の UUID v5 で記録した。
// 未送信分だけを止めるため、この形式の既存レコードを識別する（本体 #693）。
func retiredNewCoachAnnouncement(a db.Announcement) bool {
	return a.SenderID == nil && a.ID.Version() == 5 && a.Title == retiredNewCoachAnnouncementTitle
}
