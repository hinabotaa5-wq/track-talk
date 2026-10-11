import Link from "next/link";
import type { Post } from "@/lib/types";

export function feedTitle(title: string) {
  const trimmed = title.trim();
  return trimmed === "" ? "（質問なし）" : trimmed;
}

export function FeedCard({ post }: { post: Post }) {
  const title = feedTitle(post.title);
  return (
    <Link href={`/posts/${post.id}`} className="block overflow-hidden rounded-2xl border bg-white">
      <div className="flex aspect-[4/3] items-center bg-zinc-200 p-3">
        <p className="line-clamp-4 text-sm font-semibold leading-snug">{title}</p>
      </div>
      <p className="truncate px-3 py-2.5 text-sm">{title}</p>
    </Link>
  );
}
