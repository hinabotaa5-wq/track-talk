import Link from "next/link";
import type { Post } from "@/lib/types";
import { formatDateTime } from "@/lib/format";
import { practiceReputationLine } from "@/components/practice-reputation";
import { PosterFallback } from "@/components/poster-card";
import { InstagramThumb } from "@/components/instagram-thumb";
import { isInstagramShortcode } from "@/lib/instagram";

export const feedGridClass = "columns-2 gap-3 md:columns-3 xl:columns-4";

const cardClass =
  "group mb-3 block w-full break-inside-avoid overflow-hidden rounded-2xl border border-white/20 glass bg-white/10 shadow-md backdrop-blur-xs transition-all duration-300 ease-out hover:-translate-y-1 hover:shadow-xl hover:border-white/40 hover:shadow-blue-950/30 active:scale-95";

// タイトルを書かない投稿(YouTube・Instagram の URL など)では、カードの下のタイトル欄を出さない。
export function hasFeedTitle(title: string) {
  return title.trim() !== "";
}

export function feedTitle(title: string) {
  const trimmed = title.trim();
  return trimmed === "" ? "（質問なし）" : trimmed;
}

function CardTitle({ title }: { title: string }) {
  return (
    <div className="px-3.5 py-2.5">
      <p className="truncate text-sm font-medium text-slate-200 transition-colors group-hover:text-blue-400">{title}</p>
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
  const href = returnTo === "me" ? `/posts/${post.id}?from=me` : `/posts/${post.id}`;
  if (post.kind === "practice") {
    const info = post.practice;
    return (
      <Link
        href={href}
        className={cardClass}
      >
        <div className="flex aspect-[4/3] flex-col justify-between bg-gradient-to-br from-white/10 to-sky-400/10 p-3.5">
          <div className="flex items-center justify-between">
            <span className="w-fit rounded-full bg-blue-950/80 px-2.5 py-0.5 text-xs font-bold text-blue-400 border border-blue-800/60">
              PRACTICE 練習
            </span>
            <span className="font-mono text-xs font-bold tracking-wider text-slate-300">MEET</span>
          </div>
          {info ? (
            <div className="space-y-1 text-sm leading-snug">
              <p className="font-bold text-slate-100 transition-colors group-hover:text-blue-400 line-clamp-1">{info.event_label}</p>
              <p className="text-xs text-slate-200">{formatDateTime(info.starts_at)}</p>
              <p className="line-clamp-1 text-xs text-slate-200">{info.venue}</p>
              <p className="text-xs font-semibold text-slate-300">
                募集 {info.accepted_count}/{info.capacity}人
                <span className="font-normal text-slate-300">
                  {info.status === "full" ? "・満員" : info.status === "dissolved" ? "・解散" : "・募集中"}
                </span>
              </p>
              {practiceReputationLine(info.reputation) ? (
                <p className="text-xs text-slate-200">{practiceReputationLine(info.reputation)}</p>
              ) : null}
            </div>
          ) : (
            <p className="line-clamp-3 text-sm font-bold text-slate-100 group-hover:text-blue-400">{title}</p>
          )}
          <div className="flex items-center justify-between border-t border-white/20 pt-1 text-xs font-mono text-slate-300">
            <span>TrackTalk</span>
            <span className="font-semibold text-blue-400">JOIN &gt;</span>
          </div>
        </div>
        <CardTitle title={title} />
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
          <p className="line-clamp-4 text-sm font-bold leading-snug text-slate-100 transition-colors group-hover:text-blue-400">
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
  return (
    <Link href={href} className={cardClass}>
      <div className={`relative overflow-hidden bg-black/25 ${post.thumbnail_url || post.kind === "instagram" ? "" : "aspect-[4/3]"}`}>
        {post.kind === "recruitment" ? (
          <span className="absolute top-2.5 left-2.5 z-10">
            <KindBadge>クラブ</KindBadge>
          </span>
        ) : post.kind === "instagram" ? (
          <span className="absolute top-2.5 left-2.5 z-10">
            <KindBadge>Instagram</KindBadge>
          </span>
        ) : null}
        {post.thumbnail_url ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img src={post.thumbnail_url} alt="" className="block h-auto w-full" />
        ) : post.kind === "instagram" && (post.instagram_type === "p" || post.instagram_type === "reel") && isInstagramShortcode(post.instagram_shortcode ?? "") ? (
          <InstagramThumb type={post.instagram_type} shortcode={post.instagram_shortcode ?? ""} title={title} />
        ) : (
          <PosterFallback title={title} excerpt={post.excerpt} />
        )}
      </div>
      {hasFeedTitle(post.title) ? <CardTitle title={title} /> : null}
    </Link>
  );
}
