/** @vitest-environment jsdom */
import { act, useState } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { ReviewNomination } from "./review-nomination";
vi.mock("@/components/coach-photo", () => ({ CoachAvatar: () => null }));
let root: Root, container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });
const coaches = [{ id:"coach-1",nickname:"テストコーチ",photo_url:"",best_record:"10秒台",achievements:"全国大会出場",nomination_fee_yen:800,total_price_yen:2000 }];
function Harness() { const [value,setValue]=useState(""); return <><ReviewNomination coaches={coaches} value={value} onChange={setValue} disabled={false} error="" loading={false} /><output>{value || "random"}</output></>; }
it("初期状態はおまかせで、同じ画面から指名を追加し解除できる", async () => {
  await act(async () => root.render(<Harness />));
  expect(container.querySelector("select")).toBeNull();
  expect(container.querySelector("output")?.textContent).toBe("random");
  await act(async () => container.querySelector<HTMLInputElement>('input[type="checkbox"]')!.click());
  const select=container.querySelector("select")!;
  expect(select.textContent).toContain("合計2,000円（指名料＋800円）");
  await act(async () => {select.value="coach-1";select.dispatchEvent(new Event("change",{bubbles:true}));});
  expect(container.querySelector("output")?.textContent).toBe("coach-1");
  expect(container.textContent).toContain("指名したコーチだけに依頼します");
  await act(async () => container.querySelector<HTMLInputElement>('input[type="checkbox"]')!.click());
  expect(container.querySelector("output")?.textContent).toBe("random");
});
it("送信中は指名を変更できない", async () => {
 await act(async () => root.render(<ReviewNomination coaches={coaches} value="" onChange={vi.fn()} disabled error="" loading={false}/>));
 expect(container.querySelector("fieldset")?.disabled).toBe(true);
});
