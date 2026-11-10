import { L } from "@/lib/i18n";
import { useState } from "react";
import Link from "next/link";
import type { Post } from "@/lib/types";
import { formatDate } from "@/lib/format";
import { PosterFallback } from "@/components/poster-card";
import { InstagramThumb } from "@/components/instagram-thumb";
import { isInstagramShortcode } from "@/lib/instagram";
import { CardPending } from "@/components/card-pending";
import { prefectureName } from "@/lib/prefectures";

export const feedGridClass = "columns-2 gap-3";

const cardClass =
  "group relative mb-3 block w-full break-inside-avoid overflow-hidden rounded-2xl border border-white/20 glass bg-white/10 shadow-md backdrop-blur-xs transition-all duration-300 ease-out hover:-translate-y-1 hover:shadow-xl hover:border-white/40 hover:shadow-blue-950/30 active:scale-[0.98]";

// 空の見出しを投稿詳細に表示しない。
export function hasFeedTitle(title: string) {
  return title.trim() !== "";
}

// タイトルが空のときは空のまま返す(「（質問なし）」のような代わりの文字は出さない)。
export function feedTitle(title: string) {
  return title.trim();
}

// 横長サムネイルは高さが低く、詳細文の帯が画像の大半を覆ってしまうため、
// 帯の上余白を小さくして画像の下端に寄せ、横長のときは1行に収める。
function CardCaption({ text, landscape }: { text: string; landscape: boolean }) {
  return (
    <div className="feed-card-caption absolute inset-x-0 bottom-0 isolate px-3 pb-1.5 pt-2">
      <span aria-hidden="true" className="feed-card-caption-background pointer-events-none absolute inset-0 -z-10"
        style={{
          background: "rgb(0 0 0 / 0.6)",
          backdropFilter: "blur(8px)", WebkitBackdropFilter: "blur(8px)",
          maskImage: "linear-gradient(to top, black calc(100% - 0.5rem), transparent)",
          WebkitMaskImage: "linear-gradient(to top, black calc(100% - 0.5rem), transparent)",
        }} />
      <p className={`relative ${landscape ? "line-clamp-1" : "line-clamp-3"} whitespace-pre-wrap [overflow-wrap:anywhere] text-sm font-medium leading-snug text-pure-white`}>{text}</p>
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
  const [landscapeSrc, setLandscapeSrc] = useState("");
  const title = feedTitle(post.title);
  const description = feedTitle(post.body ?? "") || feedTitle(post.excerpt ?? "") || title;
  const thumbnail = post.thumbnail_url || (post.kind === "youtube" && post.youtube_video_id
    ? `https://i.ytimg.com/vi/${post.youtube_video_id}/hqdefault.jpg` : "");
  const href = returnTo === "me" ? `/posts/${post.id}?from=me` : `/posts/${post.id}`;
  if (post.kind === "practice") {
    const info = post.practice;
    return (
      <Link
        prefetch={false}
        href={href}
        data-pending-link=""
        className={cardClass}
      >
        <div className="flex aspect-[4/3] flex-col justify-center gap-2 bg-blue-500/10 p-3.5">
          {info ? (
            <div className="space-y-2 text-xs leading-snug text-slate-200">
              <p className="line-clamp-1 text-sm font-bold text-slate-100 transition-colors group-hover:text-blue-400">{L(info.event_label)}</p>
              <p>{L("日付　{v0}", { v0: formatDate(info.starts_at) })}</p>
              <p>{L("時間　{v0}", { v0: formatPracticeTime(info.starts_at) })}</p>
              <p className="line-clamp-1">{L("競技場　{v0}{venue}", { v0: info.prefecture ? `${prefectureName(info.prefecture)} ` : "", venue: info.venue })}</p>
              <p className="font-semibold">{L("募集 {accepted_count}/{capacity}人", { accepted_count: info.accepted_count, capacity: info.capacity })}</p>
            </div>
          ) : (
            <p className="line-clamp-3 break-words text-sm font-bold text-slate-100 group-hover:text-blue-400">{title}</p>
          )}
        </div>
        <CardPending />
      </Link>
    );
  }
  if (post.kind === "injury") {
    const info = post.injury;
    return (
      <Link prefetch={false} href={href} data-pending-link="" className={cardClass}>
        <div className="flex aspect-[4/3] flex-col justify-between bg-blue-500/10 p-3.5">
          <div className="flex flex-wrap items-center gap-1.5">
            {info ? (
              <>
                <span className="w-fit rounded-full bg-white/12 px-2.5 py-0.5 text-xs font-semibold text-slate-200 border border-white/20">
                  {L(info.body_part_label)}
                </span>
                {info.stage_label ? (
                  <span className="w-fit rounded-full bg-emerald-950/70 px-2.5 py-0.5 text-xs font-bold text-emerald-300 border border-emerald-800/60">
                    {L(info.stage_label)}
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
            <span className="font-semibold text-blue-300">{L("わかる {like_count}", { like_count: post.like_count })}</span>
          </div>
        </div>
        <CardPending />
      </Link>
    );
  }
  const hasThumbnail = Boolean(thumbnail || (post.kind === "instagram" && isInstagramShortcode(post.instagram_shortcode ?? "")));
  return (
    <Link prefetch={false} href={href} data-pending-link="" aria-label={description ? undefined : L("投稿の詳細を開く")}
      className={`${cardClass}${hasThumbnail ? " feed-card-has-thumbnail" : ""}${post.kind === "text" && !hasThumbnail ? " feed-card-text" : ""}`}>
      <div className={`feed-card-thumbnail relative overflow-hidden bg-black/25 ${thumbnail || post.kind === "instagram" ? "" : "aspect-[4/3]"}`}>
        {post.kind === "recruitment" ? (
          <span className="absolute top-2.5 left-2.5 z-10">
            <KindBadge>{L("クラブ")}</KindBadge>
          </span>
        ) : post.kind === "x" ? (
          <span className="absolute top-2.5 left-2.5 z-10"><KindBadge>X</KindBadge></span>
        ) : post.kind === "instagram" ? (
          <span className="absolute top-2.5 left-2.5 z-10">
            <KindBadge>Instagram</KindBadge>
          </span>
        ) : null}
        {thumbnail ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img src={thumbnail} alt="" loading="lazy" decoding="async" className="block h-auto w-full"
            ref={(image) => { if (image?.complete) markOrientation(image, setLandscapeSrc); }}
            onLoad={(event) => markOrientation(event.currentTarget, setLandscapeSrc)} />
        ) : post.kind === "instagram" && (post.instagram_type === "p" || post.instagram_type === "reel") && isInstagramShortcode(post.instagram_shortcode ?? "") ? (
          <InstagramThumb type={post.instagram_type} shortcode={post.instagram_shortcode ?? ""} title={description || "Instagram"} />
        ) : (
          <PosterFallback title={description} />
        )}
        {hasThumbnail && description ? <CardCaption text={description} landscape={Boolean(thumbnail) && landscapeSrc === thumbnail} /> : null}
      </div>
      <CardPending />
    </Link>
  );
}

// 読み込んだ画像が横長なら、その URL を覚えておく(サムネイルが差し替わったら縦長扱いに戻る)。
function markOrientation(image: HTMLImageElement, setLandscapeSrc: (src: string) => void) {
  if (image.naturalWidth === 0) return;
  setLandscapeSrc(image.naturalWidth > image.naturalHeight ? image.getAttribute("src") ?? "" : "");
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
