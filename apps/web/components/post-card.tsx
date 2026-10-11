import Link from "next/link";
import { formatDateTime } from "@/lib/format";
import type { Post } from "@/lib/types";
import { Card } from "./ui/card";
import { feedTitle } from "./feed-card";

export function PostCard({ post }: { post: Post }) {
  return (
    <Card>
      <Link href={`/posts/${post.id}`} className="block">
        <h2 className="text-lg font-semibold">{feedTitle(post.title)}</h2>
        <p className="mt-2 line-clamp-3 text-sm text-zinc-600">
          {post.body_preview ?? post.body}
        </p>
        <p className="mt-3 text-xs text-zinc-500">
          {post.author.nickname} · {formatDateTime(post.created_at)} · いいね{" "}
          {post.like_count} · コメント {post.comment_count}
        </p>
      </Link>
    </Card>
  );
}
