/** @vitest-environment jsdom */
import { act, useState } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { ReviewCoachPicker } from "./review-coach-picker";
vi.mock("@/components/coach-photo", () => ({ CoachAvatar: () => null }));
let root: Root, container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });
const coaches = [
  { id:"coach-b",nickname:"Bコーチ",photo_url:"",events:["100m" as const],years:12,best_record:"10秒台",achievements:"スタートが専門です" },
  { id:"coach-a",nickname:"Aコーチ",photo_url:"",events:["100m" as const],years:0,best_record:"",achievements:"" },
];
function Harness() { const [value,setValue]=useState(""); return <><ReviewCoachPicker coaches={coaches} value={value} onChange={setValue} disabled={false} error="" loading={false} /><output>{value || "none"}</output></>; }
it("おまかせはなく、受け取った順のままコーチを選べる", async () => {
  await act(async () => root.render(<Harness />));
  expect(container.textContent).not.toContain("おまかせ");
  expect(container.querySelector("output")?.textContent).toBe("none");
  const radios = container.querySelectorAll<HTMLInputElement>('input[type="radio"]');
  expect([...radios].map((r) => r.value)).toEqual(["coach-b", "coach-a"]);
  await act(async () => radios[1].click());
  expect(container.querySelector("output")?.textContent).toBe("coach-a");
  expect(container.textContent).toContain("別のコーチには回さず");
});
it("料金や人気の指標は出さず、種目・競技歴・紹介文を出す", async () => {
  await act(async () => root.render(<Harness />));
  const text = container.textContent ?? "";
  expect(text).toContain("100m · 競技歴12年");
  expect(text).toContain("スタートが専門です");
  for (const word of ["円", "件", "★", "人気", "ランキング", "指名料"]) expect(text).not.toContain(word);
});
it("送信中は選び直せない", async () => {
  await act(async () => root.render(<ReviewCoachPicker coaches={coaches} value="" onChange={vi.fn()} disabled error="" loading={false}/>));
  expect(container.querySelector("fieldset")?.disabled).toBe(true);
});
