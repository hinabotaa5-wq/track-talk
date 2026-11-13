// 表示言語の辞書（本体 #521・#530）。対応言語は当面 ja / en。
// #530（運営者決定 2026-11-11）：en を選んだら画面上のシステム文言はすべて英語にする。
// 未訳を日本語で出すことを仕様にしない。辞書の漏れは lib/i18n-coverage.test.ts で検出する。
//
// 文言の出し方は2通り：
// - t(locale, "キー")：#521 からのキー方式（共通部分・コーチ導線）。en はキーをすべて持つ。
// - L("日本語の文言", vars)：#530 で全画面に広げた方式。日本語の文言そのものを鍵に英語辞書（i18n-en.ts）を引く。
//   いまの表示言語は LocaleProvider が setActiveLocale で決める。言語が変わると画面を描き直す（key を変える）ので、
//   描画中に呼ぶ L はいつも新しい言語を返す。

export type Locale = "ja" | "en";

// 言語設定の値を ja / en に正規化する。en 以外はすべて ja。
export function parseLocale(value: unknown): Locale {
  return value === "en" ? "en" : "ja";
}

// 表示言語の唯一の決め方：ブラウザ言語の自動判定（運営者決定 2026-11-13）。
// ログイン状態・端末保存・プロフィール locale は表示言語に使わない。
// 対応言語は ja / en のみ。en で始まる値（en, en-US, en-GB 等）だけ英語にし、それ以外は日本語。
export function detectBrowserLocale(): Locale {
  if (typeof navigator === "undefined") return "ja";
  let candidates: readonly unknown[] = [];
  try {
    candidates = [navigator.language, ...(Array.isArray(navigator.languages) ? navigator.languages : [])];
  } catch {
    return "ja";
  }
  for (const candidate of candidates) {
    if (typeof candidate !== "string" || candidate.length === 0) continue;
    return candidate.toLowerCase().startsWith("en") ? "en" : "ja";
  }
  return "ja";
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
  "profile.languageHint": "選ぶとすぐに切り替わります。「保存」でアカウントに保存します。",
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
  "notify.review_offer": "動画添削の依頼が届きました",
  "notify.review_offer_reminder": "フォーム添削の返却期限が近づいています",
  "notify.review_accepted": "コーチがフォーム添削の依頼を引き受けました",
  "notify.review_nomination_ended": "指名されたフォーム添削の依頼が終了しました",
  "notify.review_expired": "フォーム添削の依頼が期限を過ぎました",
  "notify.review_returned": "フォーム添削の返答が届きました",
  "notify.review_clarify": "添削の補足を頼まれました",
  "notify.review_clarified": "添削の補足が届きました",
  "notify.review_info_request": "コーチから依頼内容の確認が届きました",
  "notify.review_resubmitted": "差し戻した依頼が再提出されました",
  "notify.review_resubmitted_after_accept": "差し戻した依頼が再提出されました",
  "notify.review_withdrawn": "コーチが依頼を返却しました",
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
  "coach.tile.reviewing": "対応中",
  "coach.tile.consultations": "追加相談",
  "coach.tile.contracts": "契約・申込み",
  "coach.tile.supportConsults": "無料相談",
  "coach.tile.plans": "サポートプラン",
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
  "coach.reviewingTitle": "対応中",

  "coach.offers.note": "新しい依頼が入ってから90分以内（23:00–07:00 はカウントが止まります）に引き受けないと、依頼は終了します。",
  "coach.offers.empty": "いま届いている依頼はありません",
  "coach.offers.deadline": "{date}まで（あと{minutes}分）※23:00–07:00はカウント停止。過ぎると依頼は終了します",
  "coach.offers.accept": "引き受ける",
  "coach.offers.decline": "見送る",
  "coach.offers.loadError": "依頼を読み込めませんでした",
  "coach.reviewing.empty": "添削中の依頼はありません",
  "coach.reviewing.writeResponse": "返答を書く",
  "coach.reviewing.writeFollowup": "補足を書く",
  "coach.reviewing.open": "依頼を開く",
  "coach.reviewing.due": "返却期限: {date}",
  "coach.reviewing.question": "相談内容: {text}",

  "coach.detail.notFound": "依頼が見つかりません",
  "coach.detail.loadError": "読み込みに失敗しました",
  "coach.detail.due": "返却期限: {date}",
  "coach.detail.question": "相談内容: {text}",
  "coach.detail.responseTitle": "返答を書く",
  "coach.detail.contactNote": "外部の連絡先（URL・メール・SNS IDなど）は送れません。直接のやりとりに誘うと利用停止になります。",
  "coach.detail.responseNote": "返答と一緒に、ニックネーム・担当種目・記録・成績や指導経験が利用者に届きます。",
  "coach.detail.good": "良い点（1つ）",
  "coach.detail.fix1": "直す点（1つ目）",
  "coach.detail.fix2": "直す点（2つ目、なくてもよい）",
  "coach.detail.drill": "おすすめのドリル（1つ）",
  "coach.detail.demoVideo": "見本動画（なくてもよい、{seconds}秒まで）",
  "coach.detail.demoTooBig": "100MB以内の動画にしてください",
  "coach.detail.demoTooLong": "見本動画は{seconds}秒以内にしてください",
  "coach.detail.sendResponse": "返答を送る",
  "coach.detail.responseSent": "返答を送りました",
  "coach.detail.followup": "補足（1回だけ送れます）",
  "coach.detail.sendFollowup": "補足を送る",
  "coach.detail.clarificationSent": "補足を送りました",
  "coach.detail.infoSent": "差し戻しました",
  "coach.detail.clarifiedAs": "「{reason}」と言われました",
  "coach.detail.toCoach": "コーチ画面へ",
} as const;

export type I18nKey = keyof typeof ja;

// 英語辞書。#530 からはキーをすべて持つ（型で強制する）。
const en: Record<I18nKey, string> = {
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
  "profile.languageHint": "Switches immediately. Tap Save to keep it on your account.",
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
  "notify.review_offer": "New video-review request",
  "notify.review_offer_reminder": "Form review due soon",
  "notify.review_accepted": "Coach accepted the review request",
  "notify.review_nomination_ended": "Nominated review request closed",
  "notify.review_expired": "Form-review request expired",
  "notify.review_returned": "Form-review response arrived",
  "notify.review_clarify": "Asked for a review follow-up",
  "notify.review_clarified": "Review follow-up arrived",
  "notify.review_info_request": "Coach asked for request details",
  "notify.review_resubmitted": "Returned request resubmitted",
  "notify.review_resubmitted_after_accept": "Returned request resubmitted",
  "notify.review_withdrawn": "Coach returned the request",
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
  "coach.tile.plans": "Support plan",
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
  "coach.offers.deadline": "By {date} ({minutes} min left; paused 23:00–07:00). After that the request closes.",
  "coach.offers.accept": "Accept",
  "coach.offers.decline": "Decline",
  "coach.offers.loadError": "Couldn't load requests",
  "coach.reviewing.empty": "No reviews in progress",
  "coach.reviewing.writeResponse": "Write response",
  "coach.reviewing.writeFollowup": "Write follow-up",
  "coach.reviewing.open": "Open request",
  "coach.reviewing.due": "Due: {date}",
  "coach.reviewing.question": "Question: {text}",

  "coach.detail.notFound": "Request not found",
  "coach.detail.loadError": "Couldn't load. Please try again.",
  "coach.detail.due": "Due: {date}",
  "coach.detail.question": "Question: {text}",
  "coach.detail.responseTitle": "Write a response",
  "coach.detail.contactNote": "No outside contact info (URLs, emails, social IDs). Asking for direct contact results in suspension.",
  "coach.detail.responseNote": "Your nickname, events, records, results, and coaching experience will be sent with the response.",
  "coach.detail.good": "Strong point (1)",
  "coach.detail.fix1": "Fix (1st)",
  "coach.detail.fix2": "Fix (2nd, optional)",
  "coach.detail.drill": "Recommended drill (1)",
  "coach.detail.demoVideo": "Demo video (optional, up to {seconds}s)",
  "coach.detail.demoTooBig": "Please keep videos under 100MB",
  "coach.detail.demoTooLong": "Please keep demo videos under {seconds}s",
  "coach.detail.sendResponse": "Send response",
  "coach.detail.responseSent": "Response sent",
  "coach.detail.followup": "Follow-up (one time only)",
  "coach.detail.sendFollowup": "Send follow-up",
  "coach.detail.clarificationSent": "Follow-up sent",
  "coach.detail.infoSent": "Sent back for clarification",
  "coach.detail.clarifiedAs": "\"{reason}\" was requested",
  "coach.detail.toCoach": "Back to Coach",
};

// 文言を取る。英語にないキーは日本語を返す。{name} は vars で置き換える。
export function t(locale: Locale, key: I18nKey, vars?: Record<string, string | number>): string {
  const template = locale === "en" ? en[key] : ja[key];
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

// ---- #530：日本語の文言を鍵にした辞書引き ----

let activeLocale: Locale = "ja";
let english: Readonly<Record<string, string>> | null = null;

// いまの表示言語。LocaleProvider が描画のたびに決める（サーバー描画では常に ja）。
export function currentLocale(): Locale {
  return activeLocale;
}

export function setActiveLocale(locale: Locale): void {
  activeLocale = locale;
}

export function englishLoaded(): boolean {
  return english !== null;
}

export function installEnglish(dictionary: Readonly<Record<string, string>>): void {
  english = dictionary;
}

// 英語辞書は英語を選んだ人だけが読み込む（日本語の人の初回読み込みを重くしない）。
let loading: Promise<void> | null = null;
export function loadEnglish(): Promise<void> {
  if (english) return Promise.resolve();
  loading ??= import("./i18n-en").then((mod) => {
    english = mod.en;
  });
  return loading;
}

function fill(template: string, vars?: Record<string, string | number | null | undefined>): string {
  if (!vars) return template;
  return template.replace(/\{(\w+)\}/g, (match, name: string) => {
    const value = vars[name];
    return value === undefined || value === null ? match : String(value);
  });
}

// 画面の文言。ja はそのまま、en は英語辞書から引く。{name} は vars で置き換える。
// 英語辞書に無いと日本語が出てしまうので、辞書の網羅は i18n-coverage.test.ts で必ず検査する。
export function L(ja: string, vars?: Record<string, string | number | null | undefined>): string {
  if (activeLocale === "en" && english) {
    const found = english[ja];
    if (found !== undefined) return fill(found, vars);
    // 動的な文言（API のエラー文など）で辞書に無いものは、日本語のまま出さずに分かる形にする。
    if (process.env.NODE_ENV !== "production") console.warn("[i18n] missing English:", ja);
  }
  return fill(ja, vars);
}

// 日本語の文言が英語辞書にあるか（動的な文言の出し分けに使う）。
export function hasEnglish(ja: string): boolean {
  return !!english && english[ja] !== undefined;
}

// 英語の人は、辞書を当てるまで画面を隠す（日本語が一瞬見えるのを防ぐ）。3秒で必ず見せる。
// 判定はブラウザ言語だけを見る（運営者決定 2026-11-13）。描画前の同期実行でチラつきを避ける。
export const localePendingClass = "locale-pending";
export const localeStartupScript = `try{var l=navigator.language||((navigator.languages||[])[0]||"");var e=/^en/i.test(l||"");var d=document.documentElement;d.lang=e?"en":"ja";if(e){d.classList.add(${JSON.stringify(localePendingClass)});setTimeout(function(){d.classList.remove(${JSON.stringify(localePendingClass)})},3000)}}catch(e){}`;

// 曜日の短い表記（0 が日曜）。英語は1文字だと火・木などが区別できないので3文字にする。
const weekdaysEn = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"] as const;
export function weekdayLabel(index: number): string {
  return activeLocale === "en" && english ? weekdaysEn[index] : "日月火水木金土"[index];
}

// タブのタイトルの英語化（本体 #530）。タイトルは「ページ名｜TrackTalk」の形で、部分ごとに L で引く。
// 都道府県名などを埋め込んだタイトルは辞書に無いので、画面側が「日本語の部分 → 英語」を登録する（useTitleTranslation）。
const titleParts = new Map<string, string>();
export const titleChangedEvent = "tracktalk:title-parts";

export function registerTitlePart(ja: string, translated: string) {
  if (!ja || titleParts.get(ja) === translated) return;
  titleParts.set(ja, translated);
  if (typeof window !== "undefined") window.dispatchEvent(new Event(titleChangedEvent));
}

export function translateTitle(title: string): string {
  return title
    .split("｜")
    .map((part) => titleParts.get(part) ?? L(part))
    .join(" | ");
}

// 文の途中に要素（強調した数字やリンク）を挟むときに、差し込み位置の前後の文言を返す。
// 例：L("まであと {days} 日") を英語で "{days} days to go" にすると、語順が変わっても正しく並ぶ。
export function splitAround(ja: string, slot: string): [string, string] {
  const text = L(ja);
  const marker = `{${slot}}`;
  const index = text.indexOf(marker);
  return index < 0 ? [text, ""] : [text.slice(0, index), text.slice(index + marker.length)];
}

// API が日本語で作った日付や範囲（「2026年11月10日」「9月11日〜19日」「25.0〜25.5cm」など）を、英語表示のときは数字の形にする。
export function localizeJaDate(text: string): string {
  if (activeLocale !== "en" || !english) return text;
  return text
    .replace(/(\d{4})年(\d{1,2})月(\d{1,2})日/g, "$2/$3/$1")
    .replace(/(\d{4})年(\d{1,2})月/g, "$2/$1")
    .replace(/(\d{1,2})月(\d{1,2})日/g, "$1/$2")
    .replace(/(\d{1,2})日/g, "$1")
    .replace(/(\d{4})年/g, "$1")
    .replace(/〜/g, "–");
}
