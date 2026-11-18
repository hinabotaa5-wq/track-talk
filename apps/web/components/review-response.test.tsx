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
} as Extract<ReviewResponse, { hidden: false }>;

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

// 本体 #916 B1・B4: 海外コーチの回答3点は、和訳があれば和訳を主に・英語の原文を併記する。訳が無い項目・コーチ本人の画面は原文だけ。
const shownResponse = {
  hidden: false as const, good_point: "Stable first 8 steps", fix_points: ["Take off half a foot earlier", "Lift the trail knee"], drill: "Mini hurdles x3",
  hitokoto: null, demo_video_url: null, created_at: "2026-12-01T00:00:00Z", clarify_reason: null, clarify_requested_at: null, clarification: null, clarified_at: null, helpful_asked: false,
};
it("B1: 選手（ja）には、回答3点が日本語メイン＋英語原文の併記で出て、訳が無い項目は原文だけ", async () => {
  await act(async () => root.render(<ReviewResponseView response={{ ...shownResponse, good_point_ja: "最初の8歩が安定しています", fix_points_ja: ["半足ぶん手前で踏み切りましょう", "抜き脚の膝を上げましょう"], drill_ja: null }} />));
  const originals = [...container.querySelectorAll("p[lang='en']")].map((p) => p.textContent);
  expect(container.textContent).toContain("最初の8歩が安定しています");
  expect(container.textContent).toContain("抜き脚の膝を上げましょう");
  expect(originals).toEqual(["Stable first 8 steps", "Take off half a foot earlier", "Lift the trail knee"]);
  expect(container.textContent).toContain("Mini hurdles x3");
});
it("B4: 訳の項目が null（国内コーチ）やコーチ本人の画面では、全て原文だけ", async () => {
  await act(async () => root.render(<ReviewResponseView response={{ ...shownResponse, good_point_ja: null, fix_points_ja: null, drill_ja: null }} />));
  expect(container.querySelectorAll("p[lang='en']")).toHaveLength(0);
  expect(container.textContent).toContain("Stable first 8 steps");
  await act(async () => root.render(<ReviewResponseView viewer="coach" response={{ ...shownResponse, good_point_ja: "訳", fix_points_ja: ["訳1", "訳2"], drill_ja: "訳3" }} />));
  expect(container.querySelectorAll("p[lang='en']")).toHaveLength(0);
  expect(container.textContent).not.toContain("訳1");
});

it.each(["requester", "coach"] as const)("%s は項目ごとに独立したカードで、閲覧者基準の寄せ・色・日時を表示する", async (viewer) => {
  await act(async () => root.render(<ReviewResponseView viewer={viewer} response={{ ...baseResponse, fix_points: ["接地を直す", "姿勢を保つ"], demo_video_url: "https://example.com/demo.mp4" }} />));
  const cards = [...container.querySelector("section")!.children];
  expect(cards.map((c) => c.querySelector("h2")?.textContent)).toEqual(["良い点", "直す点 1", "直す点 2", "おすすめのドリル", "ひとこと", "見本動画"]);
  for (const card of cards) {
    expect(card.classList).toContain("max-w-[85%]");
    expect(card.classList).toContain(viewer === "coach" ? "self-end" : "self-start");
    expect(card.classList).toContain(viewer === "coach" ? "bg-blue-500/25" : "bg-white/10");
  }
  expect(cards[0].querySelector("time")?.textContent).toBe("11/13 13:00");
  expect(cards[0].querySelector("time")?.getAttribute("datetime")).toBe(baseResponse.created_at);
  expect(container.querySelectorAll("time")).toHaveLength(1);
  expect(cards.slice(1).every((card) => !card.querySelector("time"))).toBe(true);
  expect(container.textContent).not.toContain("2026");
  const video = container.querySelector("video")!;
  expect(video.controls).toBe(true);
  expect(video.playsInline).toBe(true);
  expect(video.classList).toContain("max-w-[260px]");
  expect(container.querySelector("button")).toBeNull();
});

it("任意項目なしの返答は3枚だけで、直す点に番号を付けない", async () => {
  await act(async () => root.render(<ReviewResponseView response={{ ...baseResponse, hitokoto: null, demo_video_url: null }} />));
  expect([...container.querySelectorAll("h2")].map((h) => h.textContent)).toEqual(["良い点", "直す点", "おすすめのドリル"]);
  expect(container.querySelector("section")?.children).toHaveLength(3);
  expect(container.querySelector("video")).toBeNull();
});

it.each([null, "用語の説明です"])("確認質問・補足は最後の1カードに従来の分岐で出る（%s）", async (clarification) => {
  await act(async () => root.render(<ReviewResponseView response={{ ...baseResponse, clarify_reason: "term", clarify_body: "用語が分かりません", clarification }} />));
  const card = container.querySelector("section")!.lastElementChild!;
  expect(card.textContent).toContain("確認質問：用語が分かりません");
  expect(card.textContent).toContain(clarification ? "補足（「" : "確認質問（");
  if (clarification) expect(card.textContent).toContain(clarification);
  expect(card.querySelector("time")).toBeNull();
});

it("非表示の返答は案内1枚だけで本文・日時・動画を出さない", async () => {
  await act(async () => root.render(<ReviewResponseView viewer="coach" response={{ ...baseResponse, hidden: true }} />));
  expect(container.textContent).toBe("この返答は運営が非表示にしました。");
  expect(container.children).toHaveLength(1);
  expect(container.querySelector("time, video, h2")).toBeNull();
});
