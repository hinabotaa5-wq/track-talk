import { expect, it, vi } from "vitest";
import CoachOffersPage from "./page";

const { redirect } = vi.hoisted(() => ({ redirect: vi.fn() }));
vi.mock("next/navigation", () => ({ redirect }));

it("旧依頼一覧を対応中の画面へ転送する", () => {
  CoachOffersPage();
  expect(redirect).toHaveBeenCalledWith("/coach/reviewing");
});
