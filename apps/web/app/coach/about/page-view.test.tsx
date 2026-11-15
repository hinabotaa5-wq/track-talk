/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { CoachAboutView } from "./page-view";

vi.mock("next/link", () => ({ default: ({ href, children, replace, ...props }: { href: string; children: ReactNode; replace?: boolean }) => <a href={href} data-replace={replace} {...props}>{children}</a> }));

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });

it("継続プランの説明が無料相談から提案で成立する流れで表示される", async () => {
  await act(async () => root.render(<CoachAboutView />));
  expect(container.textContent).toContain("選手からの無料相談に回答し");
  expect(container.textContent).toContain("提案に同意して支払うと");
  expect(container.textContent).toContain("引き受け操作なしに契約が成立");
  expect(container.textContent).toContain("無料相談から始め、提案への同意・支払いで成立");
  expect(container.textContent).toContain("受付設定と無料相談への回答がコーチの操作");
});

it("継続プランに旧手順の文が残っていない", async () => {
  await act(async () => root.render(<CoachAboutView />));
  expect(container.textContent).not.toContain("プランを公開し");
  expect(container.textContent).not.toContain("承諾した後");
  expect(container.textContent).not.toContain("出品");
  expect(container.textContent).not.toContain("目標の大会に向けて、準備や調整をサポート");
  expect(container.textContent).not.toContain("プランに明記し");
  expect(container.textContent).not.toContain("対応できる内容と定員を自分で決めます");
});

it("1回プランの案内は変わっていない", async () => {
  await act(async () => root.render(<CoachAboutView />));
  expect(container.textContent).toContain("受付をオンにすると、申込みが対応中の一覧に届きます");
  expect(container.textContent).toContain("届いてから24時間以内に良い点・直す点・おすすめのドリルを返してください");
});
