/** @vitest-environment jsdom */
import { act, type MouseEvent as ReactMouseEvent, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { expect, it, vi } from "vitest";
import { LeaveGuardProvider } from "@/components/leave-guard";
import ReviewRequestPage from "./page";

const routerMock = vi.hoisted(() => ({ push: vi.fn(), replace: vi.fn() }));
vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "review-1" }),
  useRouter: () => routerMock,
  useSearchParams: () => new URLSearchParams(),
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
