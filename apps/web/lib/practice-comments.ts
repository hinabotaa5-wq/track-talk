import type { Post } from "./types";

export function canCommentOnPost(post: Pick<Post, "kind" | "mine" | "practice">) {
  if (post.kind !== "practice" || !post.practice) return true;
  if (post.practice.status === "open") return true;
  return post.mine || post.practice.my_application === "accepted";
}
