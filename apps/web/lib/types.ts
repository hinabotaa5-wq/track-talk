export type Meta = {
  page: number;
  per_page: number;
  total: number;
  has_more?: boolean;
  next_cursor?: string;
};

export type MajorHost = {
  kind: "worlds" | "olympics";
  year: number;
  name: string;
  city: string;
  country: string;
  venue: string;
  starts_on: string;
  ends_on: string;
  schedule_label: string;
  image_url: string;
  page_url: string;
};

export type DomesticMajor = {
  slug: string;
  year: number;
  name: string;
  category: "youth" | "student" | "open" | "distance";
  venue: string;
  starts_on: string;
  ends_on: string;
  schedule_label: string;
  site_url: string;
};

export type Competition = {
  id: string;
  name: string;
  starts_on: string;
  ends_on: string | null;
  location: string;
  prefecture: string;
  season_year: number;
  organizer_url: string;
  source_url: string;
  link_label: string;
  guidelines_url: string;
};

export type Post = {
  id: string;
  title: string;
  author_name?: string;
  author_post_master?: boolean;
  body?: string;
  // 一覧カードに出す詳細の冒頭。写真付きの文章や動画URLの投稿にも使う。
  excerpt?: string;
  mine: boolean;
  like_count: number;
  comment_count: number;
  liked_by_me: boolean;
  saved_by_me: boolean;
  kind: "text" | "youtube" | "instagram" | "x" | "video" | "recruitment" | "practice" | "injury" | "meet";
  thumbnail_url?: string;
  youtube_shorts?: boolean;
  youtube_video_id?: string;
  source_title?: string;
  instagram_type?: "p" | "reel";
  instagram_shortcode?: string;
  x_post_id?: string;
  video_public_id?: string;
  image_public_id?: string;
  playback_url?: string;
  recruitment?: Recruitment;
  practice?: Practice;
  injury?: Injury;
  created_at: string;
  updated_at: string;
};

// 今日開催中の大会の実況スレッド。API /v1/meet-threads/today が返す。
export type MeetThread = {
  meet_key: string;
  name: string;
  date_label: string;
  post_id: string;
  comment_count: number;
  href: string;
  // いま実施中の種目(例: 「女子100m予選」)。タイムテーブルが無い大会では空。
  now_running: string[];
  // 大会タブの一覧のどの大会か。国内主要大会は major_slug と major_year、国際大会は competition_id(もう一方は空)。
  major_slug: string;
  major_year: number;
  competition_id: string;
};

export type Injury = {
  body_part: string;
  body_part_label: string;
  stage?: string;
  stage_label?: string;
  updates?: InjuryUpdate[];
};

export type InjuryUpdate = {
  id: string;
  stage: string;
  stage_label: string;
  body: string;
  created_at: string;
};

export type RecruitmentPhoto = {
  public_id: string;
  url: string;
};

export type Recruitment = {
  prefectures: string[];
  venue: string;
  schedule: string;
  gender_ratio: string;
  average_age: string;
  recruit_age: string;
  links: string[];
  photos: RecruitmentPhoto[];
};

export type PracticeReputationComment = {
  id: string;
  score: "good" | "ok" | "bad" | "no_show";
  comment: string;
  created_at: string;
};

export type PracticeReputation = {
  good: number;
  ok: number;
  bad: number;
  comments?: PracticeReputationComment[];
};

export type PracticeAttendance = {
  total: number;
  no_show: number;
};

export type PracticeApplication = {
  id: string;
  status: "pending" | "accepted" | "declined";
  created_at: string;
  rated?: boolean;
  attendance?: PracticeAttendance;
};

export type Practice = {
  venue: string;
  starts_at: string;
  event: string;
  event_other?: string;
  event_label: string;
  capacity: number;
  accepted_count: number;
  status: "open" | "full" | "dissolved";
  dissolved_at?: string;
  // 練習の日時を過ぎてから解散した（開かれた）練習で、評価できるとき true。日時より前の解散は中止。
  ratable?: boolean;
  my_application?: PracticeApplication["status"];
  my_application_id?: string;
  rated_host?: boolean;
  applications?: PracticeApplication[];
  reputation?: PracticeReputation;
};

export type Comment = {
  id: string;
  post_id: string;
  body: string;
  mine: boolean;
  author_name?: string;
  parent_id?: string | null;
  reply_to_id?: string | null;
  reply_to_name?: string | null;
  reply_to_removed?: boolean;
  created_at: string;
  updated_at: string;
};

export type NotificationKind =
  | "recruitment_post"
  | "comment"
  | "comment_reply"
  | "practice_application"
  | "practice_accepted"
  | "practice_rate"
  | "injury_update"
  | "review_offer"
  | "review_expired"
  | "review_returned"
  | "review_clarify"
  | "review_clarified"
  | "review_helpful_ask"
  | "message";

// 添削の通知（review_*）は投稿にひもづかないので post_id が null になる。
// message はパーソナル指導や支払い、月1回のお知らせなど。見出し・本文・開く場所をそのまま持つ。
export type AppNotification = {
  id: string;
  kind: NotificationKind;
  post_id: string | null;
  post_title: string | null;
  review_request_id: string | null;
  title: string | null;
  body: string | null;
  url: string | null;
  read: boolean;
  created_at: string;
};

// 通知の種類ごとの受け取り設定。in_app はベルの一覧、push は通知をオンにした端末。
export type NotificationCategory = "comment" | "practice" | "recruitment" | "injury" | "review" | "coaching" | "monthly" | "announcement";

export type NotificationSetting = {
  category: NotificationCategory;
  in_app: boolean;
  push: boolean;
};

export type Inquiry = {
  id: string;
  category: "bug" | "account" | "request" | "other";
  subject: string;
  body: string;
  contact_email: string | null;
  created_at: string;
};

export type AdminInquiry = Inquiry & {
  user_nickname: string | null;
};

export type AdminReport = {
  id: string;
  target_type: "post" | "comment" | "injury_note" | "review_response" | "coaching_message" | "coaching_consultation" | "practice_rating";
  target_id: string;
  reason: "spam" | "harassment" | "misinfo" | "other";
  detail: string;
  status: "open" | "reviewed" | "actioned" | "dismissed";
  created_at: string;
  reporter_nickname: string | null;
  // 保護者のページからの通報。reporter_nickname は子のニックネーム。
  by_guardian: boolean;
  // やりとり欄の通報のとき、その契約。中身は返らないので、開いて確かめる。
  coaching_contract_id: string;
  post_id: string;
  target_title: string;
  target_body: string;
  target_hidden: boolean;
  // ケガ部の一言のとき、どのグループの一言か。
  injury_part: string;
};

export type Profile = {
  id: string;
  nickname: string | null;
  nickname_public: boolean;
  // 登録のときに選ぶ種目(ケガ部の種目と同じコード)。以前から使っている人は、選ぶまで null。
  event?: string | null;
  post_master_badge_public?: boolean;
  is_admin?: boolean;
  // 管理者・承認済みコーチで、いまのログインに二段階認証が足りないとき true。
  mfa_required?: boolean;
  // いまの版の説明で、海外の事業者に預けることに同意済みなら true。
  overseas_consent?: boolean;
  created_at: string;
  updated_at?: string;
};

export type ApiError = {
  error: {
    code: string;
    message: string;
    fields?: Record<string, string>;
  };
};

export type Product = {
  id: string;
  kind: "shoe" | "spike";
  brand: string;
  name: string;
  image_url: string;
  product_url: string;
  discontinued: boolean;
  review_count: number;
  avg_rating: number;
};

export type ProductReview = {
  id: string;
  product_id: string;
  rating: number;
  body: string;
  mine: boolean;
  created_at: string;
  // 書いた人の属性(任意)。足のサイズは「25.0〜25.5cm」のような幅でだけ返る。
  event?: string | null;
  event_label?: string | null;
  level?: string | null;
  level_label?: string | null;
  foot_size_label?: string | null;
  foot_width?: string | null;
  foot_width_label?: string | null;
  usage?: string | null;
  usage_label?: string | null;
};

export type GuardianStatus = "pending" | "consented" | "declined" | "revoked";

export type GuardianLimitYen = 1000 | 3000 | 5000 | null;

export type CoachingGuardian = {
  status: GuardianStatus;
  monthly_limit_yen: GuardianLimitYen;
  allow_publish: boolean;
  consented_at: string | null;
  // 同意のリンクを送った保護者のメールアドレス。本人のアカウントにだけ返す。
  guardian_email?: string | null;
};

export type CoachingMe = {
  birth_month: string | null;
  is_minor: boolean | null;
  guardian: CoachingGuardian | null;
  push_devices: number;
  demo: boolean;
};

export type GuardianPublic = CoachingGuardian & {
  guardian_name: string | null;
  relationship: "father" | "mother" | "other" | null;
  limit_options: GuardianLimitYen[];
  default_limit_yen: number;
};

export type CoachTier = "coach";
export type CoachEvent =
  | "100m"
  | "200m"
  | "400m"
  | "800m"
  | "1500m"
  | "5000m"
  | "10000m"
  | "110mh"
  | "100mh"
  | "400mh"
  | "3000msc"
  | "high-jump"
  | "pole-vault"
  | "long-jump"
  | "triple-jump"
  | "shot-put"
  | "discus"
  | "hammer"
  | "javelin"
  | "race-walk"
  | "relay"
  | "heptathlon"
  | "decathlon";
export type CoachStatus = "pending" | "approved" | "rejected" | "suspended" | "withdrawn";

export type CoachApplication = {
  name?: string;
  service_types: CoachingServiceType[];
  tier: CoachTier;
  events: CoachEvent[];
  best_record: string;
  achievements: string;
  years: number;
  sns_urls: string[];
  status: CoachStatus;
  applied_at: string;
  reviewed_at: string | null;
  on_duty: boolean;
  daily_cap: number;
  // 顔写真。登録していなければ空。
  photo_url: string;
  // いまの版のパーソナル指導の誓約をしたか。
  pledged: boolean;
  pledged_current?: boolean;
  pledge_version?: number | null;
  // 報酬の受け取り口座の登録が済み、受付やパーソナル指導の承諾ができるか。
  payouts_ready: boolean;
};

// コーチへの報酬と、月ごとの送金。
export type CoachPayoutStatus = "pending" | "paid" | "failed";
export type CoachPayout = {
  id: string;
  month: string;
  amount_yen: number;
  status: CoachPayoutStatus;
  paid_at: string | null;
};
export type CoachEarning = {
  id: string;
  source: "review" | "personal" | "consultation";
  gross_yen: number;
  share_percent: number;
  amount_yen: number;
  paid: boolean;
  created_at: string;
};
export type CoachEarnings = {
  // false のあいだは支払いなしのデモ版。
  payments: boolean;
  stripe_linked: boolean;
  payouts_enabled: boolean;
  review_share: number;
  personal_fee: number;
  min_payout_yen: number;
  this_month_yen: number;
  unpaid_yen: number;
  payouts: CoachPayout[];
  earnings: CoachEarning[];
};
export type AdminPayoutCoach = {
  profile_id: string;
  nickname: string | null;
  tier: CoachTier;
  stripe_linked: boolean;
  payouts_enabled: boolean;
  unpaid_yen: number;
};
export type AdminCoachPayout = CoachPayout & {
  coach_id: string | null;
  nickname: string | null;
  failure: string | null;
  created_at: string;
};

// パーソナル指導のプラン。period は2週間か1か月、single は7日以内に終わる単発、
// trial はそのコーチの指導をはじめて受ける人だけが申し込める7日のお試し。
export type CoachingServiceType = "video_review" | "competition_support" | "in_person";
export type CoachingPlanKind = "period" | "single" | "trial";
// プランで「何が返ってくるか」。null か空なら、コーチが書いていない。
// video_reviews は動画への返答の回数（0 は回数を決めない）、menu_updates は練習メニューの作り方、reply_formats は返し方。
export type PlanMenuUpdates = "none" | "once" | "weekly";
export type PlanReplyFormat = "text" | "video";
// start_on_first_video は、期間を最初の動画が届いた日から数えるか。
export type PlanIncludes = {
  competition_name?: string | null;
  competition_date?: string | null;
  support_start_date?: string | null;
  call_count?: number;
  call_minutes?: number;
  venue_area?: string | null;
  venue_name?: string | null;
  session_count?: number;
  session_minutes?: number;
  travel_fee_yen?: number;
  max_participants?: number;
  // 未設定は以前からある指名プラン。
  service_type?: CoachingServiceType | null;
  additional_consultation_price_yen?: number | null;
  video_reviews: number | null;
  menu_updates: PlanMenuUpdates | null;
  reply_formats: PlanReplyFormat[];
  start_on_first_video: boolean;
};
export type CoachingPlan = PlanIncludes & {
  id: string;
  kind: CoachingPlanKind;
  title: string;
  description: string;
  duration_days: number;
  price_yen: number;
  capacity: number;
  published: boolean;
  // コーチ一覧と申し込む画面だけ: 期間中の人数が上限に届いているか。
  full?: boolean;
};

// パーソナル指導の申し込み。applied: コーチの返事待ち、active: 期間中、ended: 終わった、
// declined: コーチが断った、expired: 返事・同意・支払いがないまま期限が過ぎた、cancelled: 返事の前に取り下げた、
// refunded: 返金して途中で終わった。
export type CoachingContractStatus =
  | "applied"
  | "accepted"
  | "active"
  | "suspended"
  | "ended"
  | "refunded"
  | "declined"
  | "guardian_declined"
  | "expired"
  | "cancelled";
export type CoachingContract = PlanIncludes & {
  preferred_dates?: string[];
  fulfillment_version?: number;
  fulfillment_state?: string;
  // 最初の動画から数えるプランで、動画が届いて期間を数え始めた時刻。
  clock_started_at: string | null;
  id: string;
  plan_id: string;
  plan_kind: CoachingPlanKind;
  plan_title: string;
  plan_description?: string | null;
  duration_days: number;
  price_yen: number;
  event: CoachEvent;
  goal: string;
  current_record: string;
  message: string;
  minor: boolean;
  // Stripe の支払いがある申し込み。false はデモ版（支払いなし）。
  payment_required: boolean;
  status: CoachingContractStatus;
  respond_by: string;
  responded_at: string | null;
  // コーチが承諾したあと、保護者の同意や支払いを済ませる期限。
  start_by: string | null;
  suspended_at: string | null;
  starts_at: string | null;
  ends_at: string | null;
  created_at: string;
  // 保存期間（期間の終わりから1年）を過ぎて、やりとりを消した日時。
  messages_purged_at: string | null;
  role?: "requester" | "coach" | "guardian";
  rating?: { stars: number; rated_at: string } | null;
  coach?: { id: string; nickname: string | null; photo_url: string };
  requester?: { nickname: string | null };
  // 支払い画面を開いただけのものは含まない。
  payment?: CoachingPayment | null;
  // 保護者のページだけ: 保護者がこの申し込みに同意したか。
  guardian_consented?: boolean;
};

export type CoachingPayment = {
  status: "paid" | "refunded" | "partially_refunded";
  amount_yen: number;
  refunded_yen: number;
  paid_at: string;
  refunded_at: string | null;
};

// 運営のパーソナル指導の一覧。会話の中身は含まない。
export type AdminCoachingContract = {
  fulfillment_version?: number;
  fulfillment_state?: string;
  delivered_units?: number;
  total_units?: number;
  id: string;
  plan_title: string;
  plan_kind: CoachingPlanKind;
  duration_days: number;
  price_yen: number;
  minor: boolean;
  status: CoachingContractStatus;
  starts_at: string | null;
  ends_at: string | null;
  created_at: string;
  payment_required: boolean;
  // 空文字はまだ支払いがない。
  payment_status: "" | CoachingPayment["status"];
  refunded_yen: number;
  requester_nickname: string | null;
  coach_nickname: string | null;
  report_count: number;
  requester_blocked: number;
  coach_blocked: number;
};

export type CoachingBlockedAccount = {
  profile_id: string;
  nickname: string | null;
  blocked: number;
  last_blocked_at: string;
};

export type CoachingAccessLog = {
  id: string;
  admin_id: string;
  report_id: string;
  opened_at: string;
  admin_nickname: string | null;
};

// コーチが送る練習メニュー。
export type PlanMenu = { title: string; days: { day: string; content: string }[] };

// パーソナル指導のやりとり欄の1通。
export type CoachingMessage = {
  review_submission_id?: string | null;
  id: string;
  sender_role: "requester" | "coach" | "guardian";
  mine: boolean;
  sender_active: boolean;
  body: string;
  video_url: string | null;
  thumbnail_url: string | null;
  menu: PlanMenu | null;
  created_at: string;
};

// 申し込む画面に出すプランとコーチ。
export type PublicCoachingPlan = {
  // false のあいだは支払いなしのデモ版。
  payments: boolean;
  plan: CoachingPlan;
  coach: { id: string; nickname: string | null; photo_url: string; tier: CoachTier; events: CoachEvent[] };
};

export type ReviewFocus = "start" | "acceleration" | "top-speed" | "takeoff" | "arm" | "contact" | "rhythm" | "other";
export type ReviewStatus = "awaiting_payment" | "queued" | "offered" | "in_review" | "returned" | "clarifying" | "expired";

export type ClarifyReason = "term" | "drill" | "fix" | "other";
export type HelpfulAnswer = "yes" | "some" | "no";

// コーチの返答。運営が隠したものは hidden だけが返る。
export type ReviewResponse =
  | { hidden: true; created_at: string }
  | {
      hidden: false;
      good_point: string;
      fix_points: string[];
      drill: string;
      demo_video_url: string | null;
      created_at: string;
      clarify_reason: ClarifyReason | null;
      clarify_body?: string | null;
      clarify_requested_at: string | null;
      clarification: string | null;
      clarified_at: string | null;
      helpful_asked: boolean;
    };

export type ReviewRating = {
  stars: number | null;
  helpful: HelpfulAnswer | null;
};

export type ReviewRequest = {
  tickets?: { quantity: number; status: "reserved" | "consumed" | "released"; nominal_yen: number };
  id: string;
  tier: CoachTier;
  event: CoachEvent;
  focus: ReviewFocus[];
  question?: string;
  status: ReviewStatus;
  due_at: string;
  created_at: string;
  video_url: string;
  thumbnail_url: string;
  // 詳細を開いたときだけ付く。
  response?: ReviewResponse | null;
  rating?: ReviewRating | null;
  // 返答が届いたあとにだけ入る。
  coach?: ReviewCoach | null;
  // 詳細を開いたときだけ付く。支払い画面を開いただけのものは含まない。
  payment?: ReviewPayment | null;
  // 送った直後と、支払い画面を開き直したときだけ付く。
  checkout_url?: string;
};

// 支払い方法。card は Apple Pay・Google Pay を含む（仮押さえして返却で確定）。paypay はその場で払う。
export type ReviewPayMethod = "card" | "paypay";

export type ReviewPayment = {
  method: ReviewPayMethod;
  amount_yen: number;
  // authorized: 仮押さえ中。captured: 確定。canceled: 取り消し。refunded: 返金。
  status: "authorized" | "captured" | "canceled" | "refunded";
  captured_at: string | null;
  canceled_at: string | null;
  refunded_at: string | null;
};

export type CoachProbationResponse = {
  request_id: string;
  coach_id: string;
  coach_nickname: string | null;
  nth: number;
  tier: CoachTier;
  event: CoachEvent;
  good_point: string;
  fix_points: string[];
  drill: string;
  clarification: string | null;
  created_at: string;
  hidden_at: string | null;
};

// コーチに見せる依頼。動画は引き受けたあとにだけ付く。
export type CoachReviewRequest = Omit<ReviewRequest, "video_url" | "thumbnail_url" | "rating"> & {
  video_url?: string;
  thumbnail_url?: string;
};

export type ReviewOffer = {
  id: string;
  expires_at: string;
  request: CoachReviewRequest;
};

export type CoachRequests = {
  offers: ReviewOffer[];
  reviewing: CoachReviewRequest[];
};

export type ReviewEstimate = {
  due_at: string;
  coaches_on_duty: boolean;
  // false のあいだは支払いなしのデモ版。
  payments: boolean;
  price_yen: number;
};

export type DemoCode = {
  code: string;
  created_at: string;
  used_at: string | null;
  used_by_nickname: string | null;
};

export type AdminCoachApplication = Omit<CoachApplication, "status"> & {
  profile_id: string;
  status: CoachStatus;
  nickname: string | null;
  // 打診の順番から外す判定に使う★（運営が戻したあとのものだけ）。
  rating_count: number;
  rating_average: number;
  rating_paused: boolean;
  photo_url: string;
};

// 依頼する前に見られるコーチ一覧の1人。★はこれまでの全部。
export type PublicCoach = {
  service_types: CoachingServiceType[];
  id: string;
  nickname: string | null;
  photo_url: string;
  tier: CoachTier;
  events: CoachEvent[];
  best_record: string;
  achievements: string;
  years: number;
  sns_urls: string[];
  rating_count: number;
  rating_average: number;
  // パーソナル指導の公開中のプラン。安い順。
  plans: CoachingPlan[];
};

// コーチが受けた★の件数と平均。paused は、評価が低くて依頼が回ってこない状態。
export type CoachRatingSummary = {
  count: number;
  average: number;
  paused: boolean;
};

// 返答と一緒に利用者へ見せるコーチのプロフィール。
export type ReviewCoach = {
  id?: string;
  nickname: string | null;
  photo_url: string;
  tier: CoachTier;
  events: CoachEvent[];
  best_record: string;
  achievements: string;
  years: number;
};
