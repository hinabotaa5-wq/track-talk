/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { ReviewCoach, ReviewResponse } from "@/lib/types";
import { ReviewCoachCard, ReviewResponseView } from "./review-response";

const flags = { list: true };
vi.mock("@/lib/features", async (original) => ({
  ...await original<typeof import("@/lib/features")>(),
  get coachingCoachListEnabled() { return flags.list; },
}));
vi.mock("next/link", () => ({ default: ({ href, children, ...props }: { href: string; children: ReactNode }) => <a href={href} {...props}>{children}</a> }));
vi.mock("@/components/coach-photo", () => ({ CoachAvatar: ({ name }: { name: string }) => <span>{name}の顔写真</span> }));

const coach: ReviewCoach = {
  id: "coach-1", nickname: "テストコーチ", photo_url: "", tier: "coach", events: ["100m"],
  best_record: "10秒台", achievements: "全国大会出場", years: 10,
};
let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  flags.list = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });

it("添削のコーチからコーチ詳細へ直接進み、種目・記録・実績を行並びで表示する", async () => {
  await act(async () => root.render(<ReviewCoachCard coach={coach} />));
  const link = [...container.querySelectorAll("a")].find((a) => a.textContent?.includes("このコーチのプロフィールを見る"));
  expect(link?.getAttribute("href")).toBe("/coaches/coach-1");
  expect(container.textContent).toContain("テストコーチ");
  expect(container.textContent).toContain("100m");
  expect(container.textContent).toContain("10秒台");
  expect(container.textContent).toContain("全国大会出場");
  expect(container.textContent).not.toContain("見てくれたコーチ");
  expect(container.textContent).not.toContain("担当種目");
  expect(container.textContent).not.toContain("成績や指導経験");
});

it("コーチ一覧を閉じている間は、準備中のコーチ詳細へ案内しない", async () => {
  flags.list = false;
  await act(async () => root.render(<ReviewCoachCard coach={coach} />));
  expect(container.textContent).not.toContain("このコーチのプロフィールを見る");
  expect(container.querySelector('a[href^="/coaches"]')).toBeNull();
  expect(container.textContent).toContain("テストコーチ");
});

const baseResponse = {
  hidden: false,
  good_point: "腕振りが良い",
  fix_points: ["接地を直す"],
  drill: "もも上げ",
  hitokoto: "次は抜き脚だけ意識してみましょう",
  demo_video_url: null,
  created_at: "2026-11-13T13:00:00+09:00",
  clarify_reason: null,
  clarify_body: null,
  clarify_requested_at: null,
  clarification: null,
  clarified_at: null,
  helpful_asked: false,
} as unknown as ReviewResponse;

it("返答にひとことがあればひとこと欄を表示する", async () => {
  await act(async () => root.render(<ReviewResponseView response={baseResponse} />));
  expect(container.textContent).toContain("ひとこと");
  expect(container.textContent).toContain("次は抜き脚だけ意識してみましょう");
});

it("ひとことが無い過去の返答はひとこと欄を出さずに表示する", async () => {
  const past = { ...baseResponse, hitokoto: null } as unknown as ReviewResponse;
  await act(async () => root.render(<ReviewResponseView response={past} />));
  expect(container.textContent).toContain("腕振りが良い");
  expect(container.textContent).not.toContain("ひとこと");
});
