export type Meta = {
  page: number;
  per_page: number;
  total: number;
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
  category: "youth" | "student" | "open";
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
  body?: string;
  mine: boolean;
  like_count: number;
  comment_count: number;
  liked_by_me: boolean;
  saved_by_me: boolean;
  kind: "text" | "youtube" | "instagram" | "video" | "recruitment" | "practice" | "injury";
  thumbnail_url?: string;
  youtube_video_id?: string;
  instagram_type?: "p" | "reel";
  instagram_shortcode?: string;
  video_public_id?: string;
  image_public_id?: string;
  playback_url?: string;
  recruitment?: Recruitment;
  practice?: Practice;
  injury?: Injury;
  created_at: string;
  updated_at: string;
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
  score: "good" | "ok" | "bad";
  comment: string;
  created_at: string;
};

export type PracticeReputation = {
  good: number;
  ok: number;
  bad: number;
  comments?: PracticeReputationComment[];
};

export type PracticeApplication = {
  id: string;
  status: "pending" | "accepted" | "declined";
  created_at: string;
  rated?: boolean;
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
  | "injury_update";

export type AppNotification = {
  id: string;
  kind: NotificationKind;
  post_id: string;
  post_title: string;
  read: boolean;
  created_at: string;
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
  target_type: "post" | "comment";
  target_id: string;
  reason: "spam" | "harassment" | "misinfo" | "other";
  detail: string;
  status: "open" | "reviewed" | "actioned" | "dismissed";
  created_at: string;
  reporter_nickname: string | null;
  post_id: string;
  target_title: string;
  target_body: string;
  target_hidden: boolean;
};

export type Profile = {
  id: string;
  nickname: string | null;
  nickname_public: boolean;
  is_admin?: boolean;
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
};
