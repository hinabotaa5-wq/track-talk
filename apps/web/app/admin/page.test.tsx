/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { ConfirmProvider } from "@/components/confirm-dialog";
import Page from "./page";

const mocks = vi.hoisted(() => ({ api: vi.fn(), replace: vi.fn() }));
vi.mock("next/link", () => ({ default: ({ href, children }: { href: string; children: ReactNode }) => <a href={href}>{children}</a> }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: mocks.replace }) }));
vi.mock("@/components/auth-provider", () => ({
  useAuth: () => ({
    session: { access_token: "admin-token" },
    profile: { is_admin: true },
    token: "admin-token",
    loading: false,
  }),
}));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => mocks.api(...args) }));

function emptyList(key: string) {
  return { [key]: [], meta: { page: 1, per_page: 20, total: 0 } };
}

function calledPaths() {
  return mocks.api.mock.calls.map((call) => String(call[0]));
}

let container: HTMLDivElement;
let root: Root;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  mocks.api.mockReset();
  mocks.replace.mockReset();
  mocks.api.mockImplementation((path: string) =>
    Promise.resolve(path.startsWith("/v1/admin/reports") ? emptyList("reports") : emptyList("inquiries")),
  );
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
});

it("初期表示では開いている通報タブだけを読む", async () => {
  await act(async () => {
    root.render(<Page />);
  });
  await act(async () => {});
  expect(calledPaths()).toContainEqual(expect.stringContaining("/v1/admin/reports"));
  expect(calledPaths().filter((p) => p.includes("/v1/admin/inquiries"))).toEqual([]);
});

it("コーチタブでは評価の平均・件数・停止表示と「順番に戻す」が出ない", async () => {
  const approved = {
    profile_id: "coach-1", status: "approved", nickname: "テストコーチ",
    service_types: ["video_review"], tier: "coach", events: ["100m"],
    best_record: "", achievements: "", years: 3, sns_urls: [], hitokoto: "",
    applied_at: "2026-11-01T00:00:00Z", reviewed_at: null, on_duty: true, daily_cap: 5,
    photo_url: "", pledged: true, payouts_ready: true,
    stripe_linked: true, payouts_enabled: true,
    rating_count: 8, rating_average: 2.1, rating_paused: true, country_code: "JP",
  };
  mocks.api.mockImplementation((path: string) =>
    Promise.resolve(
      path.startsWith("/v1/admin/coach-applications")
        ? { applications: path.includes("status=approved") ? [approved] : [], meta: { page: 1, per_page: 50, total: 1 } }
        : path.startsWith("/v1/admin/reports") ? emptyList("reports") : emptyList("inquiries"),
    ),
  );
  await act(async () => {
    root.render(<ConfirmProvider><Page /></ConfirmProvider>);
  });
  await act(async () => {});
  const coachTab = Array.from(container.querySelectorAll('[role="tab"]')).find((b) =>
    b.textContent === "コーチ",
  )!;
  await act(async () => {
    coachTab.dispatchEvent(new MouseEvent("click", { bubbles: true }));
  });
  await act(async () => {});
  const approvedFilter = Array.from(container.querySelectorAll("button")).find((b) =>
    b.textContent?.includes("承認済み"),
  )!;
  await act(async () => {
    approvedFilter.dispatchEvent(new MouseEvent("click", { bubbles: true }));
  });
  await act(async () => {});
  expect(container.textContent).toContain("テストコーチ");
  expect(container.textContent).not.toContain("★8");
  expect(container.textContent).not.toContain("評価:");
  expect(container.textContent).not.toContain("順番に戻す");
  expect(container.textContent).not.toContain("依頼が回っていません");
});

it("隠れていたタブは開いたときに読む", async () => {
  await act(async () => {
    root.render(<Page />);
  });
  await act(async () => {});
  const before = calledPaths().filter((p) => p.includes("source=feedback")).length;
  expect(before).toBe(0);
  const feedbackTab = Array.from(container.querySelectorAll('[role="tab"]')).find((b) =>
    b.textContent?.includes("なんでも書いてね"),
  )!;
  await act(async () => {
    feedbackTab.dispatchEvent(new MouseEvent("click", { bubbles: true }));
  });
  await act(async () => {});
  expect(calledPaths().filter((p) => p.includes("source=feedback"))).not.toEqual([]);
});

// 本体 #932 B5: コーチタブの各行に今の国（コードと国旗）を出し、運営が国を設定できる。
it("コーチタブで国を表示し、「国を設定」で API を呼んで行の表示を更新する", async () => {
  const coach = (id: string, name: string) => ({
    profile_id: id, status: "approved", nickname: name,
    service_types: ["video_review"], tier: "coach", events: ["100m"],
    best_record: "", achievements: "", years: 3, sns_urls: [], hitokoto: "",
    applied_at: "2026-11-01T00:00:00Z", reviewed_at: null, on_duty: true, daily_cap: 5,
    photo_url: "", pledged: true, payouts_ready: true,
    stripe_linked: true, payouts_enabled: true,
    rating_count: 0, rating_average: 0, rating_paused: false, country_code: "JP",
  });
  mocks.api.mockImplementation((path: string, init?: { method?: string; body?: string }) => {
    if (path === "/v1/admin/coaching/coaches/coach-1/country" && init?.method === "POST") {
      const code = (JSON.parse(init.body ?? "{}") as { country_code: string }).country_code.trim().toUpperCase();
      return Promise.resolve({ application: { ...coach("coach-1", "国を変えるコーチ"), country_code: code } });
    }
    return Promise.resolve(
      path.startsWith("/v1/admin/coach-applications")
        ? { applications: path.includes("status=approved") ? [coach("coach-1", "国を変えるコーチ"), coach("coach-2", "そのままのコーチ")] : [], meta: { page: 1, per_page: 50, total: 2 } }
        : path.startsWith("/v1/admin/reports") ? emptyList("reports") : emptyList("inquiries"),
    );
  });
  await act(async () => {
    root.render(<ConfirmProvider><Page /></ConfirmProvider>);
  });
  await act(async () => {});
  // 他のタブ（初期の通報タブ）には国の欄を出さない。
  expect(container.querySelector('[aria-label="国コード"]')).toBeNull();
  const coachTab = Array.from(container.querySelectorAll('[role="tab"]')).find((b) => b.textContent === "コーチ")!;
  await act(async () => {
    coachTab.dispatchEvent(new MouseEvent("click", { bubbles: true }));
  });
  await act(async () => {});
  const approvedFilter = Array.from(container.querySelectorAll("button")).find((b) => b.textContent?.includes("承認済み"))!;
  await act(async () => {
    approvedFilter.dispatchEvent(new MouseEvent("click", { bubbles: true }));
  });
  await act(async () => {});

  const rows = () => Array.from(container.querySelectorAll("li")).filter((li) => li.querySelector('[data-testid="coach-country"]'));
  expect(rows().map((li) => li.querySelector('[data-testid="coach-country"]')?.textContent)).toEqual(["国: JP 🇯🇵", "国: JP 🇯🇵"]);
  const input = rows()[0].querySelector<HTMLInputElement>('[aria-label="国コード"]')!;
  expect(input.value).toBe("JP");

  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!.call(input, "US");
    input.dispatchEvent(new Event("input", { bubbles: true }));
  });
  const submit = Array.from(rows()[0].querySelectorAll("button")).find((b) => b.textContent === "国を設定")!;
  await act(async () => {
    submit.dispatchEvent(new MouseEvent("click", { bubbles: true }));
  });
  await act(async () => {});

  const call = mocks.api.mock.calls.find((c) => c[0] === "/v1/admin/coaching/coaches/coach-1/country");
  expect(call?.[1]).toMatchObject({ method: "POST", token: "admin-token" });
  expect(JSON.parse(String((call?.[1] as { body: string }).body))).toEqual({ country_code: "US" });
  expect(rows().map((li) => li.querySelector('[data-testid="coach-country"]')?.textContent)).toEqual(["国: US 🇺🇸", "国: JP 🇯🇵"]);
  expect(container.textContent).toContain("国を設定しました");
});

// 本体 #1067 W6: 試用期間の返答は 良い点→改善点→キュー→おすすめのドリル→成功の目安→次に撮る動画→コーチなら明日から何をするか→補足 の順で出る。
it("W6: 試用期間の返答に改善点・おすすめのドリル・コーチなら明日から何をするかが順に出る", async () => {
  const response = (id: string, extra: Record<string, unknown>) => ({
    request_id: id, coach_id: "coach-1", coach_nickname: "審査コーチ", nth: 1, tier: "coach", event: "100m",
    good_point: "腕振りが良い", fix_points: ["接地を直す"], drill: "もも上げ", clarification: null,
    created_at: "2026-11-27T00:00:00Z", hidden_at: null, hitokoto: null, cue: null, success_criteria: null, next_video: null, ...extra,
  });
  mocks.api.mockImplementation((path: string) =>
    Promise.resolve(
      path.startsWith("/v1/admin/coaching/probation")
        ? {
            responses: [
              response("r-new", { hitokoto: "明日から壁押しを毎日やる", clarification: "補足です" }),
              response("r-v2", { hitokoto: "結論", cue: "地面を押す", success_criteria: "膝がそろう", next_video: "真横から" }),
              response("r-none", {}),
            ],
          }
        : path.startsWith("/v1/admin/coach-applications")
          ? { applications: [], meta: { page: 1, per_page: 50, total: 0 } }
          : path.startsWith("/v1/admin/reports") ? emptyList("reports") : emptyList("inquiries"),
    ),
  );
  await act(async () => {
    root.render(<ConfirmProvider><Page /></ConfirmProvider>);
  });
  await act(async () => {});
  const coachTab = Array.from(container.querySelectorAll('[role="tab"]')).find((b) => b.textContent === "コーチ")!;
  await act(async () => {
    coachTab.dispatchEvent(new MouseEvent("click", { bubbles: true }));
  });
  await act(async () => {});
  const items = [...container.querySelectorAll("li")].filter((li) => li.textContent?.includes("審査コーチ"));
  expect(items).toHaveLength(3);
  const lines = (li: Element) => [...li.querySelectorAll("p.whitespace-pre-wrap")].map((p) => p.textContent);
  expect(lines(items[0])).toEqual(["良い点: 腕振りが良い", "改善点: 接地を直す", "おすすめのドリル: もも上げ", "コーチなら明日から何をするか: 明日から壁押しを毎日やる", "補足: 補足です"]);
  expect(lines(items[1])).toEqual(["良い点: 腕振りが良い", "改善点: 接地を直す", "キュー: 地面を押す", "おすすめのドリル: もも上げ", "成功の目安: 膝がそろう", "次に撮る動画: 真横から", "コーチなら明日から何をするか: 結論"]);
  expect(lines(items[2])).toEqual(["良い点: 腕振りが良い", "改善点: 接地を直す", "おすすめのドリル: もも上げ"]);
  expect(container.textContent).not.toContain("直す点");
});

// 本体 #1086 W10：試用期間の返答に、追加項目（タイトル＋本文）が固定項目の後に出る。
it("W10: 試用期間の返答に追加項目のタイトルと本文が出る", async () => {
  mocks.api.mockImplementation((path: string) =>
    Promise.resolve(
      path.startsWith("/v1/admin/coaching/probation")
        ? {
            responses: [{
              request_id: "r1", coach_id: "coach-1", coach_nickname: "審査コーチ", nth: 1, tier: "coach", event: "100m",
              good_point: "腕振りが良い", fix_points: ["接地を直す"], drill: "もも上げ", clarification: null,
              created_at: "2026-11-27T00:00:00Z", hidden_at: null, hitokoto: "明日から壁押し", cue: null, success_criteria: null, next_video: null,
              offer_items: [{ title: "スタート局面の詳しい分析", body: "低く出られています" }],
            }],
          }
        : path.startsWith("/v1/admin/coach-applications")
          ? { applications: [], meta: { page: 1, per_page: 50, total: 0 } }
          : path.startsWith("/v1/admin/reports") ? emptyList("reports") : emptyList("inquiries"),
    ),
  );
  await act(async () => {
    root.render(<ConfirmProvider><Page /></ConfirmProvider>);
  });
  await act(async () => {});
  const coachTab = Array.from(container.querySelectorAll('[role="tab"]')).find((b) => b.textContent === "コーチ")!;
  await act(async () => {
    coachTab.dispatchEvent(new MouseEvent("click", { bubbles: true }));
  });
  await act(async () => {});
  const item = [...container.querySelectorAll("li")].find((li) => li.textContent?.includes("審査コーチ"))!;
  const lines = [...item.querySelectorAll("p.whitespace-pre-wrap")].map((p) => p.textContent);
  expect(lines).toEqual(["良い点: 腕振りが良い", "改善点: 接地を直す", "おすすめのドリル: もも上げ", "コーチなら明日から何をするか: 明日から壁押し", "スタート局面の詳しい分析: 低く出られています"]);
});
