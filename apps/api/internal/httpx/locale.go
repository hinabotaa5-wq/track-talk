package httpx

import (
	"context"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/push"
)

// 表示言語（本体 #521）。利用者の設定として profiles.locale に持つ（ja / en）。
// 未訳の箇所は日本語にフォールバックする。送るときに落ちないよう、読めない相手は ja とみなす。

// normalizeLocale は、言語設定を ja / en に正規化する。en 以外はすべて ja。
func normalizeLocale(v string) string {
	if v == "en" {
		return "en"
	}
	return "ja"
}

// recipientLocale は、その人の表示言語を返す。読めなければ ja。
func (s *Server) recipientLocale(ctx context.Context, id uuid.UUID) string {
	p, err := s.q.GetProfileByID(ctx, id)
	if err != nil {
		return "ja"
	}
	return normalizeLocale(p.Locale)
}

// pushTitleEN は、プッシュ・ベルの表題の日英対応。表題はすべて固定文なので、一致すれば置き換える。
// 本文に名前・金額・日時が入るものは置き換えない（日本語フォールバック）。
var pushTitleEN = map[string]string{
	// 添削ワークフロー
	"動画添削の依頼が届きました":           "New video-review request",
	"フォーム添削の引受期限が近づいています":     "Form-review request expiring soon",
	"フォーム添削の依頼が期限を過ぎました":      "Form-review request expired",
	"添削中の依頼が期限を過ぎました":         "In-progress review expired",
	"指名されたフォーム添削の依頼が終了しました":   "Nominated review request closed",
	"選んだコーチが依頼を引き受けられませんでした":  "Chosen coach couldn't take the request",
	"コーチがフォーム添削の依頼を引き受けました":   "Coach accepted the review request",
	"コーチが依頼を辞退しました":           "Coach declined the request",
	"コーチが依頼を返却しました":           "Coach returned the request",
	"フォーム添削の返金が完了しました":        "Form-review refund completed",
	"フォーム添削の仮押さえの取り消しが完了しました": "Form-review hold voided",
	"追加相談の返金が完了しました":          "Consultation refund completed",
	"フォーム添削の返答が届きました":         "Your form-review response arrived",
	"添削の補足を頼まれました":            "Asked for a review follow-up",
	"添削の補足が届きました":             "Review follow-up arrived",
	"コーチから依頼内容の確認が届きました":      "Coach asked for request details",
	"差し戻した依頼が再提出されました":        "Returned request resubmitted",
	"差し戻した依頼が終了しました":          "Returned request closed",
	"フォーム添削の依頼が終了しました":        "Form-review request closed",
	"アドバイスは役に立ちましたか":          "Was the advice helpful?",
	"依頼の受付を始めました":             "Started accepting requests",
	"フォーム添削の通知テスト":            "Form-review notification test",
	"コーチの応募の審査が終わりました":        "Coach application review finished",
	"コーチの応募が承認されました":          "Coach application approved",
	// 追加相談
	"追加相談が届きました":     "New consultation received",
	"追加相談が更新されました":   "Consultation updated",
	"回答への確認質問が届きました": "Follow-up question on the answer",
	"追加相談の期限が過ぎました":  "Consultation expired",
	"追加質問が届きました":     "New additional question received",
	"追加質問が更新されました":   "Additional question updated",
	"追加質問の期限が過ぎました":  "Additional question expired",
	"追加質問の返金が完了しました": "Additional question refund completed",
	// パーソナル指導
	"パーソナル指導の申し込みが届きました":    "New personal-coaching application",
	"パーソナル指導が始まりました":        "Personal coaching started",
	"パーソナル指導の申し込みの結果":       "Personal-coaching application result",
	"コーチが申し込みを承諾しました":       "Coach accepted your application",
	"パーソナル指導は始まりませんでした":     "Personal coaching didn't start",
	"パーソナル指導の申し込みの期限が過ぎました": "Personal-coaching application expired",
	"パーソナル指導の期間がもうすぐ終わります":  "Personal coaching ending soon",
	"パーソナル指導の期間が終わりました":     "Personal coaching ended",
	"パーソナル指導が終了しました":        "Personal coaching ended",
	"パーソナル指導に新しいメッセージ":      "New personal-coaching message",
	"コーチからメッセージが届きました":      "Message from your coach",
	"保護者が同意しました":            "Guardian consented",
	"保護者が同意を取り消しました":        "Guardian withdrew consent",
	"支払いに異議が申し立てられました":      "Payment disputed",
	"お支払いを返金しました":           "Payment refunded",
	"パーソナル指導を返金しました":        "Personal coaching refunded",
	"無料相談の結果":               "Consultation result",
	"相談が届きました":              "New consultation request",
	"無料相談の聞き直しに答えが届きました":    "Answer to your consultation question",
	"無料相談にコーチの提案が届きました":     "Coach sent a proposal",
	"無料相談の期限が過ぎました":         "Consultation request expired",
	"保護者が提案に同意しませんでした":      "Guardian declined the proposal",
	"1ヶ月プランが終了しました":         "1-month plan ended",
	"異議申立ての再審査が終わりました":      "Appeal review finished",
	"18歳未満のパーソナル指導に通報":      "Report on under-18 coaching",
	"パーソナル指導のやりとりを再開しました":   "Personal-coaching chat resumed",
	"パーソナル指導のやりとりを止めました":    "Personal-coaching chat paused",
	"対面指導が更新されました":          "In-person session updated",
	"大会サポートの通話が更新されました":     "Meet-support call updated",
	"報酬を送金しました":             "Payout sent",
	// 共通
	"大会の記録を残せていますか？":         "Have you logged your meet results?",
	"いいねがありました":              "New like",
	"参加申請がありました":             "New practice application",
	"参加が承認されました":             "Participation approved",
	"TrackTalk を開いて確認してください": "Open TrackTalk to check",
}

// pushBodyEN は、プッシュ・ベルの本文の日英対応。本文は固定文のものだけ置き換える。
// 名前・金額・日時が入る本文はここに置かず、日本語のまま出す。
// 通知は表題1行だけにして本文は原則空にした（本体 #609）。空の本文は置き換えずに空のまま出す。
var pushBodyEN = map[string]string{
	// 添削ワークフロー
	"返答が届くまでお待ちください":           "Please wait for the response",
	"この依頼の引受受付は終了しました":         "Accepting for this request has ended",
	"返却の目安を過ぎたため、この依頼は終わりました":  "The request ended because the response guideline passed",
	"コーチからのアドバイスを見てみましょう":      "See the coach's advice",
	"わからなかったところに、補足を1回返してください": "Please send one follow-up on what was unclear",
	"わからなかったところへの補足を見てみましょう":   "See the follow-up on what was unclear",
	"1週間前のフォーム添削について教えてください":   "Tell us about the form review from a week ago",
	"フォーム添削の依頼が届くと、この端末に通知します": "You'll be notified on this device when a review request arrives",
	"依頼の受付を始める前の接続テストです":       "This is a connection test before accepting requests",
	// 追加相談
	// パーソナル指導
	"終わる前に、文章とメニューをダウンロードしてください": "Download your messages and menus before it ends",
	"止めていた日数の分、期間を延ばしました":        "Extended by the paused days",
	"運営が確認しています。終わるまで送れません":      "Under review by staff. You can't send until it's done",
	// 共通
	"通知の設定ができました": "Notifications are set up",
}

// localizePushMessage は、プッシュ・ベルの文面を受け手の言語に合わせる。
// en でも表にない文面（名前・金額・日時入りの本文など）は日本語のまま返す。
func localizePushMessage(locale string, msg push.Message) push.Message {
	if normalizeLocale(locale) != "en" {
		return msg
	}
	if title, ok := pushTitleEN[msg.Title]; ok {
		msg.Title = title
	}
	if body, ok := pushBodyEN[msg.Body]; ok {
		msg.Body = body
	}
	return msg
}
