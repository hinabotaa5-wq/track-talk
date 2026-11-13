/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { installEnglish, setActiveLocale } from "@/lib/i18n";
import { en } from "@/lib/i18n-en";

// 海外コーチの指導希望のアンケート（本体 #542）は #615 でコードごと撤去した。
// コーチ一覧を開けた状態でも、ja／en・ログインの有無にかかわらずカードと登録導線が出ず、旧 API も読みに行かない。
// 一覧の見出しと説明文は #641 で撤去した。切り替えは共通部品 ViewSwitch が担い、?tab=support の直接表示を保つ。
const apiFetch = vi.fn();
const routerReplace = vi.fn();
const auth = { token: "tok" as string | null };
const nav = { tab: null as string | null, pathname: "/coaches" };
vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: routerReplace }),
  usePathname: () => nav.pathname,
  useSearchParams: () => ({ get: (key: string) => (key === "tab" ? nav.tab : null) }),
}));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => apiFetch(...args) }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => ({ token: auth.token, loading: false }) }));
vi.mock("./coach-list", () => ({ CoachList: () => <div>coach-list</div> }));
vi.mock("./support-plan-list", () => ({ SupportPlanList: () => <div>support-plan-list</div> }));
vi.mock("./coach-button", () => ({ CoachButton: () => <div>coach-button</div> }));
const { CoachesView } = await import("./page-view");

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
  apiFetch.mockReset(); routerReplace.mockReset();
  auth.token = "tok"; nav.tab = null; nav.pathname = "/coaches";
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); setActiveLocale("ja"); });
const render = async () => { await act(async () => root.render(<CoachesView />)); await act(async () => { await Promise.resolve(); }); };
const switchTo = async (label: string) => {
  const el = [...container.querySelectorAll("a")].find((a) => a.textContent === label);
  await act(async () => { (el as HTMLAnchorElement).click(); });
};

for (const locale of ["ja", "en"] as const) {
  for (const token of ["tok", null]) {
    for (const tab of [null, "support"]) {
      it(`${locale}・${token ? "ログイン済み" : "未ログイン"}・${tab ?? "video"} タブで海外コーチのアンケートを出さない`, async () => {
        if (locale === "en") installEnglish(en);
        setActiveLocale(locale);
        auth.token = token;
        nav.tab = tab;
        await render();
        const text = container.textContent ?? "";
        for (const word of ["海外コーチ", "指導を受けたい", "Coaches overseas", "overseas coach", "coming soon"]) {
          expect(text).not.toContain(word);
        }
        expect(container.querySelector('input[name="overseas-interest-kind"]')).toBeNull();
        expect(apiFetch).not.toHaveBeenCalled();
      });
    }
  }
}

it("見出し「コーチ一覧」と説明文を出さない（#641）", async () => {
  await render();
  expect(container.querySelector("h1")).toBeNull();
  expect(container.textContent).not.toContain("コーチ一覧");
  expect(container.textContent).not.toContain("上のタブで、動画添削の指名とサポートプランを切り替えられます。");
});

it("ViewSwitch で切り替え、?tab=support の直接表示ができる", async () => {
  nav.tab = "support";
  await render();
  expect(container.textContent).toContain("support-plan-list");
  expect(container.textContent).not.toContain("coach-list");
});

it("切り替えは今のルートを保って tab だけを変える", async () => {
  await render();
  expect(container.textContent).toContain("coach-list");
  await switchTo("サポートプラン");
  expect(routerReplace).toHaveBeenCalledWith("/coaches?tab=support");
  await switchTo("動画添削");
  expect(routerReplace).toHaveBeenCalledWith("/coaches");
});

it("/coaching 上では /coaching を保って切り替える", async () => {
  nav.pathname = "/coaching";
  await render();
  await switchTo("サポートプラン");
  expect(routerReplace).toHaveBeenCalledWith("/coaching?tab=support");
});

it("サポート側の URL を共有できるリンクとして出す", async () => {
  await render();
  const support = [...container.querySelectorAll("a")].find((a) => a.textContent === "サポートプラン");
  expect(support?.getAttribute("href")).toBe("/coaches?tab=support");
});
