/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

const apiFetch = vi.fn();
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => apiFetch(...args) }));
const { OverseasInterestAdmin } = await import("./overseas-interest-admin");

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
  apiFetch.mockReset();
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });

it("総数・種別内訳・登録者の一覧を出す", async () => {
  apiFetch.mockResolvedValue({
    total: 2,
    breakdown: { monthly: 1, single: 0, both: 1 },
    interests: [
      { kind: "monthly", nickname: "希望者", created_at: "2026-11-11 12:00" },
      { kind: "both", nickname: "", created_at: "2026-11-11 13:00" },
    ],
  });
  await act(async () => root.render(<OverseasInterestAdmin token="tok" />));
  await act(async () => { await Promise.resolve(); });
  expect(apiFetch).toHaveBeenCalledWith("/v1/admin/overseas-interests", { token: "tok" });
  const text = container.textContent!;
  expect(text).toContain("登録の総数：2件");
  expect(text).toContain("1ヶ月コーチング 1件");
  expect(text).toContain("単発の動画添削 0件");
  expect(text).toContain("両方 1件");
  expect(text).toContain("希望者");
  expect(text).toContain("名前なし");
});
