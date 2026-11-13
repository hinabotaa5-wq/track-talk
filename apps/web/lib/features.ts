// 提供前の機能を隠すためのフラグ。値は "1" か "true" のときだけ有効にする。
export function flagEnabled(value: string | undefined): boolean {
  return ["1", "true", "yes", "on"].includes((value ?? "").trim().toLowerCase());
}

// 明示OFFのときだけ隠すフラグ。未設定・空のときは表示する。
export function flagEnabledDefaultOn(value: string | undefined): boolean {
  return !["0", "false", "no", "off"].includes((value ?? "").trim().toLowerCase());
}

// 現在は募集のみ。利用者向け画面は準備中にし、/coaching・/coach・/coach/apply・/coach/about
// から説明・応募・審査状況の確認を行える。APIのCOACHING_ENABLEDと揃えること。
export const coachingEnabled = flagEnabled(process.env.NEXT_PUBLIC_COACHING_ENABLED);

// 有料の1対1のやりとり(契約のやりとり欄・追加相談・添削の確認質問と補足)。電気通信事業の届出が
// 受理されるまで立てない。空の間は読むだけにして、送る欄は出さない。APIのCOACHING_MESSAGING_ENABLEDと揃えること。
export const coachingMessagingEnabled = flagEnabled(process.env.NEXT_PUBLIC_COACHING_MESSAGING_ENABLED);

// コーチ一覧(/coaches とコーチの詳細)の公開。coachingEnabled が立っていても、空の間は一覧を準備中にする。
// 動画添削の申込み(/coaching/new)は指名候補の API を使うので、このフラグでは止めない。APIのCOACHING_COACH_LIST_ENABLEDと揃えること。
export const coachingCoachListEnabled = flagEnabled(process.env.NEXT_PUBLIC_COACHING_COACH_LIST_ENABLED);

// 動画添削のあとの追加質問（1回500円の都度払いのみ）。開けるのは運営者の指示があってから。
// API の COACHING_CONSULTATION_ENABLED と揃えること。空の間は送る欄を出さず、届いた内容と下書きは読める。
export const coachingConsultationEnabled = flagEnabled(process.env.NEXT_PUBLIC_COACHING_CONSULTATION_ENABLED);

export const consultationClosedNote = "追加質問の受付は準備中です。届いた内容は読めます。";

export const messagingClosedNote = "メッセージのやりとりは準備中です。届いた内容は読めます。";

// 感覚メモを保存順に6個ずつまとめるふりかえり(AI)。感覚メモを Claude(Anthropic)に送る(プライバシーポリシーの委託先に書いてある)。
// API 側の SENSE_SUMMARY_ENABLED と揃えること。
export const senseSummaryEnabled = flagEnabled(process.env.NEXT_PUBLIC_SENSE_SUMMARY_ENABLED);

// サポートプランの入口の一覧（コーチタブの「サポートプラン」側）。空の間は「12月開始予定」を出す。
// 開始日に立てて一覧へ切り替える（日付を画面に直書きしない。開始日と開放の手順は運営者が決める）。
// 一覧の API は /v1/coaches/support-plan（COACHING_COACH_LIST_ENABLED の側）。
export const supportPlanListEnabled = flagEnabled(process.env.NEXT_PUBLIC_SUPPORT_PLAN_LIST_ENABLED);

// 海外コーチの指導希望の集計カード（コーチタブの独立カード、本体 #542）。
// 集計が目的なので、デプロイ後は表示される状態で出す（既定ON）。
// 正式開始のタイミングで運営者が消すときは NEXT_PUBLIC_OVERSEAS_INTEREST_ENABLED に
// 明示OFF（0/false/no/off）を設定する。
export const overseasInterestEnabled = flagEnabledDefaultOn(process.env.NEXT_PUBLIC_OVERSEAS_INTEREST_ENABLED);
