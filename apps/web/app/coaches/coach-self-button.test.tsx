/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

// 円ボタンの件数バッジは、表示中の利用者の分だけを出す（本体 #1009）。
const auth = vi.hoisted(() => ({ token: "tok-a" as string | null, session: { user: { id: "user-a" } } as object | null }));
const apiFetch = vi.hoisted(() => vi.fn());
vi.mock("@/components/auth-provider", () => ({ useAuth: () => ({ token: auth.token, session: auth.session }) }));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => apiFetch(...args) }));
vi.mock("next/link", () => ({
  default: ({ href, children, ...props }: { href: string; children: ReactNode }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));
const { CoachSelfButton } = await import("./coach-self-button");

type Counts = { player_count: number; coach_count: number };
function deferred() {
  let resolve!: (value: Counts) => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<Counts>((res, rej) => { resolve = res; reject = rej; });
  return { promise, resolve, reject };
}

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
  auth.token = "tok-a";
  auth.session = { user: { id: "user-a" } };
  apiFetch.mockReset();
  apiFetch.mockResolvedValue({ player_count: 0, coach_count: 0 });
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
});

const render = async () => {
  await act(async () => root.render(<CoachSelfButton />));
  await act(async () => { await Promise.resolve(); });
};
const badges = () => [...container.querySelectorAll("a")].map((a) => a.querySelector("span[aria-hidden]")?.textContent ?? null);
const labels = () => [...container.querySelectorAll("a")].map((a) => a.getAttribute("aria-label"));
const switchTo = async (token: string | null, userId: string | null) => {
  auth.token = token;
  auth.session = userId ? { user: { id: userId } } : null;
  await act(async () => root.render(<CoachSelfButton />));
};
const pageshow = async (persisted: boolean) => {
  const event = new Event("pageshow");
  Object.defineProperty(event, "persisted", { value: persisted });
  await act(async () => { window.dispatchEvent(event); });
  await act(async () => { await Promise.resolve(); });
};

it("T1：利用者 A の件数のあと B に切り替えると、B の取得が終わるまでバッジを出さず、終わったら B の件数だけを出す", async () => {
  apiFetch.mockResolvedValueOnce({ player_count: 2, coach_count: 3 });
  await render();
  expect(badges()).toEqual(["2", "3"]);

  const b = deferred();
  apiFetch.mockReturnValueOnce(b.promise);
  await switchTo("tok-b", "user-b");
  expect(badges()).toEqual([null, null]);
  expect(labels()).toEqual(["選手", "コーチ"]);
  expect(apiFetch).toHaveBeenLastCalledWith("/v1/me/coaching/action-counts", { token: "tok-b" });

  await act(async () => { b.resolve({ player_count: 0, coach_count: 5 }); });
  expect(badges()).toEqual([null, "5"]);
});

it("T1：切替前に出した A の取得が切替後に返っても、B の画面には出さない", async () => {
  const a = deferred();
  apiFetch.mockReturnValueOnce(a.promise);
  await render();
  const b = deferred();
  apiFetch.mockReturnValueOnce(b.promise);
  await switchTo("tok-b", "user-b");
  await act(async () => { a.resolve({ player_count: 7, coach_count: 7 }); });
  expect(badges()).toEqual([null, null]);
  await act(async () => { b.resolve({ player_count: 1, coach_count: 0 }); });
  expect(badges()).toEqual(["1", null]);
});

it("T2：pageshow（persisted）で現在の利用者の件数を取り直し、返った件数に更新する", async () => {
  apiFetch.mockResolvedValueOnce({ player_count: 1, coach_count: 0 });
  await render();
  expect(badges()).toEqual(["1", null]);
  apiFetch.mockClear();
  apiFetch.mockResolvedValueOnce({ player_count: 4, coach_count: 6 });
  await pageshow(true);
  expect(apiFetch).toHaveBeenCalledTimes(1);
  expect(apiFetch).toHaveBeenCalledWith("/v1/me/coaching/action-counts", { token: "tok-a" });
  expect(badges()).toEqual(["4", "6"]);
});

it("T2：persisted でない pageshow では取り直さない", async () => {
  await render();
  apiFetch.mockClear();
  await pageshow(false);
  expect(apiFetch).not.toHaveBeenCalled();
});

it("T12：同じ利用者のまま token 文字列だけが変わったら、バッジを消さずに取り直し、完了後に新しい値へ更新する", async () => {
  apiFetch.mockResolvedValueOnce({ player_count: 2, coach_count: 3 });
  await render();
  const next = deferred();
  apiFetch.mockReturnValueOnce(next.promise);
  await switchTo("tok-a2", "user-a");
  expect(badges()).toEqual(["2", "3"]);
  expect(apiFetch).toHaveBeenLastCalledWith("/v1/me/coaching/action-counts", { token: "tok-a2" });
  await act(async () => { next.resolve({ player_count: 1, coach_count: 0 }); });
  expect(badges()).toEqual(["1", null]);
});

it("T7：ログアウト（token なし）では前の件数を出さず、取得もしない", async () => {
  apiFetch.mockResolvedValueOnce({ player_count: 2, coach_count: 3 });
  await render();
  apiFetch.mockClear();
  await switchTo(null, null);
  expect(badges()).toEqual([null, null]);
  expect(apiFetch).not.toHaveBeenCalled();
});

it("T7：取得に失敗したらバッジを出さない", async () => {
  apiFetch.mockRejectedValueOnce(new Error("fail"));
  await render();
  expect(badges()).toEqual([null, null]);
  expect(labels()).toEqual(["選手", "コーチ"]);
});

it("件数の検証規則と aria-label は従来どおり（正の有限数だけを床関数、99超は 99+）", async () => {
  apiFetch.mockResolvedValueOnce({ player_count: 2.7, coach_count: 150 });
  await render();
  expect(badges()).toEqual(["2", "99+"]);
  expect(labels()).toEqual(["選手（未対応2件）", "コーチ（未対応150件）"]);
  apiFetch.mockResolvedValueOnce({ player_count: -1, coach_count: Number.NaN });
  await pageshow(true);
  expect(badges()).toEqual([null, null]);
});
