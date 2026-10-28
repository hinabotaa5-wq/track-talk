/** @vitest-environment jsdom */
import { act, type ComponentProps } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { MainTabLink } from "./main-tab-link";
import { ScreenLoader } from "./screen-loader";
import { clearLinkPending, markLinkPending } from "@/lib/pending-link";
import { HomePostFab, PostFab } from "./post-fab";
import { ComposerPage } from "./composer-page";
import UploadPage from "@/app/upload/page";

const state = vi.hoisted(() => ({
  session: {} as object | null, loading: false, pending: false,
  ready: null as (() => void) | null, prefetch: undefined as unknown,
  replace: vi.fn(),
  search: "",
}));
vi.mock("next/link", () => ({
  default: ({ prefetch, children, ...props }: ComponentProps<"a"> & { prefetch?: unknown }) => {
    state.prefetch = prefetch;
    return <a {...props}>{children}</a>;
  },
  useLinkStatus: () => ({ pending: state.pending }),
}));
vi.mock("@/lib/after-startup", () => ({ afterStartup: (callback: () => void) => { state.ready = callback; return () => {}; } }));
vi.mock("./auth-provider", () => ({ useAuth: () => ({ session: state.session, loading: state.loading, sessionLoading: state.loading }) }));
vi.mock("next/navigation", () => ({
  usePathname: () => "/posts/new",
  useRouter: () => ({ replace: state.replace }),
  useSearchParams: () => new URLSearchParams(state.search),
}));
vi.mock("./leave-guard", () => ({ LeaveGuardArea: ({ children }: { children: React.ReactNode }) => <div>{children}</div> }));
let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  state.session = {}; state.loading = false; state.pending = false; state.ready = null; state.replace.mockReset(); state.search = "";
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });
it("起動後に投稿画面全体を先読みし、遷移中は反応を表示する", async () => {
  await act(async () => root.render(<PostFab />));
  expect(state.prefetch).toBe(false);
  expect(container.querySelector("a")?.getAttribute("href")).toBe("/upload");
  await act(async () => state.ready?.());
  expect(state.prefetch).toBe(true);
  state.pending = true;
  await act(async () => root.render(<PostFab />));
  expect(container.querySelector("[aria-busy]")).not.toBeNull();
  expect(container.querySelector("svg.animate-spin")).toBeNull();
});
it("ログイン済みならプロフィール取得待ちでも投稿の入口とフォームを表示する", async () => {
  state.loading = true;
  await act(async () => root.render(<UploadPage />));
  expect(container.textContent).toContain("何を投稿しますか?");
  expect(container.querySelectorAll("a")).toHaveLength(5);
  await act(async () => root.render(<ComposerPage title="文章・写真"><textarea aria-label="詳細" /></ComposerPage>));
  expect(container.querySelector("textarea")).not.toBeNull();
  expect(state.replace).not.toHaveBeenCalled();
});
it("認証の確認中はログインへ飛ばさず、未ログイン確定後は作成画面を戻り先にする", async () => {
  state.session = null; state.loading = true;
  await act(async () => root.render(<ComposerPage title="文章・写真"><textarea /></ComposerPage>));
  expect(container.querySelector("textarea")).toBeNull();
  expect(state.replace).not.toHaveBeenCalled();
  state.loading = false;
  await act(async () => root.render(<ComposerPage title="文章・写真"><textarea /></ComposerPage>));
  expect(state.replace).toHaveBeenCalledWith("/login?next=%2Fposts%2Fnew");
  await act(async () => root.render(<UploadPage />));
  expect(state.replace).toHaveBeenLastCalledWith("/login?next=/upload");
});

it("下部バーのタブは0.2秒待った場合だけ薄い色にし、アイコンは回さない", async () => {
  vi.useFakeTimers();
  const view = () => <MainTabLink pendingDelay={200} href="/memo"><span>メモのアイコン</span></MainTabLink>;
  const busy = () => container.querySelector("[aria-busy]");
  try {
    state.pending = true;
    await act(async () => root.render(view()));
    expect(busy()).toBeNull();
    await act(async () => vi.advanceTimersByTime(199));
    expect(busy()).toBeNull();
    await act(async () => vi.advanceTimersByTime(1));
    expect(busy()).not.toBeNull();
    expect(container.querySelector("svg")).toBeNull();
    state.pending = false;
    await act(async () => root.render(view()));
    expect(busy()).toBeNull();
    state.pending = true;
    await act(async () => root.render(view()));
    await act(async () => vi.advanceTimersByTime(100));
    state.pending = false;
    await act(async () => root.render(view()));
    await act(async () => vi.advanceTimersByTime(500));
    expect(busy()).toBeNull();
    expect(container.textContent).toContain("メモのアイコン");
  } finally { vi.useRealTimers(); }
});

it("タブをタップすると画面に読み込み表示を出し、URL が変われば消す", async () => {
  const view = () => <><MainTabLink href="/memo">メモ</MainTabLink><ScreenLoader /></>;
  await act(async () => root.render(view()));
  const loader = () => container.querySelector(".screen-loader");
  expect(loader()?.className).toContain("hidden");
  expect(container.querySelector('a[href="/memo"]')?.hasAttribute("data-screen-loader")).toBe(true);
  await act(async () => markLinkPending("memo", "/memo"));
  expect(loader()?.className).toContain("flex");
  expect(container.querySelector('[role="status"]')?.textContent).toBe("画面を開いています");
  await act(async () => clearLinkPending());
  expect(loader()?.className).toContain("hidden");
  expect(container.querySelector('[role="status"]')).toBeNull();
});

it("レビューのタブでは投稿ボタンを出さない", async () => {
  await act(async () => root.render(<HomePostFab />));
  expect(container.querySelector('a[aria-label="投稿する"]')).not.toBeNull();
  state.search = "sort=recruitment";
  await act(async () => root.render(<HomePostFab key="recruitment" />));
  expect(container.querySelector('a[aria-label="投稿する"]')).not.toBeNull();
  state.search = "sort=review";
  await act(async () => root.render(<HomePostFab key="review" />));
  expect(container.querySelector('a[aria-label="投稿する"]')).toBeNull();
});

it("未ログインの投稿ボタンはログインへ直接案内する", async () => {
  state.session = null;
  await act(async () => root.render(<PostFab />));
  expect(container.querySelector("a")?.getAttribute("href")).toBe("/login?next=/upload");
});
