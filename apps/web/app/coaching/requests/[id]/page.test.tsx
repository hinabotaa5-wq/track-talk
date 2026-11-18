/** @vitest-environment jsdom */
import { act, type MouseEvent as ReactMouseEvent, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { expect, it, vi } from "vitest";
import { LeaveGuardProvider } from "@/components/leave-guard";
import { en } from "@/lib/i18n-en";
import { reviewVideosPurgedNote } from "@/lib/review";
import ReviewRequestPage from "./page";

const routerMock = vi.hoisted(() => ({ push: vi.fn(), replace: vi.fn() }));
const searchState = vi.hoisted(() => ({ paid: false }));
vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "review-1" }),
  useRouter: () => routerMock,
  useSearchParams: () => new URLSearchParams(searchState.paid ? "paid=1" : ""),
}));
vi.mock("@/components/auth-provider", () => ({
  useAuth: () => ({ session: { user: { id: "athlete" } }, token: "token", loading: false }),
}));
const confirmMock = vi.hoisted(() => vi.fn<(options: { title: string; body: string; confirmLabel: string }) => Promise<boolean>>(async () => true));
vi.mock("@/components/confirm-dialog", () => ({
  useConfirm: () => confirmMock,
  reportConfirmOptions: (value: unknown) => value,
}));
const apiFetch = vi.fn();
// 実物の next/link（app-dir/link.js）はクリック時に e.defaultPrevented を見て
// 止まっているときは navigate しないことをソースで確認済み。
// ここではその契約だけを再現する（未阻止＝push、阻止＝何もしない）。
vi.mock("next/link", () => ({
  default: ({ href, children, onClick, ...props }: { href: string; children: ReactNode; onClick?: (e: ReactMouseEvent<HTMLAnchorElement>) => void }) => (
    <a
      href={href}
      {...props}
      onClick={(event: ReactMouseEvent<HTMLAnchorElement>) => {
        onClick?.(event);
        if (!event.defaultPrevented) {
          event.preventDefault();
          routerMock.push(href);
        }
      }}
    >
      {children}
    </a>
  ),
}));
vi.mock("@/lib/api", async (original) => ({
  ...await original<typeof import("@/lib/api")>(),
  apiFetch: (...args: unknown[]) => apiFetch(...args),
}));
vi.mock("@/lib/features", async (original) => ({
  ...await original<typeof import("@/lib/features")>(),
  coachingMessagingEnabled: true,
  coachingConsultationEnabled: true,
  coachingCoachListEnabled: true,
}));

type RequestFixture = {
  response?: { hidden: boolean } | null;
  rating?: { stars: number | null; helpful: null } | null;
  coach?: { id: string } | null;
  extra?: Record<string, unknown>;
};

function baseRequest(override: RequestFixture = {}) {
  const request: Record<string, unknown> = {
    id: "review-1",
    status: "returned",
    tier: "coach",
    event: "100m",
    focus: [],
    question: "スタートの姿勢について",
    created_at: new Date().toISOString(),
  };
  if (override.response === null) {
    request.response = undefined;
  } else {
    request.response = {
      hidden: override.response?.hidden ?? false,
      good_point: "腕振り",
      fix_points: ["接地位置"],
      drill: "もも上げ",
      created_at: new Date().toISOString(),
      helpful_asked: false,
    };
  }
  if (override.rating !== undefined) request.rating = override.rating;
  if (override.coach !== undefined) {
    request.coach = override.coach
      ? {
        id: override.coach.id,
        nickname: "テストコーチ",
        tier: "coach",
        events: ["100m"],
        best_record: "10秒台",
        achievements: "実績",
        photo_url: null,
      }
      : undefined;
  }
  if (override.extra) Object.assign(request, override.extra);
  return request;
}

function mockConsultationsEmpty(path: string): unknown {
  if (path.endsWith("/consultations")) {
    return { consultations: [], role: "requester", funding: "cash", price_yen: 500, can_purchase: true, requires_video: false };
  }
  return undefined;
}

async function renderPage() {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root = createRoot(container);
  await act(async () => root.render(
    <LeaveGuardProvider>
      <ReviewRequestPage />
    </LeaveGuardProvider>,
  ));
  return { container, root };
}

async function teardown(root: Root, container: HTMLDivElement) {
  await act(async () => root.unmount());
  container.remove();
  vi.unstubAllGlobals();
  vi.clearAllMocks();
}

function listLink(container: HTMLDivElement) {
  const link = container.querySelector('a[href="/me/coaching/requests"]');
  if (!link) throw new Error("依頼の一覧へのリンクが見つかりません");
  return link as HTMLAnchorElement;
}

async function click(element: Element) {
  await act(async () => {
    element.dispatchEvent(new MouseEvent("click", { bubbles: true, cancelable: true, button: 0 }));
  });
}

it("返却後の質問には有料の追加質問だけを表示する", async () => {
  apiFetch.mockImplementation(async (path: string) => {
    const consultations = mockConsultationsEmpty(path);
    if (consultations !== undefined) return consultations;
    if (path !== "/v1/coaching/requests/review-1") throw new Error(`Unexpected request: ${path}`);
    return baseRequest();
  });
  const { container, root } = await renderPage();
  try {
    expect(container.textContent).toContain("腕振り");
    expect(container.querySelector('textarea[aria-label="追加質問の質問"]')).not.toBeNull();
    expect(container.textContent).toContain("質問する（500円）");
    expect(container.querySelectorAll("textarea")).toHaveLength(1);
    expect(container.textContent).not.toContain("確認質問を送る（無料）");
  } finally {
    await teardown(root, container);
  }
});

it("返却済みの依頼詳細に星セクションは出ない", async () => {
  apiFetch.mockImplementation(async (path: string) => {
    const consultations = mockConsultationsEmpty(path);
    if (consultations !== undefined) return consultations;
    if (path !== "/v1/coaching/requests/review-1") throw new Error(`Unexpected request: ${path}`);
    return baseRequest();
  });
  const { container, root } = await renderPage();
  try {
    expect(container.textContent).toContain("腕振り");
    expect(container.textContent).not.toContain("この添削の評価");
    expect(container.textContent).not.toContain("この添削の評価をつけてください");
    expect(container.querySelector('button[role="radio"]')).toBeNull();
    expect(container.querySelectorAll('button[aria-label$="つ星"]')).toHaveLength(0);
  } finally {
    await teardown(root, container);
  }
});

it("未評価の返却済み依頼でも依頼の一覧へ遷移できる", async () => {
  apiFetch.mockImplementation(async (path: string) => {
    const consultations = mockConsultationsEmpty(path);
    if (consultations !== undefined) return consultations;
    if (path !== "/v1/coaching/requests/review-1") throw new Error(`Unexpected request: ${path}`);
    return baseRequest();
  });
  const { container, root } = await renderPage();
  try {
    await click(listLink(container));
    expect(routerMock.push).toHaveBeenCalledWith("/me/coaching/requests");
    expect(container.textContent).not.toContain("この添削の評価をつけてください");
  } finally {
    await teardown(root, container);
  }
});

it("未評価の返却済み依頼でもコーチのプロフィールへ遷移できる", async () => {
  apiFetch.mockImplementation(async (path: string) => {
    const consultations = mockConsultationsEmpty(path);
    if (consultations !== undefined) return consultations;
    if (path !== "/v1/coaching/requests/review-1") throw new Error(`Unexpected request: ${path}`);
    return baseRequest({ coach: { id: "coach-1" } });
  });
  const { container, root } = await renderPage();
  try {
    const profile = container.querySelector('a[href="/coaches/coach-1"]');
    expect(profile).not.toBeNull();
    await click(profile!);
    expect(routerMock.push).toHaveBeenCalledWith("/coaches/coach-1");
  } finally {
    await teardown(root, container);
  }
});

it("評価済みのデータでも星セクションは出ない", async () => {
  apiFetch.mockImplementation(async (path: string) => {
    const consultations = mockConsultationsEmpty(path);
    if (consultations !== undefined) return consultations;
    if (path !== "/v1/coaching/requests/review-1") throw new Error(`Unexpected request: ${path}`);
    return baseRequest({ rating: { stars: 3, helpful: null } });
  });
  const { container, root } = await renderPage();
  try {
    expect(container.querySelector('button[role="radio"]')).toBeNull();
    expect(container.textContent).not.toContain("この添削の評価をつけてください");
    await click(listLink(container));
    expect(routerMock.push).toHaveBeenCalledWith("/me/coaching/requests");
  } finally {
    await teardown(root, container);
  }
});

it("星の評価送信APIは呼ばれない", async () => {
  apiFetch.mockImplementation(async (path: string) => {
    const consultations = mockConsultationsEmpty(path);
    if (consultations !== undefined) return consultations;
    if (path !== "/v1/coaching/requests/review-1") throw new Error(`Unexpected request: ${path}`);
    return baseRequest();
  });
  const { container, root } = await renderPage();
  try {
    await click(listLink(container));
    expect(routerMock.push).toHaveBeenCalledWith("/me/coaching/requests");
    expect(apiFetch.mock.calls.some(([path]) => String(path).endsWith("/rating"))).toBe(false);
  } finally {
    await teardown(root, container);
  }
});

it("返答がない依頼では評価文言が出ず離脱できる", async () => {
  apiFetch.mockImplementation(async (path: string) => {
    const consultations = mockConsultationsEmpty(path);
    if (consultations !== undefined) return consultations;
    if (path !== "/v1/coaching/requests/review-1") throw new Error(`Unexpected request: ${path}`);
    return baseRequest({ response: null });
  });
  const { container, root } = await renderPage();
  try {
    expect(container.textContent).not.toContain("この添削の評価をつけてください");
    await click(listLink(container));
    expect(routerMock.push).toHaveBeenCalledWith("/me/coaching/requests");
  } finally {
    await teardown(root, container);
  }
});

it("非表示の返答では星セクションが出ず離脱できる", async () => {
  apiFetch.mockImplementation(async (path: string) => {
    const consultations = mockConsultationsEmpty(path);
    if (consultations !== undefined) return consultations;
    if (path !== "/v1/coaching/requests/review-1") throw new Error(`Unexpected request: ${path}`);
    return baseRequest({ response: { hidden: true } });
  });
  const { container, root } = await renderPage();
  try {
    expect(container.querySelector('button[role="radio"]')).toBeNull();
    await click(listLink(container));
    expect(routerMock.push).toHaveBeenCalledWith("/me/coaching/requests");
  } finally {
    await teardown(root, container);
  }
});

it("通報の導線は残る", async () => {
  apiFetch.mockImplementation(async (path: string) => {
    const consultations = mockConsultationsEmpty(path);
    if (consultations !== undefined) return consultations;
    if (path !== "/v1/coaching/requests/review-1") throw new Error(`Unexpected request: ${path}`);
    return baseRequest();
  });
  const { container, root } = await renderPage();
  try {
    expect(container.querySelector("button")?.textContent).not.toBeNull();
    expect(container.textContent).toContain("この返答を通報する");
  } finally {
    await teardown(root, container);
  }
});

const returnedResponse = () => ({
  hidden: false,
  good_point: "腕振り",
  fix_points: ["接地位置"],
  drill: "もも上げ",
  created_at: new Date().toISOString(),
  helpful_asked: false,
});

const returnedCoach = () => ({
  nickname: "デモコーチ",
  photo_url: "",
  tier: "coach",
  events: ["100m"],
  best_record: "10秒台",
  achievements: "指導歴あり",
  years: 3,
});

async function unmountPage(container: HTMLDivElement, root: ReturnType<typeof createRoot>) {
  await act(async () => root.unmount());
  container.remove();
  vi.unstubAllGlobals();
}

it("返却後＋コーチカードありは「選んだコーチ」枠を出さない", async () => {
  apiFetch.mockImplementation(async (path: string) => {
    if (path.endsWith("/consultations")) {
      return { consultations: [], role: "requester", funding: "cash", price_yen: 500, can_purchase: true, requires_video: false };
    }
    if (path !== "/v1/coaching/requests/review-1") throw new Error(`Unexpected request: ${path}`);
    return {
      id: "review-1",
      status: "returned",
      tier: "coach",
      event: "100m",
      focus: [],
      question: "スタートの姿勢について",
      created_at: new Date().toISOString(),
      requested_coach_id: "coach-1",
      requested_coach_name: "デモコーチ",
      total_price_yen: 500,
      response: returnedResponse(),
      coach: returnedCoach(),
    };
  });
  const { container, root } = await renderPage();
  try {
    expect(container.textContent).not.toContain("選んだコーチ：");
    expect(container.textContent).toContain("デモコーチ");
  } finally {
    await unmountPage(container, root);
  }
});

it("返却前は「選んだコーチ」枠を出す", async () => {
  apiFetch.mockImplementation(async (path: string) => {
    if (path !== "/v1/coaching/requests/review-1") throw new Error(`Unexpected request: ${path}`);
    return {
      id: "review-1",
      status: "in_review",
      tier: "coach",
      event: "100m",
      focus: [],
      question: "スタートの姿勢について",
      created_at: new Date().toISOString(),
      due_at: new Date(Date.now() + 86400000).toISOString(),
      requested_coach_id: "coach-1",
      requested_coach_name: "デモコーチ",
      total_price_yen: 500,
    };
  });
  const { container, root } = await renderPage();
  try {
    expect(container.textContent).toContain("選んだコーチ：デモコーチ");
    expect(container.textContent).toContain("料金500円");
  } finally {
    await unmountPage(container, root);
  }
});

it("返却後＋コーチなしは「選んだコーチ」枠を残す", async () => {
  apiFetch.mockImplementation(async (path: string) => {
    if (path.endsWith("/consultations")) {
      return { consultations: [], role: "requester", funding: "cash", price_yen: 500, can_purchase: false, requires_video: false };
    }
    if (path !== "/v1/coaching/requests/review-1") throw new Error(`Unexpected request: ${path}`);
    return {
      id: "review-1",
      status: "returned",
      tier: "coach",
      event: "100m",
      focus: [],
      question: "スタートの姿勢について",
      created_at: new Date().toISOString(),
      requested_coach_id: "coach-1",
      requested_coach_name: "デモコーチ",
      total_price_yen: 500,
      response: returnedResponse(),
      coach: null,
    };
  });
  const { container, root } = await renderPage();
  try {
    expect(container.textContent).toContain("選んだコーチ：デモコーチ");
  } finally {
    await unmountPage(container, root);
  }
});

it("期限切れは「選んだコーチ」枠と別のコーチへのリンクを出す", async () => {
  apiFetch.mockImplementation(async (path: string) => {
    if (path !== "/v1/coaching/requests/review-1") throw new Error(`Unexpected request: ${path}`);
    return {
      id: "review-1",
      status: "expired",
      tier: "coach",
      event: "100m",
      focus: [],
      question: "スタートの姿勢について",
      created_at: new Date().toISOString(),
      requested_coach_id: "coach-1",
      requested_coach_name: "デモコーチ",
      total_price_yen: 500,
    };
  });
  const { container, root } = await renderPage();
  try {
    expect(container.textContent).toContain("選んだコーチ：デモコーチ");
    expect(container.querySelector('a[href="/coaching/new"]')).not.toBeNull();
    expect(container.textContent).toContain("別のコーチを選んで申し込む");
  } finally {
    await unmountPage(container, root);
  }
});

it("お支払い待ちの依頼の支払いボタンは「確定してお支払いへ」で、金額も方法名も含まず、紹介IDの欄は「紹介ID」だけ", async () => {
  apiFetch.mockImplementation(async (path: string) => {
    if (path === "/v1/coaching/requests/review-1") {
      return {
        id: "review-1",
        status: "awaiting_payment",
        tier: "coach",
        event: "100m",
        focus: [],
        question: "スタートの姿勢について",
        created_at: new Date().toISOString(),
        due_at: new Date(Date.now() + 86400000).toISOString(),
        requested_coach_id: "coach-1",
        requested_coach_name: "デモコーチ",
        total_price_yen: 500,
      };
    }
    if (path.endsWith("/referral")) return { referral: null };
    return {};
  });
  const { container, root } = await renderPage();
  try {
    const labels = [...container.querySelectorAll("button")].map((b) => b.textContent);
    expect(labels).toContain("確定してお支払いへ");
    expect(labels.some((t) => t?.includes("で払う") || t?.includes("500円"))).toBe(false);
    const referralLabel = [...container.querySelectorAll("label")].find((l) => l.textContent?.startsWith("紹介ID"));
    expect(referralLabel?.firstChild?.textContent).toBe("紹介ID");
    expect(container.textContent).not.toContain("友達からもらった場合");
  } finally {
    await unmountPage(container, root);
  }
});

const unpaidRequest = (status: string, extra: Record<string, unknown> = {}) => ({
  id: "review-1",
  status,
  tier: "coach",
  event: "100m",
  focus: [],
  question: "スタートの姿勢について",
  created_at: new Date().toISOString(),
  due_at: new Date(Date.now() + 3600000).toISOString(),
  requested_coach_id: "coach-1",
  requested_coach_name: "デモコーチ",
  total_price_yen: 500,
  ...extra,
});

function cancelButton(container: HTMLDivElement) {
  return Array.from(container.querySelectorAll("button")).find((b) => b.textContent === "依頼をキャンセルする");
}

it("お支払い待ちはキャンセルボタンを出し、案内文は2時間の1文だけ", async () => {
  apiFetch.mockImplementation(async (path: string) => {
    if (path !== "/v1/coaching/requests/review-1") throw new Error(`Unexpected request: ${path}`);
    return unpaidRequest("awaiting_payment");
  });
  const { container, root } = await renderPage();
  try {
    expect(cancelButton(container)).toBeDefined();
    expect(container.textContent).toContain("2時間たってもお支払いがなければ、この依頼は消えます。");
    expect(container.textContent).not.toContain("コーチを探し始めます");
  } finally {
    await unmountPage(container, root);
  }
});
it("お支払い待ちのキャンセルは専用の本文で確認し、取り消すとお支払い欄が消える", async () => {
  let canceled = false;
  apiFetch.mockImplementation(async (path: string, init?: { method?: string }) => {
    if (path === "/v1/coaching/requests/review-1/cancel" && init?.method === "POST") {
      canceled = true;
      return unpaidRequest("expired", { canceled_at: new Date().toISOString() });
    }
    if (path !== "/v1/coaching/requests/review-1") throw new Error(`Unexpected request: ${path}`);
    return unpaidRequest("awaiting_payment");
  });
  const { container, root } = await renderPage();
  try {
    const payButtons = () => Array.from(container.querySelectorAll("button")).filter((b) => b.textContent !== "依頼をキャンセルする").length;
    expect(payButtons()).toBeGreaterThan(0);
    await click(cancelButton(container)!);
    expect(confirmMock).toHaveBeenCalledWith(expect.objectContaining({
      title: "1回プランの依頼をキャンセルしますか？",
      body: "まだお支払いは済んでいません。取り消すと、この依頼ではお支払いができなくなります。",
      confirmLabel: "依頼をキャンセルする",
    }));
    expect(canceled).toBe(true);
    expect(container.textContent).toContain("依頼をキャンセルしました");
    expect(cancelButton(container)).toBeUndefined();
    expect(container.textContent).not.toContain("2時間たってもお支払いがなければ");
  } finally {
    await unmountPage(container, root);
  }
});

it("着手前の添削中は従来の返金の本文のまま、着手済みはキャンセルボタンを出さない", async () => {
  apiFetch.mockImplementation(async (path: string) => {
    if (path !== "/v1/coaching/requests/review-1") throw new Error(`Unexpected request: ${path}`);
    return unpaidRequest("in_review");
  });
  const first = await renderPage();
  try {
    await click(cancelButton(first.container)!);
    expect(confirmMock).toHaveBeenLastCalledWith(expect.objectContaining({ body: "お支払いは全額返金・取消し、チケットは返還します。" }));
  } finally {
    await unmountPage(first.container, first.root);
  }
  apiFetch.mockImplementation(async () => unpaidRequest("in_review", { started_at: new Date().toISOString() }));
  const second = await renderPage();
  try {
    expect(cancelButton(second.container)).toBeUndefined();
  } finally {
    await unmountPage(second.container, second.root);
  }
});

async function renderWith(override: RequestFixture) {
  apiFetch.mockImplementation(async (path: string) => {
    const consultations = mockConsultationsEmpty(path);
    if (consultations !== undefined) return consultations;
    if (path !== "/v1/coaching/requests/review-1") throw new Error(`Unexpected request: ${path}`);
    return baseRequest(override);
  });
  return renderPage();
}

function dlTerms(container: HTMLDivElement) {
  return Array.from(container.querySelectorAll("dl dt")).map((dt) => dt.textContent);
}

it("返却前の依頼詳細は「誰に見てもらうか」と種目を出さず、相談内容だけを枠に出す", async () => {
  const { container, root } = await renderWith({ response: null, extra: { status: "in_review", due_at: new Date().toISOString() } });
  try {
    expect(container.textContent).toContain("スタートの姿勢について");
    expect(container.textContent).not.toContain("誰に見てもらうか");
    expect(container.textContent).not.toContain("100m");
    expect(dlTerms(container)).toEqual([]);
  } finally {
    await teardown(root, container);
  }
});

it("動画ありの依頼は現行のプレイヤーを出し、相談内容も出す", async () => {
  const { container, root } = await renderWith({
    response: null,
    extra: { status: "in_review", due_at: new Date().toISOString(), video_url: "https://example.com/run.mp4", thumbnail_url: "https://example.com/run.jpg" },
  });
  try {
    const videos = container.querySelectorAll("video");
    expect(videos).toHaveLength(1);
    expect(videos[0].getAttribute("src")).toBe("https://example.com/run.mp4");
    expect(videos[0].getAttribute("poster")).toBe("https://example.com/run.jpg");
    expect(videos[0].hasAttribute("controls")).toBe(true);
    expect(container.textContent).toContain("スタートの姿勢について");
    expect(container.textContent).not.toContain("誰に見てもらうか");
  } finally {
    await teardown(root, container);
  }
});

it("動画なし（悩みのみ）の依頼は動画も保存期間の注記も出さず、相談内容だけを出す", async () => {
  const { container, root } = await renderWith({ response: null, extra: { status: "in_review", due_at: new Date().toISOString() } });
  try {
    expect(container.querySelector("video")).toBeNull();
    expect(container.textContent).not.toContain(reviewVideosPurgedNote);
    expect(dlTerms(container)).toEqual([]);
  } finally {
    await teardown(root, container);
  }
});

it("動画が削除済みの依頼は保存期間の注記と相談内容を出す", async () => {
  const { container, root } = await renderWith({ response: null, extra: { status: "in_review", due_at: new Date().toISOString(), videos_purged: true } });
  try {
    expect(container.querySelector("video")).toBeNull();
    expect(container.textContent).toContain(reviewVideosPurgedNote);
    expect(container.textContent).toContain("スタートの姿勢について");
  } finally {
    await teardown(root, container);
  }
});

it("返却後も「誰に見てもらうか」は出ず、相談内容とコーチカードの種目は残る", async () => {
  const { container, root } = await renderWith({ coach: { id: "coach-1" } });
  try {
    expect(container.textContent).not.toContain("誰に見てもらうか");
    expect(dlTerms(container)).toEqual([]);
    expect(container.textContent).toContain("スタートの姿勢について");
    expect(container.textContent).toContain("テストコーチ");
    expect(container.textContent).toContain("100m");
  } finally {
    await teardown(root, container);
  }
});

it("英語辞書に「誰に見てもらうか」はなく、「相談内容」「種目」は残る", () => {
  expect(Object.keys(en)).not.toContain("誰に見てもらうか");
  expect(en["相談内容"]).toBeDefined();
  expect(en["種目"]).toBeDefined();
});

it("「依頼の一覧へ」は枠付きのボタン形で、行き先と文言は変わらない", async () => {
  const { container, root } = await renderWith({});
  try {
    const link = listLink(container);
    expect(link.className).toContain("border");
    expect(link.className).toContain("rounded-xl");
    expect(link.className).not.toContain("underline");
    expect(link.textContent).toBe("依頼の一覧へ");
    expect(link.getAttribute("href")).toBe("/me/coaching/requests");
  } finally {
    await teardown(root, container);
  }
});

it("依頼動画→相談文は右・青、返答は左の独立カードとして続く", async () => {
  const { container, root } = await renderWith({ extra: { created_at: "2026-11-17T21:19:00Z", video_url: "https://example.com/run.mp4", thumbnail_url: "https://example.com/run.jpg" } });
  try {
    const question = [...container.querySelectorAll("p")].find((p) => p.textContent === "スタートの姿勢について")!;
    const bubble = question.parentElement!;
    expect(bubble.classList).toContain("bg-blue-500/25");
    expect(bubble.parentElement?.classList).toContain("justify-end");
    expect(bubble.querySelector("time")?.textContent).toBe("11/18 6:19");
    expect(bubble.textContent).not.toContain("2026");
    expect(container.querySelectorAll("dl")).toHaveLength(0);
    const video = container.querySelector("video")!;
    expect(video.classList).toContain("max-w-[260px]");
    expect(video.controls).toBe(true);
    expect(video.closest("div.flex")?.classList).toContain("justify-end");
    expect(video.parentElement?.classList).toContain("bg-blue-500/25");
    expect(video.getAttribute("poster")).toBe("https://example.com/run.jpg");
    expect(video.compareDocumentPosition(question) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    const response = [...container.querySelectorAll("h2")].find((h) => h.textContent === "良い点")!.closest("section")!;
    expect(question.compareDocumentPosition(response) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(response.firstElementChild?.classList).toContain("self-start");
    expect(response.firstElementChild?.classList).toContain("bg-white/10");
  } finally { await teardown(root, container); }
});

it("対応中は相談文1つだけで空の返答カードを出さず、グレーの期限表示とキャンセル操作を残す", async () => {
  const { container, root } = await renderWith({ response: null, extra: { status: "in_review", started_at: null, due_at: new Date(Date.now() + 86400000).toISOString() } });
  try {
    expect(container.querySelectorAll("time")).toHaveLength(1);
    expect([...container.querySelectorAll("h2")].some((h) => h.textContent === "良い点")).toBe(false);
    const deadline = [...container.querySelectorAll("p")].find((p) => p.textContent?.startsWith("返却期限:"))!;
    for (const c of ["text-center", "text-xs", "text-slate-400"]) expect(deadline.parentElement?.classList).toContain(c);
    expect(deadline.parentElement?.className).not.toContain("bg-");
    expect([...container.querySelectorAll("button")].some((b) => b.textContent === "依頼をキャンセルする")).toBe(true);
    expect(listLink(container)).toBeTruthy();
  } finally { await teardown(root, container); }
});

it("Stripeからの支払い同期後は状態・支払い文をシステム表示、紹介をcompactにする", async () => {
  searchState.paid = true;
  let synced = false;
  const paid = baseRequest({ response: null, extra: { status: "in_review", started_at: null, due_at: new Date(Date.now() + 86400000).toISOString(), payment: { method: "card", status: "captured", amount_yen: 500 }, requested_coach_id: "c1", requested_coach_name: "指名コーチ", tickets: { quantity: 1, status: "consumed" } } });
  apiFetch.mockImplementation(async (path: string) => {
    if (path.endsWith("/payment/sync")) { synced = true; return paid; }
    if (path.endsWith("/referral")) return { referral: { code: "ABC123", active: false, friend_count: 2, granted_at: "2026-11-14T09:00:00Z", expires_at: "2027-05-14T09:00:00Z" } };
    if (path === "/v1/coaching/requests/review-1") return synced ? paid : { ...paid, status: "awaiting_payment", payment: null };
    throw new Error(`Unexpected request: ${path}`);
  });
  const { container, root } = await renderPage();
  try {
    expect(apiFetch).toHaveBeenCalledWith("/v1/coaching/requests/review-1/payment/sync", { method: "POST", token: "token" });
    const payment = [...container.querySelectorAll("p")].find((p) => p.textContent === "500円をお支払い済みです（カード・Apple Pay・Google Pay）。")!;
    expect(payment).toBeTruthy();
    const system = payment.parentElement!;
    for (const c of ["text-center", "text-xs", "text-slate-400"]) expect(system.classList).toContain(c);
    expect(system.className).not.toContain("bg-");
    expect(system.className).not.toContain("border");
    expect(system.textContent).toContain("添削中");
    expect(system.textContent).toContain("選んだコーチ：指名コーチ");
    expect(system.textContent).toContain("料金500円");
    expect(system.textContent).toContain("チケット1枚：利用済み");
    expect(system.nextElementSibling?.textContent).toBe("依頼をキャンセルする");
    expect(container.textContent).toContain("あなたの紹介ID ABC123");
    expect(container.textContent).toContain("無料チケットをプレゼントしました");
    expect(container.querySelector("section h2")?.classList).toContain("text-sm");
    for (const text of ["購入すると、紹介IDが発行されます。", "付与日", "有効期限", "このIDは終了しました。"]) expect(container.textContent).not.toContain(text);
  } finally { searchState.paid = false; await teardown(root, container); }
});
