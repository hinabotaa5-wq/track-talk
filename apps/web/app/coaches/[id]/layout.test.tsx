import { afterEach, expect, it, vi } from "vitest";

// 特集モード（本体 #985）では、特集コーチ以外の詳細を直URLで開いても特集ページ（/coaching）へ戻す。
const redirect = vi.fn((to: string) => { throw new Error(`redirect:${to}`); });
const flags = { featured: [] as string[] };
vi.mock("next/navigation", () => ({ redirect: (to: string) => redirect(to) }));
vi.mock("@/lib/features", async (original) => ({ ...await original<typeof import("@/lib/features")>(), get featuredCoachIDs() { return flags.featured; } }));
const { default: Layout } = await import("./layout");

afterEach(() => { redirect.mockClear(); flags.featured = []; });
const open = (id: string) => Layout({ children: "detail", params: Promise.resolve({ id }) });

it("特集モードでは、特集コーチ以外の詳細を /coaching へ戻す", async () => {
  flags.featured = ["c-featured"];
  await expect(open("c-other")).rejects.toThrow("redirect:/coaching");
  await expect(open("c-featured")).resolves.toBeTruthy();
});

it("特集モードを切ると、どのコーチの詳細もそのまま開ける", async () => {
  await expect(open("c-other")).resolves.toBeTruthy();
  expect(redirect).not.toHaveBeenCalled();
});

it("特集コーチが複数なら、並びに含まれる詳細は開け、含まれない詳細は /coaching へ戻す（本体 #1063）", async () => {
  flags.featured = ["c-featured", "c-second"];
  await expect(open("c-featured")).resolves.toBeTruthy();
  await expect(open("c-second")).resolves.toBeTruthy();
  await expect(open("c-other")).rejects.toThrow("redirect:/coaching");
});
