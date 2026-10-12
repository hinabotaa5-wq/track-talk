import type { Post, Practice } from "./types";

export function canCommentOnPost(post: {
  kind: Post["kind"];
  mine: boolean;
  practice?: Pick<Practice, "status" | "my_application">;
}) {
  if (post.kind !== "practice" || !post.practice) return true;
  if (post.practice.status === "open") return true;
  return post.mine || post.practice.my_application === "accepted";
}
