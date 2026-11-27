/** @vitest-environment jsdom */
import { act, type MouseEvent as ReactMouseEvent, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { expect, it, vi } from "vitest";
import { LeaveGuardProvider } from "@/components/leave-guard";
import { installEnglish, setActiveLocale } from "@/lib/i18n";
import { en } from "@/lib/i18n-en";
import { reviewVideosPurgedNote } from "@/lib/review";
import { ApiRequestError } from "@/lib/api";
import { Header } from "@/components/header";
import { BottomNav } from "@/components/bottom-nav";
import ReviewRequestPage from "./page";

const routerMock = vi.hoisted(() => ({ push: vi.fn(), replace: vi.fn(), back: vi.fn() }));
const featureFlags = vi.hoisted(() => ({ coachList: true, supportList: false }));
const searchState = vi.hoisted(() => ({ paid: false }));
vi.mock("next/navigation", () => ({
  usePathname: () => "/coaching/requests/review-1",
  useParams: () => ({ id: "review-1" }),
  useRouter: () => routerMock,
  useSearchParams: () => new URLSearchParams(searchState.paid ? "paid=1" : ""),
}));
const authProfile = vi.hoisted(() => ({ current: null as Record<string, unknown> | null }));
vi.mock("@/components/auth-provider", () => ({
  useAuth: () => ({ session: { user: { id: "athlete" } }, token: "token", loading: false, profile: authProfile.current }),
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
  useLinkStatus: () => ({ pending: false }),
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
  get coachingCoachListEnabled() { return featureFlags.coachList; },
  get supportPlanListEnabled() { return featureFlags.supportList; },
}));

type RequestFixture = {
  response?: { hidden: boolean } | null;
  rating?: { stars: number | null; helpful: null } | null;
  coach?: { id?: string } | null;
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

const feedbackLabel = "この添削はどうでしたか（任意）";
const feedbackNotice = "感想は匿名で、7日後にコーチへ届きます。どの添削への感想かはコーチには分かりません。";

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
      <Header />
      <ReviewRequestPage />
      <BottomNav />
    </LeaveGuardProvider>,
  ));
  return { container, root };
}

async function teardown(root: Root, container: HTMLDivElement) {
  await act(async () => root.unmount());
  container.remove();
  vi.unstubAllGlobals();
  vi.clearAllMocks();
  featureFlags.coachList = true;
  featureFlags.supportList = false;
  setActiveLocale("ja");
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

// 本体 #984: 追加質問は販売終了。完了した依頼には入力欄・追加質問の導線を置かない。
// 本体 #1007 B8: 継続プランは休眠。導線は「新しい動画で依頼する」だけで、継続プラン一覧のフラグが有効でも
// 「継続プランを見る」「継続プランは準備中です」は出さない。
for (const supportList of [false, true]) {
  it(`返却後は入力欄・追加質問の導線を出さず、次の依頼の導線は「新しい動画で依頼する」だけ（継続プラン一覧フラグ=${supportList}）`, async () => {
    featureFlags.supportList = supportList;
    apiFetch.mockImplementation(async (path: string) => {
      const consultations = mockConsultationsEmpty(path);
      if (consultations !== undefined) return consultations;
      if (path !== "/v1/coaching/requests/review-1") throw new Error(`Unexpected request: ${path}`);
      return baseRequest();
    });
    const { container, root } = await renderPage();
    try {
      expect(container.textContent).toContain("腕振り");
      expect(container.querySelector("textarea")).toBeNull();
      expect(container.textContent).not.toContain("質問する（");
      expect(container.textContent).not.toContain("追加質問");
      const next = container.querySelector('nav[aria-label="次の依頼"]')!;
      expect([...next.querySelectorAll("a")].map((a) => [a.textContent, a.getAttribute("href")])).toEqual([["新しい動画で依頼する", "/coaching/new"]]);
      expect(next.children).toHaveLength(1);
      expect(container.textContent).not.toContain("継続プランを見る");
      expect(container.textContent).not.toContain("継続プランは準備中です");
      expect(container.querySelector('a[href="/coaching?tab=support"]')).toBeNull();
      expect(container.textContent).not.toContain("確認質問を送る（無料）");
    } finally {
      await teardown(root, container);
    }
  });
}

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

it("A1: 未評価の返却済み依頼でもコーチのプロフィールへのリンクは当面出さない（本体 #1020）", async () => {
  apiFetch.mockImplementation(async (path: string) => {
    const consultations = mockConsultationsEmpty(path);
    if (consultations !== undefined) return consultations;
    if (path !== "/v1/coaching/requests/review-1") throw new Error(`Unexpected request: ${path}`);
    return baseRequest({ coach: { id: "coach-1" }, extra: { video_url: "https://example.com/request.mp4" } });
  });
  const { container, root } = await renderPage();
  try {
    expect(container.querySelectorAll('a[href^="/coaches/"]')).toHaveLength(0);
    expect(container.textContent).not.toContain("このコーチのプロフィールを見る");
    const heading = container.querySelector("h1")!;
    expect(heading.textContent).toBe("スタンダード");
    const header = heading.parentElement!.parentElement!;
    expect(header.classList).toContain("items-start");
    expect(header.classList).toContain("justify-between");
    expect(header.children).toHaveLength(1);
    expect(header.firstElementChild!.textContent).toContain("送った日時");
    const question = [...container.querySelectorAll("p")].find((p) => p.textContent === "スタートの姿勢について")!;
    const response = [...container.querySelectorAll("h3")].find((h) => h.textContent === "良い点")!;
    expect(container.querySelector("video")).not.toBeNull();
    expect(response).toBeDefined();
    // 依頼内容のすぐ後に、1枚の回答ドキュメントが続く（本体 #984）。
    expect(question.closest("section[aria-label]")!.nextElementSibling).toBe(response.closest("article"));
    const outside = container.textContent!.replace(response.closest("article")!.textContent!, "");
    for (const text of ["テストコーチ", "100m", "10秒台", "実績"]) {
      expect(outside).not.toContain(text);
    }
    expect(routerMock.push).not.toHaveBeenCalledWith("/coaches/coach-1");
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

it("通報済みの返答は開き直しても最初から通報済み表示になる", async () => {
  for (let i = 0; i < 2; i++) {
    const { container, root } = await renderWith({ extra: { response_reported: true } });
    try {
      expect([...container.querySelectorAll("button")].some((b) => b.textContent === "この返答を通報する")).toBe(false);
      expect(container.textContent).toContain("通報しました。運営が確認します。");
    } finally {
      await teardown(root, container);
    }
  }
});

it("未通報フラグが false でも通報ボタンを表示する", async () => {
  const { container, root } = await renderWith({ extra: { response_reported: false } });
  try {
    expect([...container.querySelectorAll("button")].some((b) => b.textContent === "この返答を通報する")).toBe(true);
    expect(container.textContent).not.toContain("通報しました。運営が確認します。");
  } finally {
    await teardown(root, container);
  }
});

it.each(["success", "conflict"])("通報の結果が %s ならその画面内でも通報済み表示に切り替える", async (result) => {
  apiFetch.mockImplementation(async (path: string) => {
    if (path === "/v1/reports") {
      if (result === "conflict") throw new ApiRequestError(409, "already reported");
      return {};
    }
    const consultations = mockConsultationsEmpty(path);
    if (consultations !== undefined) return consultations;
    if (path !== "/v1/coaching/requests/review-1") throw new Error(`Unexpected request: ${path}`);
    return baseRequest({ extra: { response_reported: false } });
  });
  const { container, root } = await renderPage();
  try {
    const button = [...container.querySelectorAll("button")].find((b) => b.textContent === "この返答を通報する")!;
    await click(button);
    expect(apiFetch).toHaveBeenCalledWith("/v1/reports", expect.objectContaining({
      method: "POST",
      token: "token",
      body: JSON.stringify({ target_type: "review_response", target_id: "review-1", reason: "other", detail: "" }),
    }));
    expect(container.textContent).not.toContain("この返答を通報する");
    expect(container.textContent).toContain("通報しました。運営が確認します。");
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

it("返却後＋コーチ情報ありは「選んだコーチ」枠を出さない", async () => {
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
    // コーチ名は回答ドキュメントの見出しにだけ出る（本体 #984）。
    const doc = container.querySelector("article")!;
    expect(container.textContent!.replace(doc.textContent!, "")).not.toContain("デモコーチ");
  } finally {
    await unmountPage(container, root);
  }
});

it("返却前も「選んだコーチ」枠を出さない", async () => {
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
    expect(container.textContent).not.toContain("選んだコーチ：");
    expect(container.textContent).not.toContain("料金500円");
  } finally {
    await unmountPage(container, root);
  }
});

it("返却後＋コーチなしも「選んだコーチ」枠を出さない", async () => {
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
    expect(container.textContent).not.toContain("選んだコーチ：");
  } finally {
    await unmountPage(container, root);
  }
});

it("期限切れは選んだコーチ枠を出さず別のコーチへのリンクを残す", async () => {
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
    expect(container.textContent).not.toContain("選んだコーチ：");
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

// 支払いの開き直しの購入前チェックは、未成年（の可能性がある人）だけに出す（本体 #1045 の T8・T9）。
it.each([
  { required: false, shown: false },
  { required: true, shown: true },
])("お支払い待ちの開き直し: purchase_notice_required=$required なら購入前チェックを出す=$shown", async ({ required, shown }) => {
  authProfile.current = { purchase_notice_required: required };
  const posted: Record<string, unknown>[] = [];
  apiFetch.mockImplementation(async (path: string, init?: { method?: string; body?: string }) => {
    if (path === "/v1/coaching/requests/review-1/checkout" && init?.method === "POST") {
      posted.push(JSON.parse(init.body ?? "{}"));
      return unpaidRequest("awaiting_payment");
    }
    if (path === "/v1/coaching/requests/review-1") return unpaidRequest("awaiting_payment");
    if (path.endsWith("/referral")) return { referral: null };
    return {};
  });
  const { container, root } = await renderPage();
  try {
    const box = container.querySelector<HTMLInputElement>('input[type="checkbox"]');
    const pay = [...container.querySelectorAll("button")].find((b) => b.textContent === "確定してお支払いへ")!;
    expect(container.textContent?.includes("未成年の方は保護者に許可を取ってください")).toBe(shown);
    expect(!!box).toBe(shown);
    expect(pay.disabled).toBe(shown);
    if (!shown) {
      await click(pay);
      expect(posted).toHaveLength(1);
      expect(posted[0]).not.toHaveProperty("guardian_permission");
    }
  } finally {
    authProfile.current = null;
    await unmountPage(container, root);
  }
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
      title: "スタンダードの依頼をキャンセルしますか？",
      body: "まだお支払いは済んでいません。取り消すと、この依頼ではお支払いができなくなります。",
      confirmLabel: "依頼をキャンセルする",
    }));
    expect(canceled).toBe(true);
    expect(container.textContent).not.toContain("依頼をキャンセルしました");
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

// 本体 #1068: 「依頼内容」枠の中に「相談内容」の見出しは出さず、本文だけを出す。判定は枠の中だけに絞る。
function expectQuestionWithoutHeading(container: HTMLDivElement, body = "スタートの姿勢について") {
  const section = container.querySelector('section[aria-label="依頼内容"], section[aria-label="Your request"]')!;
  expect(section).not.toBeNull();
  const headings = [...section.querySelectorAll("h1, h2, h3, h4, h5, h6")].map((h) => h.textContent);
  expect(headings).not.toContain("相談内容");
  expect(headings).not.toContain("Your question");
  expect([...section.querySelectorAll("p")].some((p) => p.textContent === body)).toBe(true);
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
    expectQuestionWithoutHeading(container);
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
    expectQuestionWithoutHeading(container);
  } finally {
    await teardown(root, container);
  }
});

it("動画なし（悩みのみ）の依頼は動画も保存期間の注記も出さず、相談内容だけを出す", async () => {
  const { container, root } = await renderWith({ response: null, extra: { status: "in_review", due_at: new Date().toISOString() } });
  try {
    expect(container.querySelector("video")).toBeNull();
    expect(container.textContent).not.toContain(reviewVideosPurgedNote());
    expect(dlTerms(container)).toEqual([]);
    expectQuestionWithoutHeading(container);
  } finally {
    await teardown(root, container);
  }
});

it("動画が削除済みの依頼は保存期間の注記と相談内容を出す", async () => {
  const { container, root } = await renderWith({ response: null, extra: { status: "in_review", due_at: new Date().toISOString(), videos_purged: true } });
  try {
    expect(container.querySelector("video")).toBeNull();
    expect(container.textContent).toContain(reviewVideosPurgedNote());
    expect(container.textContent).toContain("スタートの姿勢について");
    expectQuestionWithoutHeading(container);
  } finally {
    await teardown(root, container);
  }
});

it("返却後は相談内容を残しコーチの名前と種目は出さない", async () => {
  const { container, root } = await renderWith({ coach: { id: "coach-1" } });
  try {
    expect(container.textContent).not.toContain("誰に見てもらうか");
    expect(dlTerms(container)).toEqual([]);
    expect(container.textContent).toContain("スタートの姿勢について");
    // コーチ名は回答ドキュメントの見出しにだけ出る（本体 #984）。
    const outside = container.textContent!.replace(container.querySelector("article")?.textContent ?? "", "");
    expect(outside).not.toContain("テストコーチ");
    expect(container.textContent).not.toContain("100m");
    expectQuestionWithoutHeading(container);
  } finally {
    await teardown(root, container);
  }
});

it("英語表示でも「依頼内容」枠に Your question の見出しを出さず、相談文は出す（本体 #1068）", async () => {
  installEnglish(en);
  setActiveLocale("en");
  let rendered: Awaited<ReturnType<typeof renderWith>> | undefined;
  try {
    rendered = await renderWith({ response: null, extra: { status: "in_review", due_at: new Date().toISOString() } });
    expect(rendered.container.querySelector('section[aria-label="Your request"]')).not.toBeNull();
    expectQuestionWithoutHeading(rendered.container);
  } finally {
    setActiveLocale("ja");
    if (rendered) await teardown(rendered.root, rendered.container);
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

// 本体 #916 B1・B2: 選手（ja）には、海外コーチの差し戻しの質問が日本語メイン＋英語原文の併記で出る。自分の相談文は英訳があっても原文だけ。
it("B1: 差し戻しの質問の和訳を主に英語の原文を併記し、自分の相談文は原文だけ", async () => {
  const info = { body: "Which part of the race should I look at?", reason: "scene", requested_at: new Date().toISOString(), from_status: "in_review", resubmit_by: new Date(Date.now() + 3600000).toISOString(), original_question: null, video_replaced: false, resubmitted_at: null };
  const { container, root } = await renderWith({ response: null, extra: { status: "needs_info", due_at: new Date().toISOString(), info_request: info, info_request_ja: "レースのどの区間を見ればよいですか？", question_en: "About my start posture" } });
  try {
    expect(container.textContent).toContain("レースのどの区間を見ればよいですか？");
    expect(container.querySelector("p[lang='en']")?.textContent).toBe("Which part of the race should I look at?");
    expect(container.textContent).toContain("スタートの姿勢について");
    expect(container.textContent).not.toContain("About my start posture");
  } finally {
    await teardown(root, container);
  }
});

// 返却後に任意で書ける匿名の感想（本体 #911）。
function mockFeedbackRequest(extra: Record<string, unknown> = {}) {
  apiFetch.mockImplementation(async (path: string, init?: { method?: string; body?: string }) => {
    const consultations = mockConsultationsEmpty(path);
    if (consultations !== undefined) return consultations;
    if (path === "/v1/coaching/requests/review-1/feedback" && init?.method === "POST") {
      return { feedback_submitted: true, visible_at: new Date(Date.now() + 7 * 86400_000).toISOString() };
    }
    if (path !== "/v1/coaching/requests/review-1") throw new Error(`Unexpected request: ${path}`);
    return baseRequest({ extra });
  });
}

function feedbackTextarea(container: HTMLDivElement) {
  return container.querySelector<HTMLTextAreaElement>(`textarea[aria-label="${feedbackLabel}"]`);
}

for (const status of ["returned", "clarifying"]) {
  for (const submitted of [false, true]) {
    it(`A1/A2/C1/C2: ${status} 送信済み=${submitted} では感想表示がなく、未送信だけ戻る先を変える`, async () => {
      mockFeedbackRequest({ status, feedback_submitted: submitted });
      const { container, root } = await renderPage();
      try {
        expect(feedbackTextarea(container)).toBeNull();
        for (const text of [feedbackLabel, feedbackNotice, "感想を送る", "感想を送りました"]) expect(container.textContent).not.toContain(text);
        expect(container.textContent).toContain("腕振り");
        expect(container.textContent).toContain("この返答を通報する");
        expect(container.querySelector('textarea[aria-label="追加質問の質問"]')).toBeNull();
        routerMock.push.mockClear();
        await click(container.querySelector('button[aria-label="戻る"]')!);
        if (submitted) {
          expect(routerMock.push).not.toHaveBeenCalled();
          expect(routerMock.replace).toHaveBeenCalledWith("/coaching");
        } else {
          expect(routerMock.push).toHaveBeenCalledWith("/coaching/requests/review-1/feedback");
          expect(routerMock.back).not.toHaveBeenCalled();
        }
      } finally { await teardown(root, container); }
    });
  }
}

for (const status of ["awaiting_payment", "queued", "offered", "in_review", "needs_info", "expired"]) {
  it(`C2: ${status} では戻る先を差し替えない`, async () => {
    mockFeedbackRequest({ status, due_at: new Date(Date.now() + 3600_000).toISOString() });
    const { container, root } = await renderPage();
    try {
      expect(feedbackTextarea(container)).toBeNull();
      routerMock.push.mockClear();
      await click(container.querySelector('button[aria-label="戻る"]')!);
      expect(routerMock.push).not.toHaveBeenCalled();
    } finally { await teardown(root, container); }
  });
}

it("C2: 返答なしでは戻る先を差し替えない", async () => {
  const { container, root } = await renderWith({ response: null });
  try {
    await click(container.querySelector('button[aria-label="戻る"]')!);
    expect(routerMock.push).not.toHaveBeenCalled();
  } finally { await teardown(root, container); }
});

it("C4: 未送信でも依頼一覧リンクと下部タブはそれぞれのリンク先へ進む", async () => {
  mockFeedbackRequest();
  const { container, root } = await renderPage();
  try {
    await click(listLink(container));
    expect(routerMock.push).toHaveBeenLastCalledWith("/me/coaching/requests");
    await click(container.querySelector('nav a[href="/coaching"]')!);
    expect(routerMock.push).toHaveBeenLastCalledWith("/coaching");
    expect(routerMock.push).not.toHaveBeenCalledWith("/coaching/requests/review-1/feedback");
  } finally { await teardown(root, container); }
});

it("依頼内容（動画→相談文）は吹き出しにせず上にまとめ、回答は1枚のドキュメントとして続く（本体 #984）", async () => {
  const { container, root } = await renderWith({ extra: { created_at: "2026-11-17T21:19:00Z", video_url: "https://example.com/run.mp4", thumbnail_url: "https://example.com/run.jpg" } });
  try {
    const question = [...container.querySelectorAll("p")].find((p) => p.textContent === "スタートの姿勢について")!;
    const request = question.closest('section[aria-label="依頼内容"]')!;
    expect(request).not.toBeNull();
    expect(container.querySelector(".bg-blue-500\\/25")).toBeNull();
    expect(container.querySelector(".justify-end")).toBeNull();
    expect(container.querySelectorAll("dl")).toHaveLength(0);
    const video = container.querySelector("video")!;
    expect(video.closest("section")).toBe(request);
    expect(video.classList).toContain("max-w-[260px]");
    expect(video.classList).toContain("max-h-[300px]");
    expect(video.classList).toContain("mx-auto");
    expect(video.controls).toBe(true);
    expect(video.getAttribute("poster")).toBe("https://example.com/run.jpg");
    expect(video.compareDocumentPosition(question) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    const response = [...container.querySelectorAll("h3")].find((h) => h.textContent === "良い点")!.closest("article")!;
    expect(request.nextElementSibling).toBe(response);
    expect(response.querySelectorAll("time")).toHaveLength(1);
  } finally { await teardown(root, container); }
});

it("対応中は依頼内容だけで空の回答ドキュメントを出さず、見出し内の期限表示とキャンセル操作を残す", async () => {
  const { container, root } = await renderWith({ response: null, extra: { status: "in_review", started_at: null, due_at: new Date(Date.now() + 86400000).toISOString() } });
  try {
    expect(container.querySelector("article")).toBeNull();
    expect(container.querySelector('nav[aria-label="次の依頼"]')).toBeNull();
    expect([...container.querySelectorAll("h3")].some((h) => h.textContent === "良い点")).toBe(false);
    const deadline = [...container.querySelectorAll("p")].find((p) => p.textContent?.startsWith("返却期限:"))!;
    expect(deadline.previousElementSibling?.textContent).toMatch(/^送った日時 /);
    expect(deadline.parentElement?.querySelector("h1")?.textContent).toBe("スタンダード");
    expect(deadline.className).toBe(deadline.previousElementSibling?.className);
    expect(deadline.parentElement?.className).not.toContain("bg-");
    expect([...container.querySelectorAll("button")].some((b) => b.textContent === "依頼をキャンセルする")).toBe(true);
    expect(listLink(container)).toBeTruthy();
  } finally { await teardown(root, container); }
});

it("Stripeからの支払い同期後も状態・支払い文は出さず、紹介をcompactにする", async () => {
  searchState.paid = true;
  let synced = false;
  const paid = baseRequest({ response: null, extra: { status: "in_review", started_at: null, due_at: new Date(Date.now() + 86400000).toISOString(), payment: { method: "card", status: "captured", amount_yen: 1000 }, requested_coach_id: "c1", requested_coach_name: "指名コーチ", tickets: { quantity: 1, status: "consumed" } } });
  apiFetch.mockImplementation(async (path: string) => {
    if (path.endsWith("/payment/sync")) { synced = true; return paid; }
    if (path.endsWith("/referral")) return { referral: { code: "ABC123", active: false, friend_count: 2, granted_at: "2026-11-14T09:00:00Z", expires_at: "2027-05-14T09:00:00Z" } };
    if (path === "/v1/coaching/requests/review-1") return synced ? paid : { ...paid, status: "awaiting_payment", payment: null };
    throw new Error(`Unexpected request: ${path}`);
  });
  const { container, root } = await renderPage();
  try {
    expect(apiFetch).toHaveBeenCalledWith("/v1/coaching/requests/review-1/payment/sync", { method: "POST", token: "token" });
    expectRemovedSummary(container);
    expect(cancelButton(container)).toBeDefined();
    expect(container.textContent).toContain("あなたの紹介ID ABC123");
    expect(container.textContent).toContain("¥500クーポンをプレゼントしました");
    expect(container.querySelector("section h2")?.classList).toContain("text-sm");
    for (const text of ["購入すると、紹介IDが発行されます。", "付与日", "有効期限", "このIDは終了しました。"]) expect(container.textContent).not.toContain(text);
  } finally { searchState.paid = false; await teardown(root, container); }
});

function expectRemovedSummary(container: HTMLDivElement) {
  expect(container.querySelector(".text-center.text-xs.text-slate-400")).toBeNull();
  for (const text of ["添削中", "選んだコーチ：", "料金500円", "チケット2枚：", "チケット1枚：", "現在コーチが依頼に対応しています", "500円を仮押さえしています", "500円をお支払い済みです", "500円の仮押さえを取り消しました", "500円を返金しました"]) expect(container.textContent).not.toContain(text);
}

it.each(["authorized", "captured", "canceled", "refunded"])("A1–A3: %s でも見出しの期限だけを表示する", async (status) => {
  for (const started of [false, true]) {
    const { container, root } = await renderWith({ response: null, extra: {
      status: "in_review", created_at: "2026-11-18T21:00:00Z", due_at: "2026-11-20T21:00:00Z",
      started_at: started ? "2026-11-18T22:00:00Z" : null, requested_coach_id: "coach-1", requested_coach_name: "デモコーチ", total_price_yen: 500,
      tickets: { quantity: 2, status: "reserved" }, payment: { method: "card", status, amount_yen: 500 },
    } });
    try {
      const heading = container.querySelector("h1")!;
      expect(heading.textContent).toBe("スタンダード");
      const sent = heading.nextElementSibling!;
      expect(sent.textContent).toBe("送った日時 2026/11/19 6:00");
      expect(sent.nextElementSibling?.textContent).toBe("返却期限: 2026/11/21 6:00");
      expect(sent.nextElementSibling?.className).toBe(sent.className);
      expectRemovedSummary(container);
      if (started) expect(cancelButton(container)).toBeUndefined();
    } finally { await teardown(root, container); }
  }
});

it("A4: 再提出期限は差し戻し履歴だけに残す", async () => {
  const { container, root } = await renderWith({ response: null, extra: {
    status: "needs_info", due_at: "2026-11-20T21:00:00Z",
    info_request: { body: "動画を補足してください", reason: "scene", requested_at: "2026-11-18T21:00:00Z", from_status: "in_review", resubmit_by: "2026-11-20T21:00:00Z", original_question: null, video_replaced: false, resubmitted_at: null },
  } });
  try {
    expectRemovedSummary(container);
    expect(container.textContent).not.toContain("再提出の期限:");
    expect(container.textContent).not.toContain("返却期限:");
    expect(container.textContent).toContain("再提出の期限：2026/11/21 6:00（過ぎると依頼は終了し、お支払いは取り消し・返金されます）");
  } finally { await teardown(root, container); }
});

it.each(["returned", "expired", "awaiting_payment"])("A5・A6・A8: %s の既存表示・操作を残す", async (status) => {
  const { container, root } = await renderWith({ response: status === "returned" ? { hidden: false } : null, coach: status === "returned" ? { id: "coach-1" } : null, extra: {
    status, due_at: "2026-11-20T21:00:00Z", requested_coach_id: "coach-1", requested_coach_name: "デモコーチ", total_price_yen: 500, payment: { method: "card", status: "captured", amount_yen: 500 },
  } });
  try {
    expectRemovedSummary(container);
    expect(container.textContent).not.toContain("返却期限:");
    expect([...container.querySelectorAll("p")].some((p) => p.textContent === "返却済み")).toBe(false);
    if (status === "returned") {
      expect(container.textContent).toContain("腕振り");
      expect(container.querySelector('a[href^="/coaches/"]')).toBeNull();
      expect(container.textContent).not.toContain("このコーチのプロフィールを見る");
    }
    if (status === "expired") expect(container.querySelector('a[href="/coaching/new"]')?.textContent).toBe("別のコーチを選んで申し込む");
    if (status === "awaiting_payment") {
      expect(cancelButton(container)).toBeDefined();
      expect(container.textContent).toContain("2時間たってもお支払いがなければ、この依頼は消えます。");
      expect([...container.querySelectorAll("button")].some((b) => b.textContent === "確定してお支払いへ")).toBe(true);
    }
  } finally { await teardown(root, container); }
});

it("B1: 英語でも既存キーの返却期限が送った日時の直下に出る", async () => {
  installEnglish(en);
  setActiveLocale("en");
  let rendered: Awaited<ReturnType<typeof renderWith>> | undefined;
  try {
    rendered = await renderWith({ response: null, extra: { status: "in_review", due_at: "2026-11-20T21:00:00Z" } });
    const sent = rendered.container.querySelector("h1")?.nextElementSibling;
    expect(sent?.textContent).toMatch(/^Sent /);
    expect(sent?.nextElementSibling?.textContent).toBe(en["返却期限: {v0}"].replace("{v0}", "Nov 21, 2026, 6:00 AM"));
  } finally {
    setActiveLocale("ja");
    if (rendered) await teardown(rendered.root, rendered.container);
  }
});

it.each([
  { label: "一覧無効", coach: { id: "coach-1" }, list: false },
  { label: "コーチなし", coach: null, list: true },
  { label: "IDなし", coach: {}, list: true },
])("$label では詳細リンクを出さず返答を表示する", async ({ coach, list }) => {
  featureFlags.coachList = list;
  apiFetch.mockImplementation(async (path: string) => mockConsultationsEmpty(path) ?? baseRequest({ coach }));
  const { container, root } = await renderPage();
  try {
    expect(container.querySelector('a[href^="/coaches/"]')).toBeNull();
    expect(container.textContent).toContain("腕振り");
    expect(container.textContent).toContain("接地位置");
  } finally { await teardown(root, container); }
});

it("返答前は詳細リンクを出さず、選んだコーチの中央表示も出さない", async () => {
  apiFetch.mockImplementation(async () => baseRequest({ response: null, extra: {
    status: "in_review", due_at: "2026-12-01T00:00:00Z", requested_coach_id: "coach-1", requested_coach_name: "選択済みコーチ",
  } }));
  const { container, root } = await renderPage();
  try {
    expect(container.querySelector('a[href^="/coaches/"]')).toBeNull();
    expect(container.textContent).not.toContain("選んだコーチ：");
    expect(container.textContent).not.toContain("選択済みコーチ");
  } finally { await teardown(root, container); }
});

it("A3: 運営非表示の返答でも詳細リンクは出さず、非表示の案内は従来どおり出す", async () => {
  apiFetch.mockImplementation(async (path: string) => mockConsultationsEmpty(path) ?? baseRequest({
    response: { hidden: true }, coach: { id: "coach/1" },
  }));
  const { container, root } = await renderPage();
  try {
    expect(container.querySelector('a[href^="/coaches/"]')).toBeNull();
    expect(container.textContent).not.toContain("このコーチのプロフィールを見る");
    expect(container.textContent).toContain("この返答は運営が非表示にしました。");
  } finally { await teardown(root, container); }
});

it("A4: 英語表示でも右上リンクは出さず、見出しと返答は英語で出す", async () => {
  installEnglish(en);
  setActiveLocale("en");
  apiFetch.mockImplementation(async (path: string) => mockConsultationsEmpty(path) ?? baseRequest({ coach: { id: "coach-1" } }));
  const { container, root } = await renderPage();
  try {
    expect(container.querySelector('a[href^="/coaches/"]')).toBeNull();
    expect(container.textContent).not.toContain(en["このコーチのプロフィールを見る"]);
    expect(container.textContent).not.toContain("View this coach's profile");
    const heading = container.querySelector("h1")!;
    expect(heading.textContent).toBe(en["スタンダード"]);
    expect(heading.nextElementSibling?.textContent).toMatch(/^Sent /);
    expect(heading.parentElement!.parentElement!.children).toHaveLength(1);
    expect([...container.querySelectorAll("h3")].some((h) => h.textContent === en["良い点"])).toBe(true);
  } finally { await teardown(root, container); }
});
