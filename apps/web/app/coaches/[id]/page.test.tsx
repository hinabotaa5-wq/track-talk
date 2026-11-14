/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { installEnglish, setActiveLocale } from "@/lib/i18n";
import { en } from "@/lib/i18n-en";
import { rememberCoachDetailPreview } from "@/lib/coach-detail-preview";
import type { PublicCoach } from "@/lib/types";
import CoachDetailPage from "./page";

const coachID = "12345678-1234-1234-1234-123456789abc";

const mocks = vi.hoisted(() => ({
  api: vi.fn(),
  auth: { session: null, token: null as string | null, loading: false },
  search: "",
}));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => mocks.api(...args) }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => mocks.auth }));
vi.mock("next/navigation", () => ({ useParams: () => ({ id: coachID }), useSearchParams: () => new URLSearchParams(mocks.search) }));
vi.mock("next/link", () => ({ default: ({ href, className, children }: { href: string; className?: string; children: React.ReactNode }) => <a href={href} className={className}>{children}</a> }));

const coach: PublicCoach = {
  id: coachID, nickname: "テストコーチ", photo_url: "", tier: "coach", events: ["100m", "200m"],
  best_record: "10秒00", achievements: "全国大会優勝\n県記録保持者", years: 10,
  sns_urls: ["https://www.instagram.com/testcoach/?utm_source=qr", "note-test"],
  hitokoto: "よろしくお願いします", rating_count: 0, rating_average: 0,
  service_types: ["video_review"], plans: [], support_consult: null,
};

let container: HTMLDivElement;
let root: Root;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
  mocks.api.mockReset();
  mocks.auth.session = null;
  mocks.auth.token = null;
  mocks.search = "";
  rememberCoachDetailPreview(coach, 0);
  mocks.api.mockImplementation(async (path: string) => {
    if (path === `/v1/coaches/${coachID}`) return coach;
    throw new Error(`unexpected ${path}`);
  });
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); setActiveLocale("ja"); });

const entry = (key: "card" | "normal" | "reward") => container.querySelector(`section[aria-labelledby="coach-video-review"] [data-entry="${key}"]`);
const entryLink = (key: "card" | "normal" | "reward") => entry(key)?.querySelector("a[href]")?.getAttribute("href") ?? null;
const entryDisabled = (key: "card" | "normal" | "reward") => entry(key)?.querySelector('span[aria-disabled="true"]') != null;
const reviewHref = (pay: string) => `/coaching/new?coach=${coachID}&pay=${pay}`;
// 本体 #691：購入を完了したらこのコーチ詳細に戻る購入画面へのリンク（2つの入口で同じ）。
const ticketPurchaseHref = `/me/coaching/tickets?return_to=${encodeURIComponent(`/coaches/${coachID}`)}`;
function withTickets(summary: Record<string, unknown>, accepting = true) {
  mocks.auth.token = "token";
  mocks.api.mockImplementation(async (path: string) => {
    if (path === `/v1/coaches/${coachID}`) return coach;
    if (path === "/v1/me/coaching/tickets") return { can_purchase: true, ...summary };
    if (path.startsWith("/v1/coaching/nominees")) {
      return { coaches: accepting ? [{ id: coachID, nickname: "テストコーチ", photo_url: "", events: ["100m"], years: 10, best_record: "", achievements: "" }] : [] };
    }
    throw new Error(`unexpected ${path}`);
  });
}
const byKind = (normal: number, reward: number) => ({ balance_by_kind: {
  normal: { available: normal, next_expires_at: normal ? "2027-05-01T00:00:00Z" : null },
  reward: { available: reward, next_expires_at: reward ? "2027-04-01T00:00:00Z" : null },
} });

async function render() {
  await act(async () => root.render(<CoachDetailPage />));
  await act(async () => {});
}

it("未ログインではフォーム添削の節と3つの入口を有効なまま出し、受付状態を取りに行かない", async () => {
  await render();
  expect(container.textContent).toContain("フォーム添削");
  expect(container.querySelectorAll('section[aria-labelledby="coach-video-review"] [data-entry]')).toHaveLength(3);
  expect(entry("card")?.textContent).toBe("単発500円");
  expect(entryLink("card")).toBe(reviewHref("card"));
  // 枚数が分からないときは枚数を出さず、申込画面へ進める。
  expect(entry("normal")?.textContent).toBe("通常チケット");
  expect(entryLink("normal")).toBe(reviewHref("normal"));
  expect(entry("reward")?.textContent).toBe("無料チケット");
  expect(entryLink("reward")).toBe(reviewHref("reward"));
  expect(container.textContent).not.toContain("残り");
  expect(container.textContent).toContain("500円");
  expect(container.textContent).toContain("チケットも使えます");
  expect(container.textContent).not.toContain("現在は受付していません");
  expect(mocks.api.mock.calls).toEqual([[`/v1/coaches/${coachID}`]]);
  const tickets = container.querySelector('section[aria-labelledby="coach-tickets"]');
  expect(tickets?.querySelector("h2")?.textContent).toBe("チケット");
  expect(tickets?.querySelector("p")?.textContent).toBe("チケット6枚で2,500円です。添削1回につき1枚消費します。");
  expect(tickets?.querySelector(`a[href="${ticketPurchaseHref}"]`)?.textContent).toBe("購入する");
  const purchaseClass = tickets?.querySelector("a")?.className ?? "";
  expect(purchaseClass).toContain("bg-blue-600");
  expect(purchaseClass).toContain("text-pure-white");
  expect(purchaseClass).not.toContain("bg-white/10");
  expect(tickets?.previousElementSibling?.getAttribute("aria-labelledby")).toBe("coach-video-review");
  // フォーム添削文脈（既定）ではチケット節が最後の節で、継続プラン節は続かない（本体 #694）。
  expect(tickets?.nextElementSibling).toBeNull();
  expect(tickets?.className).toBe(tickets?.previousElementSibling?.className);
  expect(tickets?.querySelector("a")?.classList.contains("w-full")).toBe(true);
});

it("受付停止中でもチケットを購入できる", async () => {
  mocks.auth.token = "token";
  mocks.api.mockImplementation(async (path: string, init?: { token?: string | null }) => {
    if (path === `/v1/coaches/${coachID}`) return coach;
    if (path === "/v1/me/coaching/tickets") return { can_purchase: true };
    if (path.startsWith("/v1/coaching/nominees")) {
      expect(init?.token).toBe("token");
      return { coaches: [] };
    }
    throw new Error(`unexpected ${path}`);
  });
  await render();
  expect(container.querySelector('a[href^="/coaching/new"]')).toBeNull();
  for (const key of ["card", "normal", "reward"] as const) expect(entryDisabled(key)).toBe(true);
  expect(container.textContent).toContain("現在は受付していません");
  const purchaseLink = container.querySelector('section[aria-labelledby="coach-tickets"] a[href]');
  expect(purchaseLink?.textContent).toBe("購入する");
  expect(purchaseLink?.hasAttribute("aria-disabled")).toBe(false);
  expect(mocks.api).toHaveBeenCalledWith("/v1/me/coaching/tickets", { token: "token" });
});

it("ログイン済みでチケットを購入できないときはチケット節を表示しない", async () => {
  mocks.auth.token = "token";
  mocks.api.mockImplementation(async (path: string) => {
    if (path === `/v1/coaches/${coachID}`) return coach;
    if (path === "/v1/me/coaching/tickets") return { can_purchase: false };
    if (path.startsWith("/v1/coaching/nominees")) return { coaches: [] };
    throw new Error(`unexpected ${path}`);
  });
  await render();
  expect(container.querySelector('section[aria-labelledby="coach-tickets"]')).toBeNull();
});

it("ログイン済みで受付中なら3つの入口を有効にする", async () => {
  mocks.auth.token = "token";
  mocks.api.mockImplementation(async (path: string) => {
    if (path === `/v1/coaches/${coachID}`) return coach;
    if (path.startsWith("/v1/coaching/nominees")) {
      return { coaches: [{ id: coachID, nickname: "テストコーチ", photo_url: "", events: ["100m"], years: 10, best_record: "", achievements: "" }] };
    }
    throw new Error(`unexpected ${path}`);
  });
  await render();
  expect(entryLink("card")).toBe(reviewHref("card"));
  expect(entryLink("normal")).toBe(reviewHref("normal"));
  expect(entryLink("reward")).toBe(reviewHref("reward"));
  expect(container.textContent).not.toContain("現在は受付していません");
});

it("種別ごとの残り枚数を出し、それぞれの支払い方式で申込画面へ進む", async () => {
  withTickets({ balance: { available: 3, reserved: 0 }, ...byKind(2, 1) });
  await render();
  expect(entry("card")?.textContent).toBe("単発500円");
  expect(entry("normal")?.textContent).toBe("通常チケット残り 2 枚");
  expect(entry("reward")?.textContent).toBe("無料チケット残り 1 枚");
  expect(entryLink("card")).toBe(reviewHref("card"));
  expect(entryLink("normal")).toBe(reviewHref("normal"));
  expect(entryLink("reward")).toBe(reviewHref("reward"));
});

it("通常チケット0枚は購入画面へ、無料チケット0枚は押せない", async () => {
  withTickets({ balance: { available: 0, reserved: 0 }, ...byKind(0, 0) });
  await render();
  expect(entry("normal")?.textContent).toBe("通常チケット残り 0 枚");
  expect(entryLink("normal")).toBe(ticketPurchaseHref);
  // 「チケット」節の「購入する」と同じ遷移先・同じ戻り先（入口で動きが割れない）。
  expect(container.querySelector('section[aria-labelledby="coach-tickets"] a')?.getAttribute("href")).toBe(ticketPurchaseHref);
  expect(entry("reward")?.textContent).toBe("無料チケット残り 0 枚");
  expect(entryLink("reward")).toBeNull();
  expect(entryDisabled("reward")).toBe(true);
  expect(entryLink("card")).toBe(reviewHref("card"));
});

it("受付していないコーチでは通常チケット0枚でも購入へ遷移させず、チケット節の購入は押せる", async () => {
  withTickets({ balance: { available: 0, reserved: 0 }, ...byKind(0, 0) }, false);
  await render();
  for (const key of ["card", "normal", "reward"] as const) {
    expect(entryLink(key)).toBeNull();
    expect(entryDisabled(key)).toBe(true);
  }
  expect(entry("normal")?.textContent).toBe("通常チケット残り 0 枚");
  expect(container.textContent).toContain("現在は受付していません");
  expect(container.querySelector(`section[aria-labelledby="coach-tickets"] a[href="${ticketPurchaseHref}"]`)?.textContent).toBe("購入する");
});

it("受付状態とチケット要約の取得に失敗しても両方のボタンを表示する", async () => {
  mocks.auth.token = "token";
  mocks.api.mockImplementation(async (path: string) => {
    if (path === `/v1/coaches/${coachID}`) return coach;
    throw new Error("network down");
  });
  await render();
  expect(entryLink("card")).toBe(reviewHref("card"));
  expect(entryLink("normal")).toBe(reviewHref("normal"));
  expect(entryLink("reward")).toBe(reviewHref("reward"));
  expect(container.textContent).not.toContain("残り");
  expect(container.querySelector('section[aria-labelledby="coach-tickets"] a')?.textContent).toBe("購入する");
});

it("プロフィールは種目チップ・目立つ記録・箇条書きの実績・短いSNSラベルで出す", async () => {
  await render();
  const chips = [...container.querySelectorAll("li")].filter((li) => ["100m", "200m"].includes(li.textContent ?? ""));
  expect(chips).toHaveLength(2);
  expect(container.textContent).toContain("全国大会優勝");
  expect(container.textContent).toContain("県記録保持者");
  const items = [...container.querySelectorAll("li")].filter((li) => ["全国大会優勝", "県記録保持者"].includes(li.textContent ?? ""));
  expect(items).toHaveLength(2);
  const sns = container.querySelector('a[href="https://www.instagram.com/testcoach/?utm_source=qr"]');
  expect(sns?.getAttribute("target")).toBe("_blank");
  expect(sns?.textContent).toContain("Instagram");
  expect(sns?.textContent).toContain("@testcoach");
  expect(sns?.textContent).not.toContain("utm_source");
  expect(container.textContent).toContain("note-test");
});

it("評価データがあっても平均評価（★）と「評価はまだありません」は出ない", async () => {
  mocks.api.mockImplementation(async (path: string) => {
    if (path === `/v1/coaches/${coachID}`) return { ...coach, rating_count: 5, rating_average: 4.2 };
    throw new Error(`unexpected ${path}`);
  });
  await render();
  expect(container.textContent).not.toContain("★");
  expect(container.textContent).not.toContain("評価はまだありません");
  expect(container.textContent).toContain("テストコーチ");
});

it("英語では依頼ボタンと注記を英語で出す", async () => {
  installEnglish(en);
  setActiveLocale("en");
  mocks.auth.token = "token";
  mocks.api.mockImplementation(async (path: string) => {
    if (path === `/v1/coaches/${coachID}`) return coach;
    if (path.startsWith("/v1/coaching/nominees")) return { coaches: [] };
    throw new Error(`unexpected ${path}`);
  });
  await render();
  expect(entry("card")?.textContent).toBe("One-time¥500");
  expect(entry("normal")?.textContent).toBe("Regular ticket");
  expect(entry("reward")?.textContent).toBe("Free ticket");
  expect(container.textContent).toContain("Not accepting requests right now");
  const tickets = container.querySelector('section[aria-labelledby="coach-tickets"]');
  expect(tickets?.querySelector("h2")?.textContent).toBe("Tickets");
  expect(tickets?.querySelector("p")?.textContent).toBe("6 tickets for ¥2,500. One ticket is used per video review.");
  expect(tickets?.querySelector("a")?.textContent).toBe("Buy");
});

// 本体 #694：一覧で切り替えた先のプランの節だけを出す。tab=support だけがサポート文脈、それ以外はフォーム添削文脈。
const sectionIDs = () => [...container.querySelectorAll("section[aria-labelledby]")].map((s) => s.getAttribute("aria-labelledby"));
const supportCoach: PublicCoach = {
  ...coach,
  // 従来の継続プラン（service_type 未設定の公開プラン）。継続プラン節の中に一覧で出る。
  plans: [{ id: "plan-s", title: "従来の1ヶ月指導", kind: "period", description: "説明", duration_days: 30,
    price_yen: 8000, capacity: 3, published: true,
    video_reviews: 4, menu_updates: null, reply_formats: [], start_on_first_video: false }],
};

it.each([
  ["指定なし", ""],
  ["tab=video", "tab=video"],
  ["不正値", "tab=unknown"],
])("フォーム添削文脈（%s）では継続プランの節を描画せず、フォーム添削とチケットの節を出す", async (_label, search) => {
  mocks.search = search;
  mocks.api.mockImplementation(async (path: string) => {
    if (path === `/v1/coaches/${coachID}`) return supportCoach;
    throw new Error(`unexpected ${path}`);
  });
  await render();
  expect(sectionIDs()).toEqual(["coach-video-review", "coach-tickets"]);
  expect(container.querySelector("#coach-plans")).toBeNull();
  expect(container.textContent).not.toContain("継続プラン");
  expect(container.textContent).not.toContain("従来の1ヶ月指導");
  expect(container.textContent).not.toContain("このコーチは受付していません");
  expect(container.querySelectorAll('section[aria-labelledby="coach-video-review"] [data-entry]')).toHaveLength(3);
  expect(container.querySelector(`section[aria-labelledby="coach-tickets"] a[href="${ticketPurchaseHref}"]`)?.textContent).toBe("購入する");
  expect(container.querySelector("h1")?.textContent).toBe("テストコーチ");
});

it("サポート文脈（tab=support）では継続プランの節だけを出し、フォーム添削とチケットの節を描画しない", async () => {
  mocks.search = "tab=support";
  mocks.api.mockImplementation(async (path: string) => {
    if (path === `/v1/coaches/${coachID}`) return supportCoach;
    throw new Error(`unexpected ${path}`);
  });
  await render();
  expect(sectionIDs()).toEqual(["coach-plans"]);
  expect(container.querySelector("#coach-plans")?.textContent).toBe("継続プラン");
  expect(container.textContent).toContain("このコーチは受付していません");
  expect(container.textContent).toContain("従来の1ヶ月指導");
  expect(container.querySelector('section[aria-labelledby="coach-video-review"]')).toBeNull();
  expect(container.querySelector('section[aria-labelledby="coach-tickets"]')).toBeNull();
  expect(container.querySelector('a[href^="/coaching/new"]')).toBeNull();
  expect(container.querySelector('a[href^="/me/coaching/tickets"]')).toBeNull();
  // プロフィール節は文脈によらず出す。
  expect(container.querySelector("h1")?.textContent).toBe("テストコーチ");
  expect(container.textContent).toContain("全国大会優勝");
});

it("サポート文脈ではフォーム添削の受付状態とチケットの要約を取りに行かない", async () => {
  mocks.search = "tab=support";
  withTickets({ balance: { available: 3, reserved: 0 }, ...byKind(2, 1) });
  await render();
  expect(mocks.api.mock.calls).toEqual([[`/v1/coaches/${coachID}`]]);
  expect(sectionIDs()).toEqual(["coach-plans"]);
});
