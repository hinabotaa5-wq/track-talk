import type { Comment } from "@/lib/types";

export type CommentThread = {
  // 親が削除・非表示で一覧に無いときは null
  top: Comment | null;
  replies: Comment[];
};

// 返信は1階層に固定されているので、parent_id でグルーピングするだけでよい。
// 一覧は作成順で届く前提で、スレッドの並びは最初の出現順に保つ。
export function buildCommentThreads(comments: Comment[]): CommentThread[] {
  const threads = new Map<string, CommentThread>();
  const ids = new Set(comments.map((c) => c.id));
  for (const c of comments) {
    if (!c.parent_id) threads.set(c.id, { top: c, replies: [] });
  }
  for (const c of comments) {
    if (!c.parent_id) continue;
    const key = c.parent_id;
    let thread = threads.get(key);
    if (!thread) {
      thread = { top: ids.has(key) ? (comments.find((x) => x.id === key) ?? null) : null, replies: [] };
      threads.set(key, thread);
    }
    thread.replies.push(c);
  }
  return [...threads.values()];
}

export function replyLabel(c: Pick<Comment, "reply_to_name" | "reply_to_removed">): string {
  return c.reply_to_removed || !c.reply_to_name ? "削除されたコメント" : c.reply_to_name;
}
