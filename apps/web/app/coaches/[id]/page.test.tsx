/** @vitest-environment jsdom */
import { readFileSync } from "node:fs";
import path from "node:path";
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
  service_types: ["video_review"], plans: [], support_consult: null, on_duty: true, country_code: "JP",
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

type EntryKey = "card" | "coupon" | "normal" | "reward";
const entry = (key: EntryKey) => container.querySelector(`section[aria-labelledby="coach-video-review"] [data-entry="${key}"]`);
const entryLink = (key: EntryKey) => entry(key)?.querySelector("a[href]")?.getAttribute("href") ?? null;
const entryDisabled = (key: EntryKey) => entry(key)?.querySelector('span[aria-disabled="true"]') != null;
const entryAnchorClass = (key: EntryKey) => entry(key)?.querySelector("a[href]")?.getAttribute("class") ?? "";
const entryKeys = () => [...container.querySelectorAll('section[aria-labelledby="coach-video-review"] [data-entry]')].map((el) => el.getAttribute("data-entry"));
const reviewHref = (pay: string) => `/coaching/new?coach=${coachID}&pay=${pay}`;
function withTickets(summary: Record<string, unknown>, accepting = true) {
  mocks.auth.token = "token";
  mocks.api.mockImplementation(async (path: string) => {
    if (path === `/v1/coaches/${coachID}`) return coach;
    if (path === "/v1/me/coaching/tickets") return { can_purchase: false, sales_open: false, products: [], ...summary };
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
const coupons = (available: number) => ({ coupons: { available, next_expires_at: available ? "2027-05-14T09:00:00Z" : null, amount_yen: 500 } });

async function render() {
  await act(async () => root.render(<CoachDetailPage />));
  await act(async () => {});
}

// チケットの販売は終了した（本体 #1031）。入口は単発・¥500クーポンの2つで、発行済みのチケットが残っている人にだけチケットの入口を足す。
// チケットの購入画面への導線（チケット節・購入ボタン）は置かない。
it("未ログインでは単発と¥500クーポンの2つの入口を有効なまま出し、受付状態を取りに行かない", async () => {
  await render();
  expect(container.querySelector("h2#coach-video-review")?.textContent).toBe("フォーム添削や競技に関する悩み");
  expect(entryKeys()).toEqual(["card", "coupon"]);
  expect(entry("card")?.textContent).toBe("単発1,000円");
  expect(entryLink("card")).toBe(reviewHref("card"));
  // 枚数が分からないときは枚数を出さず、申込画面へ進める。
  expect(entry("coupon")?.textContent).toBe("¥500クーポン");
  expect(entryLink("coupon")).toBe(reviewHref("coupon"));
  expect(container.querySelector("ul.grid-cols-2")).not.toBeNull();
  expect(container.textContent).not.toContain("残り");
  expect(container.textContent).toContain("選択して下さい");
  expect(container.textContent).not.toContain("現在は受付していません");
  expect(mocks.api.mock.calls).toEqual([[`/v1/coaches/${coachID}`]]);
  expect(container.querySelector('section[aria-labelledby="coach-tickets"]')).toBeNull();
  expect(container.querySelector('a[href^="/me/coaching/tickets"]')).toBeNull();
  expect(container.textContent).not.toContain("購入する");
});

// 受付停止バッジ：1か所だけ、見出しより前、枠付き・角丸・赤文字・中央寄せ（本体 #835）。
function expectNotAcceptingBadge(text: string) {
  const badges = Array.from(container.querySelectorAll("p")).filter((el) => el.textContent === text);
  expect(badges).toHaveLength(1);
  const badge = badges[0];
  const section = container.querySelector('section[aria-labelledby="coach-video-review"]');
  const heading = container.querySelector("h2#coach-video-review");
  expect(section?.contains(badge)).toBe(true);
  expect(badge.compareDocumentPosition(heading as Node) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  expect(container.textContent?.split(text).length).toBe(2);
  for (const cls of ["border", "border-red-400/70", "rounded-full", "text-red-400", "text-sm", "font-semibold", "mx-auto", "w-fit", "text-center"]) {
    expect(badge.classList.contains(cls)).toBe(true);
  }
}

it("ログイン済みで受付中なら、クーポンの残り枚数を出して申込画面へ進める", async () => {
  withTickets({ balance: { available: 0, reserved: 0 }, ...byKind(0, 0), ...coupons(1) });
  await render();
  expect(entryKeys()).toEqual(["card", "coupon"]);
  expect(entry("coupon")?.textContent).toBe("¥500クーポン残り 1 枚");
  expect(entryLink("card")).toBe(reviewHref("card"));
  expect(entryLink("coupon")).toBe(reviewHref("coupon"));
  expect(container.textContent).not.toContain("現在は受付していません");
});

it("クーポン0枚は紹介の手順ページへ。受付停止中でもこの入口だけは押せる", async () => {
  withTickets({ balance: { available: 0, reserved: 0 }, ...byKind(0, 0), ...coupons(0) }, false);
  await render();
  expect(entry("coupon")?.textContent).toBe("¥500クーポン残り 0 枚");
  expect(entryLink("coupon")).toBe("/referral");
  expect(entryAnchorClass("coupon")).toContain("bg-blue-600");
  expect(entryLink("card")).toBeNull();
  expect(entryDisabled("card")).toBe(true);
  expectNotAcceptingBadge("現在は受付していません");
});

it("受付停止中でクーポンが1枚以上なら、クーポンの入口も押せない", async () => {
  withTickets({ balance: { available: 0, reserved: 0 }, ...byKind(0, 0), ...coupons(2) }, false);
  await render();
  for (const key of ["card", "coupon"] as const) {
    expect(entryLink(key)).toBeNull();
    expect(entryDisabled(key)).toBe(true);
  }
  expect(container.querySelector('a[href^="/coaching/new"]')).toBeNull();
});

it("発行済みのチケットが残っている人には、チケットの入口を足して3列にする（無料チケットを先に使う）", async () => {
  withTickets({ balance: { available: 3, reserved: 0 }, ...byKind(2, 1), ...coupons(0) });
  await render();
  expect(entryKeys()).toEqual(["card", "coupon", "reward"]);
  expect(entry("reward")?.textContent).toBe("チケット残り 3 枚");
  expect(entryLink("reward")).toBe(reviewHref("reward"));
  expect(container.querySelector("ul.grid-cols-3")).not.toBeNull();
});

it("通常チケットだけが残っていれば、通常チケットで申込画面へ進む", async () => {
  withTickets({ balance: { available: 2, reserved: 0 }, ...byKind(2, 0), ...coupons(0) });
  await render();
  expect(entryKeys()).toEqual(["card", "coupon", "normal"]);
  expect(entryLink("normal")).toBe(reviewHref("normal"));
});

it("チケット0枚ならチケットの入口を出さず、購入画面へも案内しない", async () => {
  withTickets({ balance: { available: 0, reserved: 0 }, ...byKind(0, 0), ...coupons(0) });
  await render();
  expect(entryKeys()).toEqual(["card", "coupon"]);
  expect(container.querySelector('a[href^="/me/coaching/tickets"]')).toBeNull();
});

it("受付状態とチケット要約の取得に失敗しても単発とクーポンの入口を表示する", async () => {
  mocks.auth.token = "token";
  mocks.api.mockImplementation(async (path: string) => {
    if (path === `/v1/coaches/${coachID}`) return coach;
    throw new Error("network down");
  });
  await render();
  expect(entryLink("card")).toBe(reviewHref("card"));
  expect(entryLink("coupon")).toBe(reviewHref("coupon"));
  expect(container.textContent).not.toContain("残り");
});

it("プロフィールは種目チップ・目立つ記録・箇条書きの実績・SNSアイコンで出す", async () => {
  await render();
  const chips = [...container.querySelectorAll("li")].filter((li) => ["100m", "200m"].includes(li.textContent ?? ""));
  expect(chips).toHaveLength(2);
  expect(container.textContent).toContain("全国大会優勝");
  expect(container.textContent).toContain("県記録保持者");
  const items = [...container.querySelectorAll("li")].filter((li) => ["全国大会優勝", "県記録保持者"].includes(li.textContent ?? ""));
  expect(items).toHaveLength(2);
  const sns = container.querySelector('a[href="https://www.instagram.com/testcoach/?utm_source=qr"]');
  expect(sns?.getAttribute("target")).toBe("_blank");
  expect(sns?.getAttribute("aria-label")).toBe("Instagram @testcoach");
  expect(sns?.getAttribute("title")).toBe("Instagram @testcoach");
  expect(sns?.textContent).toBe("");
  expect(sns?.querySelector("svg")).not.toBeNull();
  expect(container.textContent).not.toContain("utm_source");
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
  expect(entry("card")?.textContent).toBe("One-time$6.25");
  // クーポンの名称（¥500 coupon）は商品名なので変えない（本体 #1085 §3）。
  expect(entry("coupon")?.textContent).toBe("¥500 coupon");
  expect(container.textContent).toContain("Not accepting requests right now");
  expectNotAcceptingBadge("Not accepting requests right now");
  expect(container.textContent).toContain("Please select");
  expect(container.querySelector("h2#coach-video-review")?.textContent).toBe("Form feedback or concerns about competing");
  expect(container.textContent).not.toContain("Buy");
});

// 本体 #694 → #1007：継続プランは休眠。tab=support を含め、どのクエリでもフォーム添削（スタンダード）の文脈で、
// 継続プランの節（CoachServicePlans）は描画しない（B3）。
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
  ["tab=support", "tab=support"],
])("フォーム添削文脈（%s）では継続プランの節を描画せず、フォーム添削の節だけを出す", async (_label, search) => {
  mocks.search = search;
  mocks.api.mockImplementation(async (path: string) => {
    if (path === `/v1/coaches/${coachID}`) return supportCoach;
    throw new Error(`unexpected ${path}`);
  });
  await render();
  expect(sectionIDs()).toEqual(["coach-video-review"]);
  expect(container.querySelector("#coach-plans")).toBeNull();
  expect(container.textContent).not.toContain("継続プラン");
  expect(container.textContent).not.toContain("従来の1ヶ月指導");
  expect(container.textContent).not.toContain("このコーチは受付していません");
  expect(entryKeys()).toEqual(["card", "coupon"]);
  expect(container.querySelector("h1")?.textContent).toBe("テストコーチ");
});

// 本体 #869：買う前に分かる情報（回答までの目安）を動画添削節へ前倒し。
// 本体 #909：支払い方法の違いの説明文は削除した。paymentNote は「表示されないこと」の検証用に残す。
const paymentNote = "単発は1回ごとにカード払いです。通常チケットは6枚セットから、無料チケットは紹介でもらった分から、申込み1件に1枚使います。";
const replyNote = "申し込みが確定してから24時間以内に回答が来ます。";

it("動画添削文脈では支払い方法の説明文を表示せず、24時間の文だけを表示する", async () => {
  withTickets({ balance: { available: 3, reserved: 0 }, ...byKind(2, 1) });
  await render();
  const section = container.querySelector('section[aria-labelledby="coach-video-review"]');
  expect(container.textContent).not.toContain(paymentNote);
  expect(section?.textContent).toContain(replyNote);
  // 入口グリッドの直後・節の最後の要素が24時間の文の段落である。
  const grid = section?.querySelector('ul[aria-label="お支払い方法を選んで依頼する"]');
  expect(grid).not.toBeNull();
  expect(section?.lastElementChild?.tagName).toBe("P");
  expect(section?.lastElementChild?.textContent).toBe(replyNote);
  expect(grid?.nextElementSibling).toBe(section?.lastElementChild);
});

it("tab=support で開いても動画添削の節に24時間の文を出し、継続プランの節は出さない（本体 #1007）", async () => {
  mocks.search = "tab=support";
  withTickets({ balance: { available: 3, reserved: 0 }, ...byKind(2, 1) });
  await render();
  expect(sectionIDs()).toEqual(["coach-video-review"]);
  expect(container.querySelector("#coach-plans")).toBeNull();
  expect(container.querySelector('section[aria-labelledby="coach-video-review"]')?.textContent).toContain(replyNote);
  expect(container.textContent).not.toContain(paymentNote);
});

it("受付停止中でも説明文は表示せず、24時間の文は表示し、単発・チケットの入口は無効のままにする", async () => {
  withTickets({ balance: { available: 3, reserved: 0 }, ...byKind(2, 1), ...coupons(1) }, false);
  await render();
  expect(container.textContent).not.toContain(paymentNote);
  expect(container.textContent).toContain(replyNote);
  expectNotAcceptingBadge("現在は受付していません");
  for (const key of ["card", "coupon", "reward"] as const) {
    expect(entryLink(key)).toBeNull();
    expect(entryDisabled(key)).toBe(true);
  }
});

it("未ログイン（枚数未取得）でも説明文は表示せず、24時間の文は表示する", async () => {
  await render();
  expect(mocks.api.mock.calls).toEqual([[`/v1/coaches/${coachID}`]]);
  expect(container.textContent).not.toContain(paymentNote);
  expect(container.textContent).toContain(replyNote);
  expect(entryLink("card")).toBe(reviewHref("card"));
});

it("削除した支払い方法の説明文とその英訳が、製品コードと辞書に残っていない（本体 #909）", () => {
  expect(Object.hasOwn(en, paymentNote)).toBe(false);
  expect(Object.values(en)).not.toContain("Card pays per request. Each request uses one ticket: normal tickets from a pack of 6, free tickets from referrals.");
  const page = readFileSync(path.resolve(process.cwd(), "app/coaches/[id]/page.tsx"), "utf8");
  expect(page).not.toContain(paymentNote);
  expect(page).toContain(replyNote);
});
