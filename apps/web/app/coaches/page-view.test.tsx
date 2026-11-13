/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { installEnglish, setActiveLocale } from "@/lib/i18n";
import { en } from "@/lib/i18n-en";

// 海外コーチの指導希望のアンケート（本体 #542）は #615 でコードごと撤去した。
// コーチ一覧を開けた状態でも、ja／en・ログインの有無にかかわらずカードと登録導線が出ず、旧 API も読みに行かない。
const apiFetch = vi.fn();
const auth = { token: "tok" as string | null };
const params = { tab: null as string | null };
vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: vi.fn() }),
  useSearchParams: () => ({ get: () => params.tab }),
  usePathname: () => "/coaches",
}));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => apiFetch(...args) }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => ({ token: auth.token, loading: false }) }));
vi.mock("./coach-list", () => ({ CoachList: () => <div>coach-list</div> }));
vi.mock("./support-plan-list", () => ({ SupportPlanList: () => <div>support-plan-list</div> }));
const { CoachesView } = await import("./page-view");

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
  apiFetch.mockReset(); auth.token = "tok"; params.tab = null;
  apiFetch.mockResolvedValue({ application: null });
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); setActiveLocale("ja"); });
const render = async () => { await act(async () => root.render(<CoachesView />)); await act(async () => { await Promise.resolve(); }); };

for (const locale of ["ja", "en"] as const) {
  for (const token of ["tok", null]) {
    for (const tab of [null, "support"]) {
      it(`${locale}・${token ? "ログイン済み" : "未ログイン"}・${tab ?? "video"} タブで海外コーチのアンケートを出さない`, async () => {
        if (locale === "en") installEnglish(en);
        setActiveLocale(locale);
        auth.token = token;
        params.tab = tab;
        await render();
        const text = container.textContent ?? "";
        // 見出しと説明文は出さない（本体 #641）。言語が混ざらない。
        expect(text).not.toContain("コーチ一覧");
        expect(text).not.toContain("Coaches");
        expect(text).not.toContain("上のタブで、動画添削の指名とサポートプランを切り替えられます。");
        expect(text).not.toContain("Use the tabs above");
        expect(container.querySelector("h1")).toBeNull();
        expect(text).toContain(tab === "support" ? "support-plan-list" : "coach-list");
        for (const word of ["海外コーチ", "指導を受けたい", "Coaches overseas", "overseas coach", "coming soon"]) {
          expect(text).not.toContain(word);
        }
        expect(container.querySelector('input[name="overseas-interest-kind"]')).toBeNull();
        // 「コーチ」ボタンの応募確認以外の API は読みに行かない。
        expect(apiFetch.mock.calls.filter(([path]) => path !== "/v1/coach/me")).toEqual([]);
      });
    }
  }
}
