// 表示言語の辞書（本体 #521）。対応言語は当面 ja / en。
// 英語文言が未用意の箇所は日本語にフォールバックする（キー名や空白を出さない）。
// 選手側画面の辞書整備は段階対応で、ここでは共通部分とコーチ導線を優先する。

export type Locale = "ja" | "en";

// 言語設定の値を ja / en に正規化する。en 以外はすべて ja。
export function parseLocale(value: unknown): Locale {
  return value === "en" ? "en" : "ja";
}

const storageKey = "tracktalk-locale";

// 未ログイン時の言語プロンプト（本体 #521 追加要件・暫定）に答えた記録。答えるまで未ログインで開くたびに出す。
const promptKey = "tracktalk:locale-prompted";

// 言語が変わった合図。プロンプトで選んだ言語を、開いている登録・ログイン画面に反映するために使う。
export const localeChangedEvent = "tracktalk:locale-changed";

export function notifyLocaleChanged(): void {
  try {
    window.dispatchEvent(new Event(localeChangedEvent));
  } catch {
    // 送れなくても言語の保存自体は済んでいる。
  }
}

// ログイン前（登録画面など）の表示言語。ブラウザにだけ置く。
export function readStoredLocale(): Locale {
  if (typeof window === "undefined") return "ja";
  try {
    return parseLocale(window.localStorage.getItem(storageKey));
  } catch {
    return "ja";
  }
}

export function storeLocale(locale: Locale): void {
  try {
    window.localStorage.setItem(storageKey, locale);
  } catch {
    // 保存できなくても登録・設定は続ける。
  }
}

export function localePromptAnswered(): boolean {
  if (typeof window === "undefined") return true;
  try {
    return window.localStorage.getItem(promptKey) === "1";
  } catch {
    return true;
  }
}

export function markLocalePromptAnswered(): void {
  try {
    window.localStorage.setItem(promptKey, "1");
  } catch {
    // 記録できなくても先へ進める（次回また出す）。
  }
}

const ja = {
  "nav.main": "メイン",
  "nav.home": "ホーム",
  "nav.competitions": "大会",
  "nav.coach": "コーチ",
  "nav.memo": "メモ",
  "nav.injury": "ケガ",
  "nav.me": "マイページ",

  "common.loading": "読み込み中...",
  "common.save": "保存",
  "common.sending": "送っています",
  "common.back": "戻る",
  "common.close": "閉じる",
  "common.sendFailed": "送信に失敗しました",
  "common.loadFailed": "読み込みに失敗しました",

  "register.title": "新規登録",
  "register.email": "メール",
  "register.password": "パスワード（8文字以上）",
  "register.submit": "登録",
  "register.google": "Googleで登録",
  "register.hasAccount": "アカウントがある場合は",
  "register.login": "ログイン",
  "register.language": "表示言語",
  "register.languageHint": "あとからマイページの設定で変えられます。",
  "register.language.ja": "日本語",
  "register.language.en": "English",

  "login.title": "ログイン",
  "login.email": "メール",
  "login.password": "パスワード",
  "login.submit": "ログイン",
  "login.noAccount": "アカウントがない場合は",
  "login.register": "登録",
  "login.forgot": "パスワードを忘れた",
  "login.contact": "ログインできないときのお問い合わせ",

  "mypage.title": "マイページ",
  "profile.settingsTitle": "ニックネームと種目の変更",
  "profile.onboardingTitle": "ニックネームと種目を決めよう",
  "profile.onboardingHint": "どちらも、使い始めるのに必要です。あとから変えられます。",
  "profile.nickname": "ニックネーム",
  "profile.event": "種目",
  "profile.eventPlaceholder": "選んでください",
  "profile.nicknamePublic": "投稿にニックネームを公開する",
  "profile.language": "表示言語",
  "profile.languageHint": "選ぶとすぐに切り替わります。英語の文言がない箇所は日本語で表示します。",
  "profile.saved": "プロフィールを更新しました。",

  "notify.title": "通知",
  "notify.unread": "通知、未読{count}件",
  "notify.markAll": "すべて既読",
  "notify.empty": "通知はありません",
  "notify.loadError": "通知を読み込めませんでした",
  "notify.readError": "既読にできませんでした",
  "notify.read": "既読",
  "notify.unreadLabel": "未読",

  "notify.recruitment_post": "新しい募集",
  "notify.comment": "コメントがありました",
  "notify.comment_reply": "あなたのコメントに返信がありました",
  "notify.practice_application": "参加申請がありました",
  "notify.practice_accepted": "参加が承認されました",
  "notify.practice_rate": "解散しました。評価しあってください",
  "notify.injury_update": "復帰の経過が更新されました",
  "notify.review_offer": "フォーム添削の依頼が届きました",
  "notify.review_offer_reminder": "フォーム添削の引受期限が近づいています",
  "notify.review_accepted": "コーチがフォーム添削の依頼を引き受けました",
  "notify.review_nomination_ended": "指名されたフォーム添削の依頼が終了しました",
  "notify.review_expired": "フォーム添削の依頼が期限を過ぎました",
  "notify.review_returned": "フォーム添削の返答が届きました",
  "notify.review_clarify": "添削の補足を頼まれました",
  "notify.review_clarified": "添削の補足が届きました",
  "notify.review_info_request": "コーチから依頼内容の確認が届きました",
  "notify.review_resubmitted": "差し戻した依頼が再提出されました",
  "notify.review_resubmitted_after_accept": "差し戻した依頼が再提出されました",
  "notify.review_withdrawn": "コーチが依頼を辞退しました",
  "notify.review_helpful_ask": "アドバイスは役に立ちましたか",
  "notify.message": "お知らせ",

  "coach.title": "コーチ画面",
  "coach.notReady": "コーチングは準備中です。競技や指導の経験を生かして、選手の次の一歩を支えるコーチを募集しています。",
  "coach.checkApplication": "応募状況・応募内容を確認する",
  "coach.preparing": "コーチの応募は承認されています。現在は受付開始に向けて準備中です。機能の公開後、この画面から依頼への返答やプランの管理ができます。",
  "coach.about": "コーチ機能について",
  "coach.viewApplication": "応募内容を確認する",
  "coach.withdraw": "コーチをやめる",
  "coach.withdrawNote": "アカウントは残り、一般の利用者として使い続けられます。",
  "coach.onDuty": "受付中",
  "coach.offDuty": "受付停止中",
  "coach.dailyCap": "1日に受ける上限",
  "coach.perDay": "{n}本",
  "coach.pausedNote": "評価の平均が★3を下回ったため、いまは依頼が回ってきません。",
  "coach.newOffers": "新しい依頼が{count}件あります — あと{minutes}分で終了",
  "coach.status": "受付の状態",
  "coach.menu": "コーチのメニュー",
  "coach.tile.offers": "届いた依頼",
  "coach.tile.reviewing": "添削中",
  "coach.tile.consultations": "追加相談",
  "coach.tile.contracts": "契約・申込み",
  "coach.tile.supportConsults": "無料相談",
  "coach.tile.plans": "プラン管理",
  "coach.tile.availability": "カレンダー",
  "coach.tile.commerce": "特商法・身元",
  "coach.tile.payouts": "報酬・口座",
  "coach.tile.profile": "プロフィール",
  "coach.tile.push": "通知設定",
  "coach.tile.count": "{label}（{count}件）",
  "coach.tile.attention": "{label}（要確認）",
  "coach.unpaid": "未払い",
  "coach.nextPayout": "次回{month}月{day}日",
  "coach.loadError": "読み込みに失敗しました",
  "coach.saveError": "保存できませんでした",
  "coach.offersTitle": "届いた依頼",
  "coach.reviewingTitle": "添削中",

  "coach.offers.note": "新しい依頼が入ってから90分以内（23:00–07:00 はカウントが止まります）に引き受けないと、依頼は終了します。",
  "coach.offers.empty": "いま届いている依頼はありません",
  "coach.offers.deadline": "{date}までに引き受けてください（あと{minutes}分）。過ぎると依頼は終了します",
  "coach.offers.accept": "引き受ける",
  "coach.offers.decline": "見送る",
  "coach.offers.loadError": "依頼を読み込めませんでした",
  "coach.reviewing.empty": "添削中の依頼はありません",
  "coach.reviewing.writeResponse": "返答を書く",
  "coach.reviewing.writeFollowup": "補足を書く",
  "coach.reviewing.open": "依頼を開く",
  "coach.reviewing.due": "返却期限: {date}",
  "coach.reviewing.picked": "あなたを選んだ依頼 · 報酬{amount}",
  "coach.reviewing.question": "相談内容: {text}",

  "coach.detail.notFound": "依頼が見つかりません",
  "coach.detail.loadError": "読み込みに失敗しました",
  "coach.detail.due": "返却期限: {date}",
  "coach.detail.question": "相談内容: {text}",
  "coach.detail.responseTitle": "返答を書く",
  "coach.detail.contactNote": "URL・メールアドレス・電話番号・SNS の ID など、連絡先らしきものは送れません。直接のやりとりに誘うと利用停止になります。",
  "coach.detail.responseNote": "返答と一緒に、ニックネーム・担当種目・記録・成績や指導経験が利用者に届きます。",
  "coach.detail.good": "良い点（1つ）",
  "coach.detail.fix1": "直す点（1つ目）",
  "coach.detail.fix2": "直す点（2つ目、なくてもよい）",
  "coach.detail.drill": "おすすめのドリル（1つ）",
  "coach.detail.demoVideo": "見本動画（なくてもよい、{seconds}秒まで）",
  "coach.detail.demoTooBig": "100MB以内の動画にしてください",
  "coach.detail.demoTooLong": "見本動画は{seconds}秒以内にしてください",
  "coach.detail.sendResponse": "返答を送る",
  "coach.detail.followup": "補足（1回だけ送れます）",
  "coach.detail.sendFollowup": "補足を送る",
  "coach.detail.clarifiedAs": "「{reason}」と言われました",
  "coach.detail.toCoach": "コーチ画面へ",
} as const;

export type I18nKey = keyof typeof ja;

// 英語辞書。ないキーは日本語にフォールバックする（t の実装で吸収）。
const en: Partial<Record<I18nKey, string>> = {
  "nav.main": "Main",
  "nav.home": "Home",
  "nav.competitions": "Meets",
  "nav.coach": "Coach",
  "nav.memo": "Notes",
  "nav.injury": "Injury",
  "nav.me": "My Page",

  "common.loading": "Loading...",
  "common.save": "Save",
  "common.sending": "Sending...",
  "common.back": "Back",
  "common.close": "Close",
  "common.sendFailed": "Couldn't send. Please try again.",
  "common.loadFailed": "Couldn't load. Please try again.",

  "register.title": "Create account",
  "register.email": "Email",
  "register.password": "Password (8+ characters)",
  "register.submit": "Sign up",
  "register.google": "Continue with Google",
  "register.hasAccount": "If you have an account,",
  "register.login": "Log in",
  "register.language": "Language",
  "register.languageHint": "You can change it later in My Page settings.",
  "register.language.ja": "日本語",
  "register.language.en": "English",

  "login.title": "Log in",
  "login.email": "Email",
  "login.password": "Password",
  "login.submit": "Log in",
  "login.noAccount": "If you don't have an account,",
  "login.register": "Sign up",
  "login.forgot": "Forgot password",
  "login.contact": "Contact us if you can't log in",

  "mypage.title": "My Page",
  "profile.settingsTitle": "Edit nickname and event",
  "profile.onboardingTitle": "Choose your nickname and event",
  "profile.onboardingHint": "Both are needed to get started. You can change them later.",
  "profile.nickname": "Nickname",
  "profile.event": "Event",
  "profile.eventPlaceholder": "Select",
  "profile.nicknamePublic": "Show my nickname on posts",
  "profile.language": "Language",
  "profile.languageHint": "Switches immediately. Parts without English text stay in Japanese.",
  "profile.saved": "Profile updated.",

  "notify.title": "Notifications",
  "notify.unread": "{count} unread notifications",
  "notify.markAll": "Mark all read",
  "notify.empty": "No notifications",
  "notify.loadError": "Couldn't load notifications",
  "notify.readError": "Couldn't mark as read",
  "notify.read": "Read",
  "notify.unreadLabel": "Unread",

  "notify.recruitment_post": "New session",
  "notify.comment": "New comment",
  "notify.comment_reply": "Reply to your comment",
  "notify.practice_application": "New participation request",
  "notify.practice_accepted": "Participation approved",
  "notify.practice_rate": "Session ended. Please rate each other",
  "notify.injury_update": "Recovery update posted",
  "notify.review_offer": "New form-review request",
  "notify.review_offer_reminder": "Form-review request expiring soon",
  "notify.review_accepted": "Coach accepted the review request",
  "notify.review_nomination_ended": "Nominated review request closed",
  "notify.review_expired": "Form-review request expired",
  "notify.review_returned": "Form-review response arrived",
  "notify.review_clarify": "Asked for a review follow-up",
  "notify.review_clarified": "Review follow-up arrived",
  "notify.review_info_request": "Coach asked for request details",
  "notify.review_resubmitted": "Returned request resubmitted",
  "notify.review_resubmitted_after_accept": "Returned request resubmitted",
  "notify.review_withdrawn": "Coach declined the request",
  "notify.review_helpful_ask": "Was the advice helpful?",
  "notify.message": "Notice",

  "coach.title": "Coach",
  "coach.notReady": "Coaching is getting ready. We're looking for coaches who can support athletes' next step with their competition and coaching experience.",
  "coach.checkApplication": "Check application status",
  "coach.preparing": "Your coach application is approved. We're preparing for launch. After release, you'll be able to reply to requests and manage plans here.",
  "coach.about": "About coaching",
  "coach.viewApplication": "View application",
  "coach.withdraw": "Leave coaching",
  "coach.withdrawNote": "Your account stays and you can keep using the app as a regular user.",
  "coach.onDuty": "Accepting",
  "coach.offDuty": "Paused",
  "coach.dailyCap": "Daily limit",
  "coach.perDay": "{n} / day",
  "coach.pausedNote": "Requests are paused because the average rating is below 3 stars.",
  "coach.newOffers": "{count} new request(s) — closing in {minutes} min",
  "coach.status": "Availability",
  "coach.menu": "Coach menu",
  "coach.tile.offers": "Incoming",
  "coach.tile.reviewing": "Reviewing",
  "coach.tile.consultations": "Consultations",
  "coach.tile.contracts": "Contracts",
  "coach.tile.supportConsults": "Free consults",
  "coach.tile.plans": "Plans",
  "coach.tile.availability": "Calendar",
  "coach.tile.commerce": "Legal & identity",
  "coach.tile.payouts": "Payouts",
  "coach.tile.profile": "Profile",
  "coach.tile.push": "Notifications",
  "coach.tile.count": "{label} ({count})",
  "coach.tile.attention": "{label} (needs attention)",
  "coach.unpaid": "Unpaid",
  "coach.nextPayout": "Next {month}/{day}",
  "coach.loadError": "Couldn't load. Please try again.",
  "coach.saveError": "Couldn't save. Please try again.",
  "coach.offersTitle": "Incoming requests",
  "coach.reviewingTitle": "In progress",

  "coach.offers.note": "If you don't accept within 90 minutes of a new request (paused 23:00–07:00), the request closes.",
  "coach.offers.empty": "No incoming requests",
  "coach.offers.deadline": "Please accept by {date} ({minutes} min left). After that the request closes.",
  "coach.offers.accept": "Accept",
  "coach.offers.decline": "Decline",
  "coach.offers.loadError": "Couldn't load requests",
  "coach.reviewing.empty": "No reviews in progress",
  "coach.reviewing.writeResponse": "Write response",
  "coach.reviewing.writeFollowup": "Write follow-up",
  "coach.reviewing.open": "Open request",
  "coach.reviewing.due": "Due: {date}",
  "coach.reviewing.picked": "Requested you · Reward {amount}",
  "coach.reviewing.question": "Question: {text}",

  "coach.detail.notFound": "Request not found",
  "coach.detail.loadError": "Couldn't load. Please try again.",
  "coach.detail.due": "Due: {date}",
  "coach.detail.question": "Question: {text}",
  "coach.detail.responseTitle": "Write a response",
  "coach.detail.contactNote": "Don't send anything that looks like contact info (URLs, emails, phone numbers, social IDs). Asking for direct contact results in suspension.",
  "coach.detail.responseNote": "Your nickname, events, records, results, and coaching experience will be sent with the response.",
  "coach.detail.good": "Strong point (1)",
  "coach.detail.fix1": "Fix (1st)",
  "coach.detail.fix2": "Fix (2nd, optional)",
  "coach.detail.drill": "Recommended drill (1)",
  "coach.detail.demoVideo": "Demo video (optional, up to {seconds}s)",
  "coach.detail.demoTooBig": "Please keep videos under 100MB",
  "coach.detail.demoTooLong": "Please keep demo videos under {seconds}s",
  "coach.detail.sendResponse": "Send response",
  "coach.detail.followup": "Follow-up (one time only)",
  "coach.detail.sendFollowup": "Send follow-up",
  "coach.detail.clarifiedAs": "\"{reason}\" was requested",
  "coach.detail.toCoach": "Back to Coach",
};

// 文言を取る。英語にないキーは日本語を返す。{name} は vars で置き換える。
export function t(locale: Locale, key: I18nKey, vars?: Record<string, string | number>): string {
  const template = locale === "en" ? (en[key] ?? ja[key]) : ja[key];
  if (!vars) return template;
  return template.replace(/\{(\w+)\}/g, (match, name: string) => {
    const value = vars[name];
    return value === undefined ? match : String(value);
  });
}

// 下メニュー・ヘッダーのタブ表示名。
export type MainTabId = "home" | "competitions" | "coaching" | "memo" | "injury" | "me";

const tabKeys: Record<MainTabId, I18nKey> = {
  home: "nav.home",
  competitions: "nav.competitions",
  coaching: "nav.coach",
  memo: "nav.memo",
  injury: "nav.injury",
  me: "nav.me",
};

export function mainTabLabel(id: MainTabId, locale: Locale): string {
  return t(locale, tabKeys[id]);
}
