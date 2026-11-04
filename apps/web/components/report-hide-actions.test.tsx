/** @vitest-environment jsdom */
import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { ReportHideActions } from "./report-hide-actions";
import type { AdminReport } from "@/lib/types";

const fetch = vi.hoisted(() => vi.fn());
const confirmDialog = vi.hoisted(() => vi.fn());
vi.mock("@/lib/api", async (original) => ({ ...(await original<typeof import("@/lib/api")>()), apiFetch: fetch }));
vi.mock("@/components/confirm-dialog", async (original) => ({
  ...(await original<typeof import("@/components/confirm-dialog")>()),
  useConfirm: () => confirmDialog,
}));
vi.mock("next/link", () => ({
  default: ({ href, children }: { href: string; children: React.ReactNode }) => createElement("a", { href }, children),
}));

import { ApiRequestError } from "@/lib/api";

let root: Root, container: HTMLDivElement;

const report = (over: Partial<AdminReport>) =>
  ({ id: "r1", target_type: "post", target_id: "t1", post_id: "p1", target_hidden: false, injury_part: "", ...over }) as AdminReport;

function button() {
  return [...container.querySelectorAll("button")].find((b) => b.textContent === "非表示にする");
}

async function show(r: AdminReport) {
  await act(async () => root.render(createElement(ReportHideActions, { report: r, token: "token" })));
}

async function clickHide() {
  await act(async () => button()!.dispatchEvent(new MouseEvent("click", { bubbles: true })));
}

beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  fetch.mockReset();
  fetch.mockResolvedValue(undefined);
  confirmDialog.mockReset();
  confirmDialog.mockResolvedValue(true);
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
});

it("投稿の通報から投稿を非表示にし、開けなくなるリンクを消す", async () => {
  await show(report({ target_type: "post" }));
  expect(container.querySelector("a")?.getAttribute("href")).toBe("/posts/p1");
  await clickHide();
  expect(fetch).toHaveBeenCalledWith("/v1/admin/posts/t1/hide", { method: "POST", token: "token" });
  expect(container.textContent).toContain("非表示済み");
  expect(button()).toBeUndefined();
  expect(container.querySelector("a")).toBeNull();
});

it("コメントの通報からコメントを非表示にし、投稿へのリンクは残す", async () => {
  await show(report({ target_type: "comment" }));
  await clickHide();
  expect(fetch).toHaveBeenCalledWith("/v1/admin/comments/t1/hide", { method: "POST", token: "token" });
  expect(container.textContent).toContain("非表示済み");
  expect(container.querySelector("a")?.getAttribute("href")).toBe("/posts/p1");
});

it("確認でやめたら API を呼ばない", async () => {
  confirmDialog.mockResolvedValue(false);
  await show(report({ target_type: "post" }));
  await clickHide();
  expect(fetch).not.toHaveBeenCalled();
  expect(button()).toBeDefined();
});

it("すでに非表示の対象にはボタンを出さない", async () => {
  await show(report({ target_type: "post", target_hidden: true }));
  expect(button()).toBeUndefined();
  expect(container.textContent).toContain("非表示済み");
});

it("対象が消えていたら、その旨を出す", async () => {
  fetch.mockRejectedValue(new ApiRequestError(404, { error: { code: "not_found", message: "見つかりませんでした" } }));
  await show(report({ target_type: "post" }));
  await clickHide();
  expect(container.textContent).toContain("すでに非表示か、削除されています");
  expect(button()).toBeDefined();
});

it("二段階認証が済んでいないときは API の理由をそのまま出す", async () => {
  fetch.mockRejectedValue(new ApiRequestError(403, { error: { code: "mfa_required", message: "この操作には二段階認証が必要です" } }));
  await show(report({ target_type: "comment" }));
  await clickHide();
  expect(container.textContent).toContain("この操作には二段階認証が必要です");
});
