/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { ApiRequestError } from "@/lib/api";
import type { CoachReviewRequest } from "@/lib/types";
import CoachReviewRequestPage from "./page";

const apiFetch = vi.fn();
vi.mock("next/navigation", () => ({ useParams: () => ({ id: "r1" }), useRouter: () => ({ replace: vi.fn(), push: vi.fn() }) }));
vi.mock("@/lib/api", async (original) => ({ ...(await original<typeof import("@/lib/api")>()), apiFetch: (...args: unknown[]) => apiFetch(...args) }));
vi.mock("@/lib/features", async (original) => ({ ...(await original<typeof import("@/lib/features")>()), coachingMessagingEnabled: true }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => ({ session: { user: { id: "u1" } }, token: "t", loading: false }) }));
vi.mock("@/components/confirm-dialog", () => ({ useConfirm: () => async () => true, reportConfirmOptions: {} }));
const localeState = vi.hoisted(() => ({ value: "ja" as "ja" | "en" }));
vi.mock("@/components/locale-provider", () => ({ useLocale: () => localeState.value }));

const base = {
  id: "r1",
  tier: "coach",
  event: "100m",
  focus: [],
  question: "スタートが遅いです",
  status: "in_review",
  started_at: "2026-11-13T12:01:00+09:00",
  due_at: "2026-11-14T12:00:00+09:00",
  created_at: "2026-11-13T12:00:00+09:00",
  info_request: null,
  withdrawal: null,
  response: null,
} as unknown as CoachReviewRequest;

const responded = {
  ...base,
  response: {
    hidden: false,
    good_point: "腕振りが良い",
    fix_points: ["接地を直す"],
    drill: "もも上げ",
    demo_video_url: null,
    created_at: "2026-11-13T13:00:00+09:00",
    clarify_reason: null,
    clarify_body: null,
    clarify_requested_at: null,
    clarification: null,
    clarified_at: null,
    helpful_asked: false,
  },
} as unknown as CoachReviewRequest;

// 追加質問の欄は返答後に出る。部屋の取得は空の履歴で答える。
const emptyRoom = {
  consultations: [],
  role: "coach",
  funding: "cash",
  price_yen: 500,
  can_purchase: false,
  block_reason: "",
  requires_video: false,
};

let root: Root;
let container: HTMLDivElement;
let scroll: ReturnType<typeof vi.fn>;
let previousScroll: typeof HTMLElement.prototype.scrollIntoView;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
  vi.useFakeTimers();
  apiFetch.mockReset();
  localeState.value = "ja";
  scroll = vi.fn();
  previousScroll = HTMLElement.prototype.scrollIntoView;
  HTMLElement.prototype.scrollIntoView = scroll as unknown as typeof HTMLElement.prototype.scrollIntoView;
});
afterEach(async () => {
  await act(async () => root.unmount());
  vi.useRealTimers();
  container.remove();
  HTMLElement.prototype.scrollIntoView = previousScroll;
});

async function renderPage() {
  await act(async () => root.render(<CoachReviewRequestPage />));
  await act(async () => {
    await Promise.resolve();
  });
  await act(async () => {
    await Promise.resolve();
  });
}

function submitButton(text: string): HTMLButtonElement {
  const button = [...container.querySelectorAll("button")].find((b) => b.textContent === text);
  if (!button) throw new Error(`button not found: ${text}`);
  return button as HTMLButtonElement;
}

function responseForm(): HTMLFormElement {
  const form = [...container.querySelectorAll("form")].find((f) =>
    [...f.querySelectorAll("button")].some((b) => b.textContent === "返答を送る"),
  );
  if (!form) throw new Error("response form not found");
  return form as HTMLFormElement;
}

async function fillTextarea(textarea: HTMLTextAreaElement, value: string) {
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!.call(textarea, value);
    textarea.dispatchEvent(new Event("input", { bubbles: true }));
  });
}

// 完了バナーだけが3秒で消え、返答カードは残る。
it("返答を送ると上部オーバーレイが出て3秒で消え、返答は残る", async () => {
  apiFetch.mockImplementation(async (path: string, options?: { method?: string }) => {
    if (!options?.method && path.includes("/consultations")) return emptyRoom;
    if (options?.method === "POST") return responded;
    return base;
  });
  await renderPage();
  expect(document.querySelector('[role="status"]')).toBeNull();
  await act(async () => submitButton("返答を送る").click());
  expect(document.querySelector('[role="status"]')?.textContent).toContain("返答を送りました");
  expectOverlay();
  expect(scroll).not.toHaveBeenCalled();
  await expectBannerExpires("腕振りが良い");
});

// 返答の送信に失敗したら、バナーは出さない。
it("返答の送信に失敗したらバナーは出ない", async () => {
  apiFetch.mockImplementation(async (path: string, options?: { method?: string }) => {
    if (!options?.method && path.includes("/consultations")) return emptyRoom;
    if (options?.method === "POST") throw new Error("送信に失敗しました");
    return base;
  });
  await renderPage();
  await act(async () => submitButton("返答を送る").click());
  expect(document.querySelector('[role="status"]')).toBeNull();
  expect(container.textContent).toContain("送信に失敗しました");
  expect(scroll).not.toHaveBeenCalled();
});

it("保存済みの確認質問と補足は表示し、補足の送信フォームは出さない", async () => {
  const historical = {
    ...responded,
    status: "returned",
    response: { ...responded.response, clarify_reason: "term", clarify_body: "用語が分かりません", clarification: "補足です" },
  };
  apiFetch.mockImplementation(async (path: string) => path.includes("/consultations") ? emptyRoom : historical);
  await renderPage();
  expect(container.textContent).toContain("用語が分かりません");
  expect(container.textContent).toContain("補足です");
  expect(container.textContent).not.toContain("補足を送る");
});

// 差し戻しも同じオーバーレイ・消去になり、履歴は残る。
it("差し戻すと上部オーバーレイが出て3秒で消え、履歴は残る", async () => {
  const sentBack = {
    ...base,
    status: "needs_info",
    info_request: {
      body: "どこが分からないか具体的に教えてください",
      reason: null,
      requested_at: "2026-11-13T13:00:00+09:00",
      from_status: "in_review",
      resubmit_by: "2026-11-14T13:00:00+09:00",
      original_question: "スタートが遅いです",
      video_replaced: false,
      resubmitted_at: null,
    },
  } as unknown as CoachReviewRequest;
  apiFetch.mockImplementation(async (path: string, options?: { method?: string }) => {
    if (!options?.method && path.includes("/consultations")) return emptyRoom;
    if (options?.method === "POST") return sentBack;
    return base;
  });
  await renderPage();
  const textarea = container.querySelector("details textarea")!;
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!.call(textarea, "どこが分からないか具体的に教えてください");
    textarea.dispatchEvent(new Event("input", { bubbles: true }));
  });
  await act(async () => submitButton("質問を付けて差し戻す").click());
  expect(document.querySelector('[role="status"]')?.textContent).toContain("差し戻しました");
  expectOverlay();
  expect(scroll).not.toHaveBeenCalled();
  await expectBannerExpires("差し戻し中");
});

// 返答フォームは回答の型（本体 #1067）の4項目＋見本動画で、改善点は1欄だけ。
it("返答フォームの項目名は4つ＋見本動画で改善点は1欄だけ", async () => {
  apiFetch.mockImplementation(async (path: string) => (path.includes("/consultations") ? emptyRoom : base));
  await renderPage();
  const form = responseForm();
  const labels = [...form.querySelectorAll("label > span.font-semibold")].map((s) => s.textContent);
  expect(labels).toEqual(["良い点", "改善点", "おすすめのドリル", "コーチなら明日から何をするか", "見本動画（任意、60秒まで）"]);
  const text = container.textContent!;
  for (const stale of ["（1つ）", "（1つ目）", "（2つ目", "なくてもよい"]) {
    expect(text).not.toContain(stale);
  }
  expect(form.querySelectorAll("textarea")).toHaveLength(4);
});

// 送信内容は改善点1つ・コーチなら明日から何をするか（hitokoto）1つで、成功すると完了バナーが出る。
it("返答の送信内容は改善点1つとひとことを含む", async () => {
  let sentBody = "";
  apiFetch.mockImplementation(async (path: string, options?: { method?: string; body?: string }) => {
    if (!options?.method && path.includes("/consultations")) return emptyRoom;
    if (options?.method === "POST") {
      sentBody = options.body ?? "";
      return responded;
    }
    return base;
  });
  await renderPage();
  const areas = [...responseForm().querySelectorAll("textarea")] as HTMLTextAreaElement[];
  expect(areas).toHaveLength(4);
  await fillTextarea(areas[0], "腕振りが良い");
  await fillTextarea(areas[1], "接地を直す");
  await fillTextarea(areas[2], "もも上げ");
  await fillTextarea(areas[3], "次は抜き脚だけ意識してみましょう");
  await act(async () => submitButton("返答を送る").click());
  const payload = JSON.parse(sentBody);
  expect(payload.good_point).toBe("腕振りが良い");
  expect(payload.fix_points).toEqual(["接地を直す"]);
  expect(payload.drill).toBe("もも上げ");
  expect(payload.hitokoto).toBe("次は抜き脚だけ意識してみましょう");
  expect(document.querySelector('[role="status"]')?.textContent).toContain("返答を送りました");
});

// 本体 #1067 W2: コーチなら明日から何をするか（hitokoto）が空だと送れず、その欄の下に指摘が出る（サーバーが 422 を返した想定）。
it("W2: コーチなら明日から何をするかが空だと送信できず、その欄に指摘が出る", async () => {
  let sentBody = "";
  apiFetch.mockImplementation(async (path: string, options?: { method?: string; body?: string }) => {
    if (!options?.method && path.includes("/consultations")) return emptyRoom;
    if (options?.method === "POST") {
      const payload = JSON.parse(options.body ?? "{}");
      sentBody = JSON.stringify(payload);
      if (!payload.hitokoto) {
        throw new ApiRequestError(422, {
          error: { code: "validation_error", message: "入力を確認してください", fields: { hitokoto: "コーチなら明日から何をするかを入力してください" } },
        });
      }
      return responded;
    }
    return base;
  });
  await renderPage();
  const areas = [...responseForm().querySelectorAll("textarea")] as HTMLTextAreaElement[];
  await fillTextarea(areas[0], "腕振りが良い");
  await fillTextarea(areas[1], "接地を直す");
  await fillTextarea(areas[2], "もも上げ");
  // コーチなら明日から何をするか（hitokoto）は空のまま送る。
  await act(async () => submitButton("返答を送る").click());
  expect(JSON.parse(sentBody).hitokoto).toBe("");
  expect(document.querySelector('[role="status"]')).toBeNull();
  // 指摘は「コーチなら明日から何をするか」の欄（同じ label の中）に出る。
  const label = areas[3].closest("label")!;
  expect(label.querySelector("span.font-semibold")?.textContent).toBe("コーチなら明日から何をするか");
  expect(label.textContent).toContain("コーチなら明日から何をするかを入力してください");
});

it("添削をはじめると入力欄を表示する", async () => {
  apiFetch.mockImplementation(async (path: string, options?: { method?: string }) => {
    if (!options?.method && path.includes("/consultations")) return emptyRoom;
    return options?.method === "POST" ? base : { ...base, started_at: null };
  });
  await renderPage();
  expect(container.textContent).not.toContain("返答を送る");
  await act(async () => submitButton("添削をはじめる").click());
  expect(apiFetch).toHaveBeenCalledWith("/v1/coach/requests/r1/start", { method: "POST", token: "t" });
  expect(container.textContent).toContain("返答を送る");
});

// 本体 #916 B1・B4: コーチ（en）には、選手の相談文が英語メイン＋日本語原文の併記で出る。英訳が無い（国内コーチ）ときは原文だけ。
it("B1: コーチ（en）には相談文が英語メイン＋日本語原文の併記で出る", async () => {
  localeState.value = "en";
  apiFetch.mockResolvedValue({ ...base, question_en: "My start is slow" });
  await renderPage();
  expect(container.textContent).toContain("My start is slow");
  expect(container.querySelector("p[lang='ja']")?.textContent).toBe("スタートが遅いです");
  expect(questionHeadings()).not.toContain("Your question");
  expect(questionHeadings()).not.toContain("相談内容");
});
it("B4: 英訳が null なら、en でも原文だけ", async () => {
  localeState.value = "en";
  apiFetch.mockResolvedValue({ ...base, question_en: null });
  await renderPage();
  expect(container.textContent).toContain("スタートが遅いです");
  expect(container.querySelector("p[lang='ja']")).toBeNull();
});

// 本体 #1068: 「依頼内容」枠の中に「相談内容」の見出しは出さない（本文・作成日時・選手情報は従来どおり）。
function questionHeadings() {
  const section = container.querySelector('section[aria-label="依頼内容"], section[aria-label="Your request"]')!;
  expect(section).not.toBeNull();
  return [...section.querySelectorAll("h1, h2, h3, h4, h5, h6")].map((h) => h.textContent);
}

it("依頼内容（動画→相談文）は吹き出しにせず上にまとめ、自分の回答は1枚のドキュメントで出し、末尾リンクを出さない（本体 #984）", async () => {
  apiFetch.mockImplementation(async (path: string) => path.includes("/consultations") ? emptyRoom : { ...responded, status: "returned", video_url: "https://example.com/run.mp4", thumbnail_url: "https://example.com/run.jpg" });
  await renderPage();
  const question = [...container.querySelectorAll("p")].find((p) => p.textContent === base.question)!;
  const request = question.closest('section[aria-label="依頼内容"]')!;
  expect(request).not.toBeNull();
  expect(request.querySelector("time")?.getAttribute("datetime")).toBe(base.created_at);
  expect(questionHeadings()).not.toContain("相談内容");
  expect(container.querySelector(".justify-start, .justify-end")).toBeNull();
  expect(container.querySelector("h1")?.parentElement?.textContent).not.toContain(base.question);
  const video = container.querySelector("video")!;
  expect(video.closest("section")).toBe(request);
  expect(video.controls).toBe(true);
  expect(video.playsInline).toBe(true);
  expect(video.getAttribute("poster")).toBe("https://example.com/run.jpg");
  expect(video.classList).toContain("max-w-[260px]");
  expect(video.classList).toContain("max-h-[300px]");
  expect(video.classList).toContain("mx-auto");
  expect(video.compareDocumentPosition(question) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  const response = [...container.querySelectorAll("h3")].find((h) => h.textContent === "良い点")!.closest("article")!;
  expect(question.compareDocumentPosition(response) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  expect(response.querySelector("h2")?.textContent).toBe("あなたの回答");
  expect(container.textContent).not.toContain("コーチ画面へ");
  expect(container.querySelector('a[href="/coach"]')).toBeNull();
});

it("返答前は空の返答を出さず、期限とフォームを従来どおり出す", async () => {
  apiFetch.mockResolvedValue(base);
  await renderPage();
  expect(container.querySelectorAll("time")).toHaveLength(1);
  expect(container.querySelector("article")).toBeNull();
  expect(container.textContent).toContain("返却期限");
  expect(responseForm()).toBeTruthy();
  expect(container.textContent).not.toContain("コーチ画面へ");
});

function expectOverlay() {
  const banner = document.querySelector('[role="status"]')!;
  expect(banner.parentElement).toBe(document.body);
  expect(container.contains(banner)).toBe(false);
  for (const name of ["fixed", "top-4", "z-50", "pointer-events-none", "max-w-md", "border-emerald-500/60", "bg-emerald-950/50"]) {
    expect(banner.classList.contains(name)).toBe(true);
  }
  expect(banner.querySelector('svg[aria-hidden="true"]')).not.toBeNull();
}

async function expectBannerExpires(remainingText: string) {
  await act(async () => vi.advanceTimersByTime(2999));
  expect(document.querySelector('[role="status"]')).not.toBeNull();
  await act(async () => vi.advanceTimersByTime(1));
  expect(document.querySelector('[role="status"]')).toBeNull();
  expect(container.textContent).toContain(remainingText);
  await renderPage();
  await act(async () => vi.advanceTimersByTime(6000));
  expect(document.querySelector('[role="status"]')).toBeNull();
  expect(container.textContent).toContain(remainingText);
}

it("英語の Response sent も上部オーバーレイで3秒後に消える", async () => {
  localeState.value = "en";
  apiFetch.mockImplementation(async (path: string, options?: { method?: string }) => {
    if (!options?.method && path.includes("/consultations")) return emptyRoom;
    if (options?.method === "POST") return responded;
    return base;
  });
  await renderPage();
  await act(async () => submitButton("Send response").click());
  expect(document.querySelector('[role="status"]')?.textContent).toContain("Response sent");
  expectOverlay();
  expect(scroll).not.toHaveBeenCalled();
  await expectBannerExpires("腕振りが良い");
});

it("差し戻しの送信に失敗したらバナーは出ない", async () => {
  apiFetch.mockImplementation(async (_path: string, options?: { method?: string }) => {
    if (options?.method === "POST") throw new Error("送信に失敗しました");
    return base;
  });
  await renderPage();
  await fillTextarea(container.querySelector("details textarea")!, "どこが分からないか具体的に教えてください");
  await act(async () => submitButton("質問を付けて差し戻す").click());
  expect(document.querySelector('[role="status"]')).toBeNull();
  expect(container.textContent).toContain("送信に失敗しました");
  expect(scroll).not.toHaveBeenCalled();
});

// 本体 #1067 W1: 4欄が 良い点→改善点→おすすめのドリル→コーチなら明日から何をするか の順で出て、キュー等の欄は無く、送信に cue 等のキーを含めない。
it("W1: 回答の型の4欄が順に出て、送信内容に cue・success_criteria・next_video が入らない", async () => {
  let sentBody = "";
  apiFetch.mockImplementation(async (path: string, options?: { method?: string; body?: string }) => {
    if (!options?.method && path.includes("/consultations")) return emptyRoom;
    if (options?.method === "POST") {
      sentBody = options.body ?? "";
      return responded;
    }
    return base;
  });
  await renderPage();
  const form = responseForm();
  const fields = [...form.querySelectorAll("label")].filter((l) => l.querySelector("textarea"));
  expect(fields.map((l) => l.querySelector("span.font-semibold")?.textContent)).toEqual(["良い点", "改善点", "おすすめのドリル", "コーチなら明日から何をするか"]);
  for (const stale of ["キュー", "成功の目安", "次に撮る動画", "一言診断", "ボトルネック"]) {
    expect(form.textContent).not.toContain(stale);
  }
  // 各欄の入力補助（プレースホルダー）は固定の文言。
  expect([...form.querySelectorAll("textarea")].map((a) => a.getAttribute("placeholder"))).toEqual([
    "残すべき所を1つ",
    "一番の課題を1つ。なぜ遅くなるかまで書く",
    "ドリル名と回数（例：壁押しドリル 10秒×3本）",
    "自分なら明日から何をするか、具体的に書く",
  ]);
  const values = ["良い", "課題", "ドリル 10秒×3本", "明日からこうする"];
  const areas = [...form.querySelectorAll("textarea")] as HTMLTextAreaElement[];
  for (const [i, v] of values.entries()) await fillTextarea(areas[i], v);
  await act(async () => submitButton("返答を送る").click());
  const payload = JSON.parse(sentBody);
  expect(payload).toEqual({
    good_point: "良い",
    fix_points: ["課題"],
    drill: "ドリル 10秒×3本",
    hitokoto: "明日からこうする",
    demo_video_public_id: "",
  });
  for (const key of ["cue", "success_criteria", "next_video"]) {
    expect(key in payload).toBe(false);
  }
  expect(document.querySelector('[role="status"]')?.textContent).toContain("返答を送りました");
});

// 本体 #1013 補足 B5: 申込の選手情報は、値のある項目だけが相談内容と並んで出る。
it("B5: 値のある選手情報だけが相談内容と並んで表示される", async () => {
  const withInfo = { ...base, focus_note: "接地で地面を押すこと", pb: "11秒20（2025年）", goal_time: null, wind_speed: "+1.2", body_condition: "" } as unknown as CoachReviewRequest;
  apiFetch.mockImplementation(async (path: string) => (path.includes("/consultations") ? emptyRoom : withInfo));
  await renderPage();
  const request = container.querySelector('section[aria-label="依頼内容"]')!;
  const info = request.querySelector('dl[aria-label="選手情報"]')!;
  // 本体 #1093 W2：「意識していること」は値があるときだけ、選手情報の先頭に出る。
  expect([...info.querySelectorAll("dt")].map((d) => d.textContent)).toEqual(["意識していること", "PB（自己ベスト）", "風速"]);
  expect([...info.querySelectorAll("dd")].map((d) => d.textContent)).toEqual(["接地で地面を押すこと", "11秒20（2025年）", "+1.2"]);
  expect(request.textContent).toContain("スタートが遅いです");
  expect(questionHeadings()).not.toContain("相談内容");
  expect(request.textContent).not.toContain("目標タイム");
  expect(request.textContent).not.toContain("その時の体のコンディション");
});

it("B5: 選手情報が1つも無い依頼（過去の申込）は、選手情報の欄を出さない", async () => {
  apiFetch.mockImplementation(async (path: string) => (path.includes("/consultations") ? emptyRoom : base));
  await renderPage();
  expect(container.querySelector('dl[aria-label="選手情報"]')).toBeNull();
  expect(container.textContent).not.toContain("意識していること");
  expect(container.textContent).toContain("スタートが遅いです");
});
