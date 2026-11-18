/** @vitest-environment jsdom */
import { act, StrictMode, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { externalTripGapMs, recoveryGraceMs, recoveryRecheckMs, resumePagehideKey } from "@/lib/resume-recovery";
import { ResumeRecovery } from "./resume-recovery";

let pathname = "/";
let search = "";
const refresh = vi.fn();
const router = { refresh };
let profile: { nickname: string; event: string } | null = null;
vi.mock("./auth-provider", () => ({ useAuth: () => ({ profile }) }));
vi.mock("next/navigation", () => ({
  usePathname: () => pathname,
  useRouter: () => router,
  useSearchParams: () => ({ toString: () => search }),
}));
vi.mock("next/link", () => ({ default: ({ href, children, ...props }: { href: string; children: ReactNode }) => <a href={href} {...props}>{children}</a> }));

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  vi.useFakeTimers();
  pathname = "/";
  search = "";
  refresh.mockReset();
  profile = null;
  vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockReturnValue({ height: 60 } as DOMRect);
  window.sessionStorage.clear();
  document.body.innerHTML = "";
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  document.body.innerHTML = "";
  vi.useRealTimers();
  vi.restoreAllMocks();
});

async function render() {
  await act(async () => root.render(<ResumeRecovery />));
}

function pastPageHide() {
  window.sessionStorage.setItem(resumePagehideKey, String(Date.now() - externalTripGapMs - 60_000));
}

it("ふだんの表示では何もしない", async () => {
  document.body.insertAdjacentHTML("afterbegin", "<header><button>戻る</button></header><main><p>本文</p></main>");
  await render();
  await act(async () => vi.advanceTimersByTime(30_000));
  expect(refresh).not.toHaveBeenCalled();
  expect(container.querySelector('[role="alert"]')).toBeNull();
});

it("外部往復のあとにヘッダーと本文がなければ、取り直してから再読み込みの導線を出す", async () => {
  pastPageHide();
  document.body.insertAdjacentHTML("afterbegin", "<main></main>");
  await render();
  await act(async () => vi.advanceTimersByTime(recoveryGraceMs));
  expect(refresh).toHaveBeenCalledOnce();
  await act(async () => vi.advanceTimersByTime(recoveryRecheckMs));
  const alert = container.querySelector('[role="alert"]');
  expect(alert?.textContent).toContain("再読み込み");
});

it("外部往復のあとに正常に描画されていれば静かに終わる", async () => {
  pastPageHide();
  document.body.insertAdjacentHTML("afterbegin", "<header><button>戻る</button></header><main><p>本文</p></main>");
  await render();
  await act(async () => vi.advanceTimersByTime(recoveryGraceMs + recoveryRecheckMs));
  expect(refresh).not.toHaveBeenCalled();
  expect(container.querySelector('[role="alert"]')).toBeNull();
});

it("取り直しで直れば導線は出ない", async () => {
  pastPageHide();
  document.body.insertAdjacentHTML("afterbegin", "<main></main>");
  await render();
  await act(async () => vi.advanceTimersByTime(recoveryGraceMs));
  expect(refresh).toHaveBeenCalledOnce();
  document.querySelector("main")!.innerHTML = "<p>本文</p>";
  document.body.insertAdjacentHTML("afterbegin", "<header><button>戻る</button></header>");
  await act(async () => vi.advanceTimersByTime(recoveryRecheckMs));
  expect(container.querySelector('[role="alert"]')).toBeNull();
});

it("ヘッダーを出さない画面ではヘッダーがなくても導線は出ない", async () => {
  pathname = "/me/profile";
  search = "onboarding=1";
  pastPageHide();
  document.body.insertAdjacentHTML("afterbegin", "<main><p>入力</p></main>");
  await render();
  await act(async () => vi.advanceTimersByTime(recoveryGraceMs + recoveryRecheckMs));
  expect(container.querySelector('[role="alert"]')).toBeNull();
});

it("導線は閉じることができる", async () => {
  pastPageHide();
  document.body.insertAdjacentHTML("afterbegin", "<main></main>");
  await render();
  await act(async () => vi.advanceTimersByTime(recoveryGraceMs + recoveryRecheckMs));
  const close = container.querySelector('button[aria-label="閉じる"]') as HTMLButtonElement | null;
  expect(close).not.toBeNull();
  await act(async () => close!.click());
  expect(container.querySelector('[role="alert"]')).toBeNull();
});

it("bfcache の復帰でも確かめる", async () => {
  document.body.insertAdjacentHTML("afterbegin", "<main></main>");
  await render();
  await act(async () => vi.advanceTimersByTime(1_000));
  expect(refresh).not.toHaveBeenCalled();
  await act(async () => {
    const event = new Event("pageshow") as Event & { persisted?: boolean };
    event.persisted = true;
    window.dispatchEvent(event);
  });
  await act(async () => vi.advanceTimersByTime(recoveryGraceMs + recoveryRecheckMs));
  expect(refresh).toHaveBeenCalledOnce();
  expect(container.querySelector('[role="alert"]')).not.toBeNull();
});

async function navigate(path: string, query = "") {
  pathname = path;
  search = query;
  await render();
}
async function tick(ms = recoveryGraceMs) {
  await act(async () => vi.advanceTimersByTime(ms));
}
async function restoreBfcache() {
  await act(async () => window.dispatchEvent(new PageTransitionEvent("pageshow", { persisted: true })));
}

it.each([
  ["/coaching/requests/request", "paid=1"],
  ["/coaching/personal/contract", "paid=1"],
  ["/coaching/requests/request", "consultation=question"],
  ["/coaching/personal/contract", "consultation=question"],
  ["/me/coaching/tickets", "purchase=tickets"],
  ["/coach", "stripe=return"],
  ["/guardian/paid", ""],
  ["/guardian/paid", "consultation=question"],
])("正常な戻り画面 %s?%s のチェック完了後も最初の遷移先を検出する", async (path, query) => {
  pathname = path;
  search = query;
  pastPageHide();
  document.body.insertAdjacentHTML("afterbegin", "<header><button>戻る</button></header><main><p>本文</p></main>");
  await render();
  await tick();
  expect(refresh).not.toHaveBeenCalled();
  document.querySelector("header")!.remove();
  await navigate("/coaching");
  await tick();
  expect(refresh).toHaveBeenCalledOnce();
  await tick(recoveryRecheckMs);
  expect(container.querySelector('[role="alert"]')).not.toBeNull();
  await tick(60_000);
  expect(refresh).toHaveBeenCalledOnce();
});

it("完了後の新しい bfcache 復帰とその最初の遷移先もチェックする", async () => {
  pastPageHide();
  document.body.insertAdjacentHTML("afterbegin", "<header><button>戻る</button></header><main><p>本文</p></main>");
  await render();
  await tick();
  await restoreBfcache();
  await tick();
  document.querySelector("header")!.remove();
  await navigate("/coaching");
  await tick();
  expect(refresh).toHaveBeenCalledOnce();
});

it("8秒以内の遷移では移り先の猶予を取り直す", async () => {
  pastPageHide();
  document.body.insertAdjacentHTML("afterbegin", "<header><button>戻る</button></header><main><p>本文</p></main>");
  await render();
  await tick(recoveryGraceMs - 1);
  document.querySelector("header")!.remove();
  await navigate("/coaching");
  await tick(1);
  expect(refresh).not.toHaveBeenCalled();
  await tick(recoveryGraceMs - 1);
  expect(refresh).toHaveBeenCalledOnce();
});

it("通常のタブ切替ではチェック自体が走らない", async () => {
  const query = vi.spyOn(document, "querySelector");
  await render();
  await navigate("/coaching");
  await tick();
  await navigate("/memo");
  await tick();
  expect(query).not.toHaveBeenCalled();
  expect(refresh).not.toHaveBeenCalled();
});

it("正常な戻り画面と最初の移り先が完了したら通常の遷移では再検出しない", async () => {
  pastPageHide();
  document.body.insertAdjacentHTML("afterbegin", "<header><button>戻る</button></header><main><p>本文</p></main>");
  await render();
  await tick();
  await navigate("/coaching");
  await tick();
  document.querySelector("header")!.remove();
  await navigate("/memo");
  await tick(60_000);
  expect(refresh).not.toHaveBeenCalled();
  expect(container.querySelector('[role="alert"]')).toBeNull();
});

it.each([
  ["/reset-password/update", "", false],
  ["/me/profile", "onboarding=1", false],
  ["/me/profile", "", true],
])("正当に非表示の %s?%s では誤検出しない", async (path, query, incomplete) => {
  pathname = path;
  search = query;
  if (incomplete) profile = { nickname: "", event: "" };
  pastPageHide();
  document.body.insertAdjacentHTML("afterbegin", "<main><p>入力</p></main>");
  await render();
  await tick(recoveryGraceMs + recoveryRecheckMs);
  expect(refresh).not.toHaveBeenCalled();
  expect(container.querySelector('[role="alert"]')).toBeNull();
  // 認証・プロフィール復元後も判定は最新の状態を使う。
  if (incomplete) {
    profile = { nickname: "選手", event: "100m" };
    await render();
  }
  await navigate("/coaching");
  await tick();
  expect(refresh).toHaveBeenCalledOnce();
});

it.each(["empty", "collapsed", "hidden"])("存在するが %s のヘッダーも検出する", async (kind) => {
  pastPageHide();
  document.body.insertAdjacentHTML("afterbegin", '<header><button>戻る</button></header><main><p>本文</p></main>');
  const header = document.querySelector("header")!;
  if (kind === "empty") header.innerHTML = "<div></div>";
  if (kind === "collapsed") vi.spyOn(header, "getBoundingClientRect").mockReturnValue({ height: 0 } as DOMRect);
  if (kind === "hidden") header.style.visibility = "hidden";
  await render();
  await tick(recoveryGraceMs + recoveryRecheckMs);
  expect(refresh).toHaveBeenCalledOnce();
  expect(container.querySelector('[role="alert"]')).not.toBeNull();
});

it("初期 pageshow は外部復帰のタイマーを取り消さない", async () => {
  pastPageHide();
  document.body.insertAdjacentHTML("afterbegin", "<main><p>本文</p></main>");
  await render();
  await act(async () => window.dispatchEvent(new PageTransitionEvent("pageshow", { persisted: false })));
  await tick();
  expect(refresh).toHaveBeenCalledOnce();
});

it("非表示復帰でも完了状態を解除し最初の遷移先を調べる", async () => {
  pastPageHide();
  document.body.insertAdjacentHTML("afterbegin", '<header><button>戻る</button></header><main><p>本文</p></main>');
  await render();
  await tick();
  const hidden = vi.spyOn(document, "hidden", "get");
  hidden.mockReturnValue(true);
  await act(async () => document.dispatchEvent(new Event("visibilitychange")));
  await tick(15_000);
  hidden.mockReturnValue(false);
  await act(async () => document.dispatchEvent(new Event("visibilitychange")));
  await tick();
  document.querySelector("header")!.remove();
  await navigate("/coaching");
  await tick();
  expect(refresh).toHaveBeenCalledOnce();
});

it("戻り画面でクエリだけ更新しても最初の画面遷移の予約を失わない", async () => {
  pathname = "/coaching/requests/request";
  search = "paid=1";
  pastPageHide();
  document.body.insertAdjacentHTML("afterbegin", '<header><button>戻る</button></header><main><p>本文</p></main>');
  await render();
  await tick();
  await navigate(pathname);
  await tick();
  document.querySelector("header")!.remove();
  await navigate("/coaching");
  await tick();
  expect(refresh).toHaveBeenCalledOnce();
});

it("取り直し中に遷移しても同じ復帰で2回取り直さない", async () => {
  pastPageHide();
  document.body.insertAdjacentHTML("afterbegin", "<main><p>本文</p></main>");
  await render();
  await tick();
  await navigate("/coaching");
  await tick(recoveryGraceMs + recoveryRecheckMs);
  expect(refresh).toHaveBeenCalledOnce();
  expect(container.querySelector('[role="alert"]')).not.toBeNull();
});

it("Strict Mode の effect 再登録でも外部復帰のチェックを失わない", async () => {
  pastPageHide();
  document.body.insertAdjacentHTML("afterbegin", "<main><p>本文</p></main>");
  await act(async () => root.render(<StrictMode><ResumeRecovery /></StrictMode>));
  await tick();
  expect(refresh).toHaveBeenCalledOnce();
});

it("戻り画面は正常でも最初の移り先で本文が空なら復旧する", async () => {
  pastPageHide();
  document.body.insertAdjacentHTML("afterbegin", '<header><button>戻る</button></header><main><p>本文</p></main>');
  await render();
  await tick();
  document.querySelector("main")!.innerHTML = "";
  await navigate("/coaching");
  await tick(recoveryGraceMs + recoveryRecheckMs);
  expect(refresh).toHaveBeenCalledOnce();
  expect(container.querySelector('[role="alert"]')).not.toBeNull();
});
