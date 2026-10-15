import { describe, expect, it } from "vitest";
import { buildCommentThreads, replyLabel } from "./comment-thread";
import type { Comment } from "./types";

function c(id: string, extra: Partial<Comment> = {}): Comment {
  return { id, post_id: "p", body: id, mine: false, created_at: "", updated_at: "", ...extra };
}

describe("buildCommentThreads", () => {
  it("groups replies under their top comment in order", () => {
    const threads = buildCommentThreads([
      c("a"),
      c("b"),
      c("a1", { parent_id: "a", reply_to_id: "a" }),
      c("a2", { parent_id: "a", reply_to_id: "a1" }),
      c("b1", { parent_id: "b", reply_to_id: "b" }),
    ]);
    expect(threads.map((t) => t.top?.id)).toEqual(["a", "b"]);
    expect(threads[0].replies.map((r) => r.id)).toEqual(["a1", "a2"]);
    expect(threads[1].replies.map((r) => r.id)).toEqual(["b1"]);
  });

  it("keeps replies whose parent is gone", () => {
    const threads = buildCommentThreads([c("x1", { parent_id: "gone", reply_to_id: "gone" })]);
    expect(threads).toHaveLength(1);
    expect(threads[0].top).toBeNull();
    expect(threads[0].replies.map((r) => r.id)).toEqual(["x1"]);
  });
});

describe("replyLabel", () => {
  it("falls back when the target was removed", () => {
    expect(replyLabel({ reply_to_name: null, reply_to_removed: true })).toBe("削除されたコメント");
    expect(replyLabel({ reply_to_name: "あなた", reply_to_removed: false })).toBe("あなた");
  });
});
