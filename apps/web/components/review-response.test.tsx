/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it } from "vitest";
import type { ReviewResponse } from "@/lib/types";
import { ReviewResponseView } from "./review-response";

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });

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

// 本体 #984: 1回プランの回答は、項目ごとの吹き出しではなく1枚の回答ドキュメント（コーチ名・顔写真・回答日＋各項目）で出す。
it.each(["requester", "coach"] as const)("%s には回答を1枚のドキュメントとして、項目の見出し順に出す", async (viewer) => {
  await act(async () => root.render(<ReviewResponseView viewer={viewer} coach={{ nickname: "テストコーチ", photo_url: "https://example.com/c.jpg" }} response={{ ...baseResponse, fix_points: ["接地を直す", "姿勢を保つ"], demo_video_url: "https://example.com/demo.mp4" }} />));
  const doc = container.querySelector("article")!;
  expect(container.children).toHaveLength(1);
  expect(doc.querySelector("h2")?.textContent).toBe(viewer === "coach" ? "あなたの回答" : "テストコーチコーチの回答");
  expect(doc.querySelector("img") !== null).toBe(viewer === "requester");
  expect([...doc.querySelectorAll("h3")].map((h) => h.textContent)).toEqual(["良い点", "直す点 1", "直す点 2", "おすすめのドリル", "ひとこと", "見本動画"]);
  expect(doc.className).not.toContain("max-w-[85%]");
  expect(container.querySelectorAll("time")).toHaveLength(1);
  expect(doc.querySelector("time")?.getAttribute("datetime")).toBe(baseResponse.created_at);
  expect(doc.querySelector("time")?.textContent).toContain("11/13");
  const video = container.querySelector("video")!;
  expect(video.controls).toBe(true);
  expect(video.playsInline).toBe(true);
  expect(video.classList).toContain("max-w-[260px]");
  expect(video.classList).toContain("max-h-[300px]");
  expect(video.classList).toContain("mx-auto");
  expect(container.querySelector("button, textarea, input")).toBeNull();
});

it("任意項目なしの返答は3項目だけで、直す点に番号を付けない", async () => {
  await act(async () => root.render(<ReviewResponseView response={{ ...baseResponse, hitokoto: null, demo_video_url: null }} />));
  expect([...container.querySelectorAll("h3")].map((h) => h.textContent)).toEqual(["良い点", "直す点", "おすすめのドリル"]);
  expect(container.querySelector("video")).toBeNull();
  expect(container.querySelector("h2")?.textContent).toBe("コーチの回答");
});

it.each([null, "用語の説明です"])("確認質問・補足はドキュメントの最後の項目に統合して出る（%s）", async (clarification) => {
  await act(async () => root.render(<ReviewResponseView response={{ ...baseResponse, clarify_reason: "term", clarify_body: "用語が分かりません", clarification }} />));
  const last = container.querySelector("article")!.lastElementChild!;
  expect(last.querySelector("h3")?.textContent).toContain("確認質問（");
  expect(last.textContent).toContain("用語が分かりません");
  if (clarification) {
    expect(last.textContent).toContain("補足（「");
    expect(last.textContent).toContain(clarification);
  } else {
    expect(last.textContent).not.toContain("補足（「");
  }
  expect(last.querySelector("time")).toBeNull();
});

it("非表示の返答は案内1枚だけで本文・日時・動画を出さない", async () => {
  await act(async () => root.render(<ReviewResponseView viewer="coach" response={{ ...baseResponse, hidden: true }} />));
  expect(container.textContent).toBe("この返答は運営が非表示にしました。");
  expect(container.children).toHaveLength(1);
  expect(container.querySelector("time, video, h2")).toBeNull();
});
