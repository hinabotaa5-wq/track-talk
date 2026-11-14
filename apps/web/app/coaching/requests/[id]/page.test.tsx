/** @vitest-environment jsdom */
import { act, type MouseEvent as ReactMouseEvent, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { expect, it, vi } from "vitest";
import { LeaveGuardProvider, useLeaveGuard } from "@/components/leave-guard";
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
vi.mock("@/components/confirm-dialog", () => ({
  useConfirm: () => async () => true,
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
  helpfulAsked?: boolean;
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
      helpful_asked: override.helpfulAsked ?? false,
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

it("未評価の返却済み依頼でも依頼の一覧へ遷移でき、星セクションも文言も出ない", async () => {
  apiFetch.mockImplementation(async (path: string) => {
    const consultations = mockConsultationsEmpty(path);
    if (consultations !== undefined) return consultations;
    if (path !== "/v1/coaching/requests/review-1") throw new Error(`Unexpected request: ${path}`);
    return baseRequest();
  });
  const { container, root } = await renderPage();
  try {
    expect(container.textContent).not.toContain("この添削の評価");
    expect(container.textContent).not.toContain("この添削の評価をつけてください");
    expect(container.querySelector('button[role="radio"]')).toBeNull();
    await click(listLink(container));
    expect(routerMock.push).toHaveBeenCalledWith("/me/coaching/requests");
  } finally {
    await teardown(root, container);
  }
});

it("confirmLeave経由の離脱（ヘッダー戻る相当）も止まらない", async () => {
  apiFetch.mockImplementation(async (path: string) => {
    const consultations = mockConsultationsEmpty(path);
    if (consultations !== undefined) return consultations;
    if (path !== "/v1/coaching/requests/review-1") throw new Error(`Unexpected request: ${path}`);
    return baseRequest();
  });
  let confirmLeave: ((go: () => void) => boolean) | null = null;
  function Probe() {
    confirmLeave = useLeaveGuard().confirmLeave;
    return null;
  }
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root = createRoot(container);
  await act(async () => root.render(
    <LeaveGuardProvider>
      <Probe />
      <ReviewRequestPage />
    </LeaveGuardProvider>,
  ));
  try {
    if (!confirmLeave) throw new Error("confirmLeave を取得できませんでした");
    const go = vi.fn();
    let blocked = true;
    await act(async () => {
      blocked = confirmLeave!(go);
    });
    expect(blocked).toBe(false);
    expect(go).not.toHaveBeenCalled();
    expect(container.textContent).not.toContain("この添削の評価をつけてください");
  } finally {
    await teardown(root, container);
  }
});

it("コーチのプロフィールへのリンクも止まらない", async () => {
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
    expect(container.textContent).not.toContain("この添削の評価をつけてください");
  } finally {
    await teardown(root, container);
  }
});

it("評価データがあっても星セクションは出ず離脱できる", async () => {
  apiFetch.mockImplementation(async (path: string) => {
    const consultations = mockConsultationsEmpty(path);
    if (consultations !== undefined) return consultations;
    if (path !== "/v1/coaching/requests/review-1") throw new Error(`Unexpected request: ${path}`);
    return baseRequest({ rating: { stars: 3, helpful: null } });
  });
  const { container, root } = await renderPage();
  try {
    expect(container.textContent).not.toContain("この添削の評価");
    expect(container.querySelector('button[role="radio"]')).toBeNull();
    await click(listLink(container));
    expect(routerMock.push).toHaveBeenCalledWith("/me/coaching/requests");
  } finally {
    await teardown(root, container);
  }
});

it("helpful・通報の導線は残っている", async () => {
  apiFetch.mockImplementation(async (path: string) => {
    const consultations = mockConsultationsEmpty(path);
    if (consultations !== undefined) return consultations;
    if (path !== "/v1/coaching/requests/review-1") throw new Error(`Unexpected request: ${path}`);
    return baseRequest({ helpfulAsked: true });
  });
  const { container, root } = await renderPage();
  try {
    expect(container.textContent).toContain("アドバイスは役に立ちましたか");
    expect(container.textContent).toContain("この返答を通報する");
    expect(container.querySelector('button[role="radio"]')).toBeNull();
  } finally {
    await teardown(root, container);
  }
});

it("返答がない依頼ではそのまま離脱できる", async () => {
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

it("非表示の返答では星が出ず離脱できる", async () => {
  apiFetch.mockImplementation(async (path: string) => {
    const consultations = mockConsultationsEmpty(path);
    if (consultations !== undefined) return consultations;
    if (path !== "/v1/coaching/requests/review-1") throw new Error(`Unexpected request: ${path}`);
    return baseRequest({ response: { hidden: true } });
  });
  const { container, root } = await renderPage();
  try {
    expect(container.textContent).not.toContain("この添削の評価");
    expect(container.querySelector('button[role="radio"]')).toBeNull();
    await click(listLink(container));
    expect(routerMock.push).toHaveBeenCalledWith("/me/coaching/requests");
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
