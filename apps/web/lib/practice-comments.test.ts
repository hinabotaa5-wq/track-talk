import { describe, expect, it } from "vitest";
import { canCommentOnPost } from "./practice-comments";

const practice = {
  kind: "practice" as const,
  mine: false,
  practice: {
    status: "open" as const,
    my_application: undefined as "pending" | "accepted" | "declined" | undefined,
  },
};

describe("canCommentOnPost", () => {
  it("lets anyone comment while recruitment is open", () => {
    expect(canCommentOnPost(practice)).toBe(true);
  });

  it("keeps closed posts readable for outsiders by hiding new comments", () => {
    expect(canCommentOnPost({ ...practice, practice: { ...practice.practice, status: "full" } })).toBe(false);
    expect(canCommentOnPost({ ...practice, practice: { ...practice.practice, status: "dissolved" } })).toBe(false);
    expect(
      canCommentOnPost({
        ...practice,
        practice: { ...practice.practice, status: "dissolved", my_application: "pending" },
      }),
    ).toBe(false);
  });

  it("lets the host and accepted members keep talking after recruitment ends", () => {
    expect(canCommentOnPost({ ...practice, mine: true, practice: { ...practice.practice, status: "dissolved" } })).toBe(
      true,
    );
    expect(
      canCommentOnPost({
        ...practice,
        practice: { ...practice.practice, status: "full", my_application: "accepted" },
      }),
    ).toBe(true);
  });

  it("does not lock ordinary posts", () => {
    expect(canCommentOnPost({ kind: "text", mine: false })).toBe(true);
  });
});
