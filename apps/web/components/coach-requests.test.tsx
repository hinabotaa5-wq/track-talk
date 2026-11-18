/** @vitest-environment jsdom */
// コーチの対応中一覧（/coach/reviewing）で、動画を消した依頼の注記が表示言語どおりに出ること（本体 #944 A1）。
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { CoachRequestsSection } from "./coach-requests";
import { installEnglish, setActiveLocale } from "@/lib/i18n";
import { en } from "@/lib/i18n-en";
import type { CoachRequests } from "@/lib/types";

const apiFetch = vi.fn();
vi.mock("@/lib/api", async (original) => ({
  ...await original<typeof import("@/lib/api")>(),
  apiFetch: (...args: unknown[]) => apiFetch(...args),
}));
vi.mock("next/link", () => ({
  default: ({ href, children, ...props }: { href: string; children: ReactNode }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

const purged: CoachRequests = {
  offers: [],
  reviewing: [
    {
      id: "r1",
      tier: "coach",
      event: "100m",
      focus: [],
      question: "スタートの姿勢について",
      status: "in_review",
      due_at: "2026-11-18T03:00:00Z",
      created_at: "2026-11-17T03:00:00Z",
      videos_purged: true,
    },
  ],
};

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
  apiFetch.mockResolvedValue(purged);
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
  apiFetch.mockReset();
  setActiveLocale("ja");
});

it("日本語では動画削除の注記を従来どおりの日本語で出す", async () => {
  setActiveLocale("ja");
  await act(async () => root.render(<CoachRequestsSection token="t" locale="ja" />));
  expect(container.textContent).toContain("動画は保存期間（添削のやりとりが終わってから7日）を過ぎたため削除しました。添削の文章は引き続き見られます。");
});

it("英語では動画削除の注記を英語で出し、日本語を残さない", async () => {
  installEnglish(en);
  setActiveLocale("en");
  await act(async () => root.render(<CoachRequestsSection token="t" locale="en" />));
  expect(container.textContent).toContain("The video was deleted because the retention period (7 days after the review ended) passed. The review text is still available.");
  expect(container.textContent).not.toContain("動画は保存期間");
});
