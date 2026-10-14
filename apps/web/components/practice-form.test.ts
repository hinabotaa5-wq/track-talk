/** @vitest-environment jsdom */
import { createElement } from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";
import { PracticeForm } from "./practice-form";
import type { Post } from "@/lib/types";

const push = vi.fn();
const apiFetch = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push }),
}));

vi.mock("./auth-provider", () => ({
  useAuth: () => ({ session: { access_token: "token" }, token: "token" }),
}));

vi.mock("@/lib/api", () => ({
  apiFetch: (...args: unknown[]) => apiFetch(...args),
  ApiRequestError: class ApiRequestError extends Error {},
}));

const post = {
  id: "4020b4de-956e-4819-97e6-90cb82f83fc0",
  title: "100m 10月20日 18:00",
  body: "ジョグから入ります",
  mine: true,
  like_count: 0,
  comment_count: 0,
  liked_by_me: false,
  saved_by_me: false,
  kind: "practice",
  practice: {
    venue: "駒沢競技場",
    starts_at: "2026-10-20T09:00:00.000Z",
    event: "100m",
    event_label: "100m",
    capacity: 3,
    accepted_count: 0,
    status: "open",
  },
  created_at: "2026-10-12T00:00:00Z",
  updated_at: "2026-10-12T00:00:00Z",
} as Post;

describe("PracticeForm", () => {
  it("keeps a minimum start time while editing", () => {
    const html = renderToStaticMarkup(createElement(PracticeForm, { initial: post }));
    expect(html).toContain('type="datetime-local"');
    expect(html).toMatch(/min="\d{4}-\d{2}-\d{2}T\d{2}:\d{2}"/);
  });

  it("rejects a past start while editing", async () => {
    const div = document.createElement("div");
    document.body.appendChild(div);
    let root: Root;
    await act(async () => {
      root = createRoot(div);
      root.render(createElement(PracticeForm, { initial: { ...post, practice: { ...post.practice!, starts_at: "2020-01-01T00:00:00.000Z" } } }));
    });
    const form = div.querySelector("form");
    if (!form) throw new Error("form missing");
    await act(async () => {
      form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    });
    expect(div.textContent).toContain("練習日時は現在より後にしてください");
    expect(apiFetch).not.toHaveBeenCalled();
    expect(push).not.toHaveBeenCalled();
    await act(async () => {
      root!.unmount();
    });
  });
});
