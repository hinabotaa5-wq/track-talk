import Link from "next/link";
import type { Post } from "@/lib/types";
import { formatDate } from "@/lib/format";
import { PosterFallback } from "@/components/poster-card";
import { InstagramThumb } from "@/components/instagram-thumb";
import { isInstagramShortcode } from "@/lib/instagram";

export const feedGridClass = "columns-2 gap-3";

const cardClass =
  "group mb-3 block w-full break-inside-avoid overflow-hidden rounded-2xl border border-white/20 glass bg-white/10 shadow-md backdrop-blur-xs transition-all duration-300 ease-out hover:-translate-y-1 hover:shadow-xl hover:border-white/40 hover:shadow-blue-950/30 active:scale-[0.98]";

// 空の見出しを投稿詳細に表示しない。
export function hasFeedTitle(title: string) {
  return title.trim() !== "";
}

// タイトルが空のときは空のまま返す(「（質問なし）」のような代わりの文字は出さない)。
export function feedTitle(title: string) {
  return title.trim();
}

function CardCaption({ text }: { text: string }) {
  return (
    <div className="feed-card-caption absolute inset-x-0 bottom-0 isolate px-3.5 pb-3 pt-8">
      <span aria-hidden="true" className="feed-card-caption-background pointer-events-none absolute inset-0 -z-10"
        style={{
          background: "linear-gradient(to top, rgb(0 0 0 / 0.85), rgb(0 0 0 / 0.5) 65%, transparent)",
          backdropFilter: "blur(8px)", WebkitBackdropFilter: "blur(8px)",
          maskImage: "linear-gradient(to top, black 65%, transparent)",
          WebkitMaskImage: "linear-gradient(to top, black 65%, transparent)",
        }} />
      <p className="relative line-clamp-3 whitespace-pre-wrap [overflow-wrap:anywhere] text-sm font-medium leading-snug text-white">{text}</p>
    </div>
  );
}

function KindBadge({ children }: { children: string }) {
  return (
    <span className="w-fit rounded-full glass bg-white/12 backdrop-blur-md px-2.5 py-0.5 text-xs font-semibold text-slate-200 shadow-xs border border-white/20">
      {children}
    </span>
  );
}

export function FeedCard({ post, returnTo }: { post: Post; returnTo?: "me" }) {
  const title = feedTitle(post.title);
  const description = feedTitle(post.body ?? "") || feedTitle(post.excerpt ?? "") || title;
  const thumbnail = post.thumbnail_url || (post.kind === "youtube" && post.youtube_video_id
    ? `https://i.ytimg.com/vi/${post.youtube_video_id}/hqdefault.jpg` : "");
  const href = returnTo === "me" ? `/posts/${post.id}?from=me` : `/posts/${post.id}`;
  if (post.kind === "practice") {
    const info = post.practice;
    return (
      <Link
        href={href}
        className={cardClass}
      >
        <div className="flex aspect-[4/3] flex-col justify-center gap-2 bg-gradient-to-br from-white/10 to-sky-400/10 p-3.5">
          {info ? (
            <div className="space-y-2 text-xs leading-snug text-slate-200">
              <p className="line-clamp-1 text-sm font-bold text-slate-100 transition-colors group-hover:text-blue-400">{info.event_label}</p>
              <p>日付　{formatDate(info.starts_at)}</p>
              <p>時間　{formatPracticeTime(info.starts_at)}</p>
              <p className="line-clamp-1">競技場　{info.venue}</p>
              <p className="font-semibold">募集 {info.accepted_count}/{info.capacity}人</p>
            </div>
          ) : (
            <p className="line-clamp-3 break-words text-sm font-bold text-slate-100 group-hover:text-blue-400">{title}</p>
          )}
        </div>
      </Link>
    );
  }
  if (post.kind === "injury") {
    const info = post.injury;
    return (
      <Link href={href} className={cardClass}>
        <div className="flex aspect-[4/3] flex-col justify-between bg-gradient-to-br from-white/10 to-rose-400/10 p-3.5">
          <div className="flex flex-wrap items-center gap-1.5">
            {info ? (
              <>
                <span className="w-fit rounded-full bg-white/12 px-2.5 py-0.5 text-xs font-semibold text-slate-200 border border-white/20">
                  {info.body_part_label}
                </span>
                {info.stage_label ? (
                  <span className="w-fit rounded-full bg-emerald-950/70 px-2.5 py-0.5 text-xs font-bold text-emerald-300 border border-emerald-800/60">
                    {info.stage_label}
                  </span>
                ) : null}
              </>
            ) : null}
          </div>
          <p className="line-clamp-4 break-words text-sm font-bold leading-snug text-slate-100 transition-colors group-hover:text-blue-400">
            {title}
          </p>
          <div className="flex items-center justify-between border-t border-white/20 pt-1 text-xs font-mono text-slate-300">
            <span>TrackTalk</span>
            <span className="font-semibold text-rose-300">わかる {post.like_count}</span>
          </div>
        </div>
      </Link>
    );
  }
  const hasThumbnail = Boolean(thumbnail || (post.kind === "instagram" && isInstagramShortcode(post.instagram_shortcode ?? "")));
  return (
    <Link href={href} aria-label={description ? undefined : "投稿の詳細を開く"}
      className={`${cardClass}${hasThumbnail ? " feed-card-has-thumbnail" : ""}${post.kind === "text" ? " feed-card-text" : ""}`}>
      <div className={`feed-card-thumbnail relative overflow-hidden bg-black/25 ${thumbnail || post.kind === "instagram" ? "" : "aspect-[4/3]"}`}>
        {post.kind === "recruitment" ? (
          <span className="absolute top-2.5 left-2.5 z-10">
            <KindBadge>クラブ</KindBadge>
          </span>
        ) : post.kind === "instagram" ? (
          <span className="absolute top-2.5 left-2.5 z-10">
            <KindBadge>Instagram</KindBadge>
          </span>
        ) : null}
        {thumbnail ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img src={thumbnail} alt="" loading="lazy" decoding="async" className="block h-auto w-full" />
        ) : post.kind === "instagram" && (post.instagram_type === "p" || post.instagram_type === "reel") && isInstagramShortcode(post.instagram_shortcode ?? "") ? (
          <InstagramThumb type={post.instagram_type} shortcode={post.instagram_shortcode ?? ""} title={description || "Instagram"} />
        ) : (
          <PosterFallback title={description} />
        )}
        {hasThumbnail && description ? <CardCaption text={description} /> : null}
      </div>
    </Link>
  );
}

function formatPracticeTime(iso: string) {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "";
  return new Intl.DateTimeFormat("ja-JP", {
    timeZone: "Asia/Tokyo",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}
