/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { ReviewCoach } from "@/lib/types";
import { ReviewCoachCard } from "./review-response";

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
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });

it("添削のコーチからコーチ詳細へ直接進み、成績や指導経験と表示する", async () => {
  await act(async () => root.render(<ReviewCoachCard coach={coach} />));
  const link = [...container.querySelectorAll("a")].find((a) => a.textContent?.includes("このコーチのプロフィールを見る"));
  expect(link?.getAttribute("href")).toBe("/coaches/coach-1");
  expect(container.textContent).toContain("成績や指導経験");
});
