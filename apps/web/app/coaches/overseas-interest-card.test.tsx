/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

const apiFetch = vi.fn();
const flags = { enabled: true };
vi.mock("next/link", () => ({ default: ({ href, children }: { href: string; children: ReactNode }) => <a href={href}>{children}</a> }));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => apiFetch(...args) }));
vi.mock("@/lib/features", async (original) => ({ ...await original<typeof import("@/lib/features")>(), get overseasInterestEnabled() { return flags.enabled; } }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => ({ token: "tok", loading: false }) }));
const { OverseasInterestCard } = await import("./overseas-interest-card");
const { ApiRequestError } = await import("@/lib/api");

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
  apiFetch.mockReset(); flags.enabled = true;
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });
const render = async () => { await act(async () => root.render(<OverseasInterestCard />)); await act(async () => { await Promise.resolve(); }); };
const click = (label: string) => act(async () => { [...container.querySelectorAll("button")].find((b) => b.textContent === label)!.click(); await Promise.resolve(); });
const choose = (label: string) => act(async () => { [...container.querySelectorAll("label")].find((l) => l.textContent === label)!.querySelector("input")!.click(); await Promise.resolve(); });

it("フラグが閉じている間はカードを出さず、読みにも行かない", async () => {
  flags.enabled = false;
  await render();
  expect(container.innerHTML).toBe("");
  expect(apiFetch).not.toHaveBeenCalled();
});

it("未ログインは登録できず、ログイン導線だけを出す", async () => {
  apiFetch.mockRejectedValue(new ApiRequestError(401, { error: { code: "unauthorized", message: "ログインが必要です" } }));
  await render();
  expect(container.textContent).toContain("海外コーチ、近日開始");
  expect(container.querySelector("a")?.getAttribute("href")).toBe("/login?next=/coaches");
  expect(container.querySelectorAll("button").length).toBe(0);
});

it("種別を選んで登録すると登録済みの2行だけになり、変更・取消の導線が出ない", async () => {
  apiFetch.mockResolvedValueOnce({ interest: null });
  await render();
  expect(apiFetch).toHaveBeenCalledWith("/v1/me/overseas-interest", { token: "tok" });
  expect(container.querySelector("button")?.textContent).toBe("指導を受けたい");
  expect(container.querySelector("button")?.disabled).toBe(true);
  expect(container.textContent).toContain("海外のコーチから指導を受けられる機能を準備中です");
  expect(container.textContent).not.toContain("海外在住のスプリントコーチ");

  await choose("両方");
  apiFetch.mockResolvedValueOnce({ interest: { kind: "both" } });
  await click("指導を受けたい");
  expect(apiFetch).toHaveBeenLastCalledWith("/v1/me/overseas-interest", { method: "PUT", token: "tok", body: JSON.stringify({ kind: "both" }) });
  expect(container.textContent).toContain("登録済みです。開始時にお知らせします");
  expect(container.textContent).toContain("いまの選択：両方");
  expect(container.querySelectorAll("input[type=radio]").length).toBe(0);
  expect(container.querySelectorAll("button").length).toBe(0);
  expect(container.textContent).not.toContain("変更する");
  expect(container.textContent).not.toContain("登録を取り消す");
  expect(container.textContent).not.toContain("受けたい形を選んでください");
});

it("登録済みで開くと最初から2行だけで、読み直し以外の通信をしない", async () => {
  apiFetch.mockResolvedValueOnce({ interest: { kind: "single" } });
  await render();
  expect(apiFetch).toHaveBeenCalledTimes(1);
  expect(container.textContent).toContain("登録済みです。開始時にお知らせします");
  expect(container.textContent).toContain("いまの選択：単発の動画添削");
  expect(container.querySelectorAll("input[type=radio]").length).toBe(0);
  expect(container.querySelectorAll("button").length).toBe(0);
});

it("登録数は利用者側に一切出さない", async () => {
  apiFetch.mockResolvedValueOnce({ interest: { kind: "single" } });
  await render();
  expect(container.textContent).toContain("登録済みです。開始時にお知らせします");
  expect(container.textContent).not.toMatch(/件|名|人|残り/);
});
