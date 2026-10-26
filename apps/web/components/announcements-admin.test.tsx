/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { AnnouncementsAdmin } from "./announcements-admin";

const fetch = vi.hoisted(() => vi.fn());
vi.mock("@/lib/api", () => ({ apiFetch: fetch, apiErrorText: () => "失敗" }));
vi.mock("@/components/confirm-dialog", () => ({ useConfirm: () => vi.fn() }));

let root: Root, container: HTMLDivElement;

function type(el: HTMLInputElement | HTMLTextAreaElement, value: string) {
  const proto = el instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
  Object.getOwnPropertyDescriptor(proto, "value")!.set!.call(el, value);
  el.dispatchEvent(new Event("input", { bubbles: true }));
}

function button(text: string) {
  return [...container.querySelectorAll("button")].find((b) => b.textContent === text)!;
}

beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  fetch.mockReset();
  fetch.mockImplementation(async (path: string, init?: RequestInit) => {
    if (path === "/v1/admin/announcements/recipients") return { in_app: 120, push: 45, push_configured: true };
    if (path === "/v1/admin/announcements" && init?.method === "POST") return { in_app: 120, push_recipients: 44 };
    return { announcements: [] };
  });
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
});

it("人数を見て「送信」と入力するまで、全員には送れない", async () => {
  await act(async () => root.render(<AnnouncementsAdmin token="t" />));
  expect(button("全員に送る...").disabled).toBe(true);
  await act(async () => {
    type(container.querySelector("input")!, "コーチ機能が始まりました");
    type(container.querySelector("textarea")!, "パーソナル指導を受けられます");
  });
  await act(async () => button("全員に送る...").click());
  expect(container.textContent).toContain("ベル: 120人");
  expect(container.textContent).toContain("プッシュ: 45人");
  const send = button("全員に送信");
  expect(send.disabled).toBe(true);
  const confirmInput = container.querySelector<HTMLInputElement>('[role="alertdialog"] input')!;
  await act(async () => type(confirmInput, "そうしん"));
  expect(send.disabled).toBe(true);
  await act(async () => type(confirmInput, "送信"));
  expect(send.disabled).toBe(false);
  await act(async () => send.click());
  const post = fetch.mock.calls.find(([path, init]) => path === "/v1/admin/announcements" && init?.method === "POST")!;
  const payload = JSON.parse(post[1].body as string);
  expect(payload).toMatchObject({ title: "コーチ機能が始まりました", body: "パーソナル指導を受けられます", url: "/", confirm: "送信" });
  expect(payload.id).toMatch(/^[0-9a-f-]{36}$/);
  expect(container.textContent).toContain("ベル 120人、プッシュ 44人に送りました");
});

it("下部バーと同じ5つのアイコンから通知の移動先を選べる", async () => {
  await act(async () => root.render(<AnnouncementsAdmin token="t" />));
  const radios = container.querySelectorAll<HTMLInputElement>('input[type="radio"]');
  expect([...radios].map((input) => input.value)).toEqual(["/", "/competitions", "/coaching", "/memo", "/injury"]);
  expect(container.textContent).not.toContain("サイト内のパス");
  await act(async () => {
    type(container.querySelector("input")!, "お知らせ");
    type(container.querySelector("textarea")!, "詳細です");
    radios[2].click();
  });
  await act(async () => button("自分にだけ試し送信").click());
  const request = fetch.mock.calls.find(([path]) => path === "/v1/admin/announcements/test")!;
  expect(JSON.parse(request[1].body as string).url).toBe("/coaching");
});
