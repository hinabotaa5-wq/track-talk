import { afterEach, expect, it, vi } from "vitest";

// 特集モード（本体 #985）では、特集コーチ以外の詳細を直URLで開いても特集ページ（/coaching）へ戻す。
const redirect = vi.fn((to: string) => { throw new Error(`redirect:${to}`); });
const flags = { featured: null as string | null };
vi.mock("next/navigation", () => ({ redirect: (to: string) => redirect(to) }));
vi.mock("@/lib/features", async (original) => ({ ...await original<typeof import("@/lib/features")>(), get featuredCoachID() { return flags.featured; } }));
const { default: Layout } = await import("./layout");

afterEach(() => { redirect.mockClear(); flags.featured = null; });
const open = (id: string) => Layout({ children: "detail", params: Promise.resolve({ id }) });

it("特集モードでは、特集コーチ以外の詳細を /coaching へ戻す", async () => {
  flags.featured = "c-featured";
  await expect(open("c-other")).rejects.toThrow("redirect:/coaching");
  await expect(open("c-featured")).resolves.toBeTruthy();
});

it("特集モードを切ると、どのコーチの詳細もそのまま開ける", async () => {
  await expect(open("c-other")).resolves.toBeTruthy();
  expect(redirect).not.toHaveBeenCalled();
});
