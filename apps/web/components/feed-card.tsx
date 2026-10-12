import Link from "next/link";
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
    <span className="w-fit rounded-full bg-white px-2 py-0.5 text-xs font-medium text-zinc-700">{children}</span>
  );
}

export function FeedCard({ post, returnTo }: { post: Post; returnTo?: "me" }) {
  const title = feedTitle(post.title);
  const href = returnTo === "me" ? `/posts/${post.id}?from=me` : `/posts/${post.id}`;
  if (post.kind === "practice") {
    const info = post.practice;
    return (
      <Link href={href} className="block overflow-hidden rounded-2xl border bg-white hover:border-blue-300">
        <div className="flex aspect-[4/3] flex-col gap-2 bg-zinc-50 p-3">
          <KindBadge>練習</KindBadge>
          {info ? (
            <div className="space-y-1 text-sm leading-snug">
              <p className="font-semibold">{info.event_label}</p>
              <p>{formatDateTime(info.starts_at)}</p>
              <p className="line-clamp-2">{info.venue}</p>
              <p>
                募集 {info.accepted_count}/{info.capacity}人
                {info.status === "full" ? "・満員" : info.status === "dissolved" ? "・解散" : ""}
              </p>
              {practiceReputationLine(info.reputation) ? (
                <p className="text-xs text-zinc-600">{practiceReputationLine(info.reputation)}</p>
              ) : null}
            </div>
          ) : (
            <p className="line-clamp-4 text-sm font-semibold leading-snug">{title}</p>
          )}
        </div>
      </Link>
    );
  }
  return (
    <Link href={href} className="block overflow-hidden rounded-2xl border bg-white hover:border-blue-300">
      <div className="relative aspect-[4/3] bg-zinc-200">
        {post.kind === "recruitment" ? (
          <span className="absolute top-2 left-2 z-10">
            <KindBadge>クラブ</KindBadge>
          </span>
        ) : null}
        {post.thumbnail_url ? (
          <img src={post.thumbnail_url} alt="" className="h-full w-full object-cover" />
        ) : (
          <div className="flex h-full items-center p-3">
            <p className="line-clamp-4 text-sm font-semibold leading-snug">{title}</p>
          </div>
        )}
      </div>
      <p className="truncate px-3 py-2.5 text-sm">{title}</p>
    </Link>
  );
}
