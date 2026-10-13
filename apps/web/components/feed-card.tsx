import Link from "next/link";
import Image from "next/image";
import type { Post } from "@/lib/types";
import { formatDateTime } from "@/lib/format";
import { practiceReputationLine } from "@/components/practice-reputation";

export const feedGridClass = "grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-4";

export function feedTitle(title: string) {
  const trimmed = title.trim();
  return trimmed === "" ? "（質問なし）" : trimmed;
}

function KindBadge({ children }: { children: string }) {
  return (
    <span className="w-fit rounded-full bg-white/90 backdrop-blur-md px-2.5 py-0.5 text-xs font-medium text-zinc-800 shadow-xs border border-white/50">
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
        className="group block overflow-hidden rounded-2xl border border-zinc-200/80 bg-white shadow-xs transition-all duration-300 ease-out hover:-translate-y-1 hover:shadow-md hover:border-zinc-300 active:scale-[0.98]"
      >
        <div className="flex aspect-[4/3] flex-col gap-2 bg-zinc-50/70 p-3.5 transition-colors group-hover:bg-zinc-50">
          <KindBadge>練習</KindBadge>
          {info ? (
            <div className="space-y-1 text-sm leading-snug">
              <p className="font-semibold text-zinc-900 transition-colors group-hover:text-blue-600">{info.event_label}</p>
              <p className="text-zinc-600">{formatDateTime(info.starts_at)}</p>
              <p className="line-clamp-2 text-zinc-600">{info.venue}</p>
              <p className="text-zinc-600">
                募集 {info.accepted_count}/{info.capacity}人
                {info.status === "full" ? "・満員" : info.status === "dissolved" ? "・解散" : ""}
              </p>
              {practiceReputationLine(info.reputation) ? (
                <p className="text-xs text-zinc-500">{practiceReputationLine(info.reputation)}</p>
              ) : null}
            </div>
          ) : (
            <p className="line-clamp-4 text-sm font-semibold leading-snug text-zinc-900 transition-colors group-hover:text-blue-600">{title}</p>
          )}
        </div>
      </Link>
    );
  }
  return (
    <Link
      href={href}
      className="group block overflow-hidden rounded-2xl border border-zinc-200/80 bg-white shadow-xs transition-all duration-300 ease-out hover:-translate-y-1 hover:shadow-md hover:border-zinc-300 active:scale-[0.98]"
    >
      <div className="relative aspect-[4/3] overflow-hidden bg-zinc-100">
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
          <div className="flex h-full items-center p-3.5">
            <p className="line-clamp-4 text-sm font-semibold leading-snug text-zinc-900 transition-colors group-hover:text-blue-600">{title}</p>
          </div>
        )}
      </div>
      <p className="truncate px-3.5 py-3 text-sm font-medium text-zinc-900 transition-colors group-hover:text-blue-600">{title}</p>
    </Link>
  );
}
