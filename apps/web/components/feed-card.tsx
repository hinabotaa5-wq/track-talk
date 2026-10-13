import Link from "next/link";
import Image from "next/image";
import type { Post } from "@/lib/types";
import { formatDateTime } from "@/lib/format";
import { practiceReputationLine } from "@/components/practice-reputation";
import { PosterFallback } from "@/components/poster-card";

export const feedGridClass = "grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-4";

export function feedTitle(title: string) {
  const trimmed = title.trim();
  return trimmed === "" ? "（質問なし）" : trimmed;
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
        className="group block overflow-hidden rounded-2xl border border-white/20 glass bg-white/10 shadow-md backdrop-blur-xs transition-all duration-300 ease-out hover:-translate-y-1 hover:shadow-xl hover:border-white/40 hover:shadow-blue-950/30 active:scale-[0.98]"
      >
        <div className="flex aspect-[4/3] flex-col justify-between bg-gradient-to-br from-white/10 to-sky-400/10 p-3.5">
          <div className="flex items-center justify-between">
            <span className="w-fit rounded-full bg-blue-950/80 px-2.5 py-0.5 text-[11px] font-bold text-blue-400 border border-blue-800/60">
              PRACTICE 練習
            </span>
            <span className="font-mono text-[10px] font-bold tracking-wider text-slate-300">MEET</span>
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
          <div className="flex items-center justify-between border-t border-white/20 pt-1 text-[10px] font-mono text-slate-300">
            <span>TRACK TALK</span>
            <span className="font-semibold text-blue-400">JOIN &gt;</span>
          </div>
        </div>
        <p className="truncate px-3.5 py-2.5 text-sm font-medium text-slate-200 transition-colors group-hover:text-blue-400">{title}</p>
      </Link>
    );
  }
  return (
    <Link
      href={href}
      className="group block overflow-hidden rounded-2xl border border-white/20 glass bg-white/10 shadow-md backdrop-blur-xs transition-all duration-300 ease-out hover:-translate-y-1 hover:shadow-xl hover:border-white/40 hover:shadow-blue-950/30 active:scale-[0.98]"
    >
      <div className="relative aspect-[4/3] overflow-hidden bg-black/25">
        {post.kind === "recruitment" ? (
          <span className="absolute top-2.5 left-2.5 z-10">
            <KindBadge>クラブ</KindBadge>
          </span>
        ) : null}
        {post.thumbnail_url ? (
          <Image
            src={post.thumbnail_url}
            alt={title}
            fill
            sizes="(max-width: 768px) 50vw, (max-width: 1280px) 33vw, 25vw"
            className="object-cover transition-transform duration-500 ease-out group-hover:scale-105"
            loading="lazy"
          />
        ) : (
          <PosterFallback id={post.id} title={title} />
        )}
      </div>
      <p className="truncate px-3.5 py-3 text-sm font-medium text-slate-200 transition-colors group-hover:text-blue-400">{title}</p>
    </Link>
  );
}
