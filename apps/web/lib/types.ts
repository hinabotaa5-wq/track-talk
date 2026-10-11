export type Meta = {
  page: number;
  per_page: number;
  total: number;
};

export type Author = {
  id: string;
  nickname: string;
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
};

export type Post = {
  id: string;
  title: string;
  body?: string;
  body_preview?: string;
  author: Author;
  like_count: number;
  comment_count: number;
  liked_by_me: boolean;
  saved_by_me: boolean;
  kind: "text" | "youtube" | "video";
  thumbnail_url?: string;
  youtube_video_id?: string;
  video_public_id?: string;
  image_public_id?: string;
  playback_url?: string;
  created_at: string;
  updated_at: string;
};

export type Comment = {
  id: string;
  post_id: string;
  body: string;
  author: Author;
  created_at: string;
  updated_at: string;
};

export type Inquiry = {
  id: string;
  category: "bug" | "account" | "request" | "other";
  subject: string;
  body: string;
  created_at: string;
};

export type Profile = {
  id: string;
  nickname: string;
  bio: string;
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
