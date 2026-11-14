/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import NewReviewRequestPage from "./page";

const mocks = vi.hoisted(() => ({
  api: vi.fn(),
  me: { demo: true, birth_month: "2011-01", is_minor: true, guardian: null, push_devices: 0 } as Record<string, unknown>,
  router: { replace: vi.fn(), push: vi.fn() },
  auth: { session: {} as Record<string, unknown> | null, token: "token" as string | null, loading: false },
}));
vi.mock("next/link", () => ({ default: ({ href, children }: { href: string; children: ReactNode }) => <a href={href}>{children}</a> }));
vi.mock("next/navigation", () => ({ useRouter: () => mocks.router }));
vi.mock("@/lib/cloudinary-upload", () => ({ eagerVideoQuery: "", uploadVideoToCloudinary: async () => ({ public_id: "vid1" }), videoDuration: async () => 1, tooLong: () => false }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => mocks.auth }));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => mocks.api(...args) }));

let container: HTMLDivElement;
let root: Root;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
  mocks.api.mockReset();
  mocks.router.replace.mockReset();
  mocks.router.push.mockReset();
  mocks.auth.session = {};
  mocks.auth.token = "token";
  mocks.auth.loading = false;
  window.history.replaceState(null, "", "/coaching/new");
  mocks.api.mockImplementation(async (path: string) => {
    if (path === "/v1/me/coaching") return mocks.me;
    if (path.startsWith("/v1/coaching/nominees")) return { coaches: [] };
    if (path === "/v1/me/coaching/tickets") return { balance: { available: 0, reserved: 0 }, can_purchase: false };
    if (path.startsWith("/v1/coaching/estimate")) return { price_yen: 1500, payments: true, due_at: "2026-11-06T00:00:00Z" };
    throw new Error(`unexpected ${path}`);
  });
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });

async function render() {
  await act(async () => root.render(<NewReviewRequestPage />));
  await act(async () => {});
}

it("18歳未満は保護者の同意がなくても依頼の画面を開け、支払いの前に保護者の許可のチェックが出る", async () => {
  mocks.me = { demo: true, birth_month: "2011-01", is_minor: true, guardian: null, push_devices: 0 };
  await render();
  expect(container.querySelector("form")).not.toBeNull();
  expect(container.textContent).not.toContain("保護者の方の同意が必要です");
  expect(container.textContent).toContain("未成年の方は保護者に許可を取ってください");
  const box = container.querySelector<HTMLInputElement>('input[type="checkbox"]');
  expect(box?.checked).toBe(false);
  expect(box?.closest("label")?.textContent).toBe("未成年の方は保護者に許可を取ってください");
});

it("18歳以上にも購入前チェックを出す", async () => {
  mocks.me = { demo: true, birth_month: "1995-01", is_minor: false, guardian: null, push_devices: 0 };
  await render();
  expect(container.querySelector("form")).not.toBeNull();
  expect(container.querySelector('input[type="checkbox"]')).not.toBeNull();
  expect(container.textContent).toContain("未成年の方は保護者に許可を取ってください");
});

it("支払い方法は単発だけを出し、PayPay の案内は出さない", async () => {
  mocks.me = { demo: true, birth_month: "1995-01", is_minor: false, guardian: null, push_devices: 0 };
  await render();
  const payment = [...container.querySelectorAll("fieldset")].find((f) => f.querySelector("legend")?.textContent?.startsWith("お支払い"));
  const choices = [...(payment?.querySelectorAll("button") ?? [])].map((b) => b.textContent);
  expect(choices).toEqual(["単発 1,500円／回"]);
  expect(payment?.querySelector("legend")?.textContent).toBe("お支払い方法");
  expect(container.textContent).not.toContain("販売者:");
  expect(container.querySelector('a[href="/terms"], a[href="/legal/commerce"]')).toBeNull();
  expect(container.textContent).not.toContain("料金は返答が返ってきた時点で確定し");
  expect(container.textContent).toContain("申し込みが確定してから24時間以内に回答が来ます。");
  expect(container.textContent).not.toContain("依頼がコーチの対応中一覧に表示されてから24時間以内に返却します。");
  expect(container.textContent).not.toContain("2026/11/06");
  expect(container.textContent).toContain("0/1000文字");
  expect(container.querySelector("textarea")?.hasAttribute("placeholder")).toBe(false);
  expect(container.textContent).not.toContain("PayPay");
});

it("「コーチを選ぶ」の見出しと選択ラジオを出さない", async () => {
  mocks.me = { demo: false, review_open: true, birth_month: "1995-01", is_minor: false, guardian: null, push_devices: 0 };
  await render();
  expect(container.textContent).not.toContain("コーチを選ぶ");
  expect(container.querySelector('input[name="review-coach"]')).toBeNull();
  expect(container.querySelectorAll('input[type="radio"]').length).toBe(0);
});

it("フォーム上部に価格ボックス（動画添削（動画1本））を出さない", async () => {
  mocks.me = { demo: true, birth_month: "1995-01", is_minor: false, guardian: null, push_devices: 0 };
  await render();
  expect(container.textContent).not.toContain("動画添削（動画1本）");
});

it("デモの招待コード画面には販売者の表記とリンクを出さない", async () => {
  mocks.me = { demo: false, review_open: false, birth_month: "1995-01", is_minor: false, guardian: null, push_devices: 0 };
  await render();
  expect(container.querySelector("h1")?.textContent).toBe("動画添削を頼む");
  expect(container.textContent).not.toContain("コーチを選んで、動画を送ります");
  expect(container.textContent).toContain("フォーム添削は準備中です");
  expect(container.querySelector('input[aria-label="招待コード"]')).not.toBeNull();
  expect(container.querySelector('button[type="submit"]')?.textContent).toBe("デモに参加する");
  expect(container.textContent).not.toContain("販売者:");
  expect(container.querySelector('a[href="/terms"], a[href="/legal/commerce"]')).toBeNull();
});

async function renderWithTickets(canPurchase: boolean) {
  mocks.me = { demo: true, birth_month: "1995-01", is_minor: false, guardian: null, push_devices: 0 };
  mocks.api.mockImplementation(async (path: string) => {
    if (path === "/v1/me/coaching") return mocks.me;
    if (path.startsWith("/v1/coaching/nominees")) return { coaches: [] };
    if (path === "/v1/me/coaching/tickets") return { balance: { available: 2, reserved: 0 }, next_expiry: { expires_at: "2027-05-01T00:00:00Z", available: 2 }, can_purchase: canPurchase };
    if (path.startsWith("/v1/coaching/estimate")) return { price_yen: 1500, payments: true, due_at: "2026-11-06T00:00:00Z" };
    throw new Error(`unexpected ${path}`);
  });
  await render();
}

it("チケットを持っている人には、申込画面に残りの枚数といちばん早い有効期限を出し、チケット払いを選べる（本体 #569）", async () => {
  await renderWithTickets(false);
  expect(container.textContent).toContain("チケットの残り 2枚");
  expect(container.textContent).toContain("いちばん早い有効期限");
  expect(container.querySelector('a[href="/me/coaching/tickets?return_to=%2Fcoaching%2Fnew"]')?.textContent).toContain("コーチングチケット");
  expect(container.textContent).toContain("チケット（6枚）2,500円");
  expect(container.textContent).not.toContain("チケット（1枚）");
});

it("チケット残高があるときは単発とチケット（6枚）の二択で、切り替えで内訳と未成年チェックが出し分けられる", async () => {
  await renderWithTickets(true);
  const payment = [...container.querySelectorAll("fieldset")].find((f) => f.querySelector("legend")?.textContent?.startsWith("お支払い"));
  const single = [...(payment?.querySelectorAll("button") ?? [])].find((b) => b.textContent?.startsWith("単発"))!;
  const tickets = [...(payment?.querySelectorAll("button") ?? [])].find((b) => b.textContent?.startsWith("チケット（6枚）"))!;
  expect(single.textContent).toBe("単発 1,500円／回");
  expect(tickets.getAttribute("aria-pressed")).toBe("true");
  expect(single.getAttribute("aria-pressed")).toBe("false");
  expect(container.textContent).not.toContain("カード・Apple Pay・Google Pay");
  expect(container.querySelector('input[type="checkbox"]')).toBeNull();
  await act(async () => { single.click(); });
  expect(single.getAttribute("aria-pressed")).toBe("true");
  expect(tickets.getAttribute("aria-pressed")).toBe("false");
  expect(container.textContent).toContain("カード・Apple Pay・Google Pay");
  expect(container.querySelector('input[type="checkbox"]')).not.toBeNull();
  await act(async () => { tickets.click(); });
  expect(tickets.getAttribute("aria-pressed")).toBe("true");
  expect(container.textContent).not.toContain("カード・Apple Pay・Google Pay");
  expect(container.querySelector('input[type="checkbox"]')).toBeNull();
});

it("チケットがなく購入不可のときはチケットの選択肢が出ず、単発のみになる", async () => {
  mocks.me = { demo: true, birth_month: "1995-01", is_minor: false, guardian: null, push_devices: 0 };
  await render();
  const payment = [...container.querySelectorAll("fieldset")].find((f) => f.querySelector("legend")?.textContent?.startsWith("お支払い"));
  const choices = [...(payment?.querySelectorAll("button") ?? [])].map((b) => b.textContent);
  expect(choices).toEqual(["単発 1,500円／回"]);
  expect(payment?.querySelector("a")).toBeNull();
  expect(container.textContent).not.toContain("チケット（6枚）");
});

it("販売中のチケット持ちには購入・確認の入口を残す", async () => {
  await renderWithTickets(true);
  expect(container.querySelector('a[href="/me/coaching/tickets?return_to=%2Fcoaching%2Fnew"]')?.textContent).toContain("チケットを購入・確認する");
});

it("支払いのある環境では、招待コードなしで申込みの画面を出す（招待コードのゲートを外した。本体 #569）", async () => {
  mocks.me = { demo: false, review_open: true, birth_month: "1995-01", is_minor: false, guardian: null, push_devices: 0 };
  await render();
  expect(container.querySelector("form")).not.toBeNull();
  expect(container.querySelector('input[aria-label="招待コード"]')).toBeNull();
  expect(container.textContent).not.toContain("フォーム添削は準備中です");
});

it("チケットがなく販売中なら、お支払い方法に単発とチケット（6枚）の二択を出す（通常価格や打ち消し線は使わない）", async () => {
  mocks.me = { demo: false, review_open: true, birth_month: "1995-01", is_minor: false, guardian: null, push_devices: 0 };
  mocks.api.mockImplementation(async (path: string) => {
    if (path === "/v1/me/coaching") return mocks.me;
    if (path.startsWith("/v1/coaching/nominees")) return { coaches: [] };
    if (path === "/v1/me/coaching/tickets") return { balance: { available: 0, reserved: 0 }, can_purchase: true };
    if (path.startsWith("/v1/coaching/estimate")) return { price_yen: 500, payments: true, due_at: "2026-11-06T00:00:00Z" };
    throw new Error(`unexpected ${path}`);
  });
  await render();
  const payment = [...container.querySelectorAll("fieldset")].find((f) => f.querySelector("legend")?.textContent?.startsWith("お支払い"));
  const single = [...(payment?.querySelectorAll("button") ?? [])].find((b) => b.textContent?.startsWith("単発"))!;
  expect(single.textContent).toBe("単発 500円／回");
  expect(single.getAttribute("aria-pressed")).toBe("true");
  const link = payment?.querySelector('a[href="/me/coaching/tickets?return_to=%2Fcoaching%2Fnew"]');
  expect(link?.textContent).toContain("チケット（6枚）2,500円");
  expect(container.textContent).toContain("1回あたり約417円（1回ずつ払うと500円）");
  expect(container.textContent).toContain("有効期限は決済から6か月");
  const unitNote = [...(payment?.querySelectorAll("p") ?? [])].find((p) => p.textContent?.includes("1回あたり約"));
  expect(unitNote?.className).not.toMatch(/border|bg-/);
  expect(container.textContent).not.toContain("チケット6枚 2,500円（税込）");
  expect(container.textContent).not.toContain("通常価格");
  expect(container.querySelector("s, del")).toBeNull();
});

it("送る前に最終確認を出し、確定を押すまで動画を上げない", async () => {
  window.history.replaceState(null, "", "/coaching/new?coach=c1");
  mocks.me = { demo: false, review_open: true, birth_month: "1995-01", is_minor: false, guardian: null, push_devices: 0 };
  mocks.api.mockImplementation(async (path: string) => {
    if (path === "/v1/me/coaching") return mocks.me;
    if (path.startsWith("/v1/coaching/nominees")) return { coaches: [{ id: "c1", nickname: "テストコーチ", photo_url: "", events: ["100m"], years: 3, best_record: "", achievements: "" }] };
    if (path === "/v1/me/coaching/tickets") return { balance: { available: 0, reserved: 0 }, can_purchase: false };
    if (path.startsWith("/v1/coaching/estimate")) return { price_yen: 500, payments: true, due_at: "2026-11-06T00:00:00Z" };
    throw new Error(`unexpected ${path}`);
  });
  await render();
  await act(async () => {});
  expect(container.querySelector('a[href="/coaches"]')).toBeNull();
  const textarea = container.querySelector("textarea")!;
  async function enterQuestion(value: string) {
    await act(async () => {
      const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!;
      setter.call(textarea, value);
      textarea.dispatchEvent(new Event("input", { bubbles: true }));
    });
  }
  await enterQuestion("こちらも見てください ｈｔｔｐｓ：／／example.com");
  const input = container.querySelector<HTMLInputElement>('input[type="file"]')!;
  const file = new File(["x"], "run.mp4", { type: "video/mp4" });
  Object.defineProperty(input, "files", { value: [file] });
  await act(async () => { input.dispatchEvent(new Event("change", { bubbles: true })); });
  await act(async () => {});
  await act(async () => container.querySelector<HTMLInputElement>('input[type="checkbox"]')!.click());
  const submit = container.querySelector<HTMLButtonElement>('button[type="submit"]')!;
  expect(submit.textContent).toBe("申込み内容を確認する");
  const contactMessage = "外部の連絡先（URL・メール・SNS IDなど）は送れません。";
  expect(container.textContent).not.toContain(contactMessage);
  await act(async () => { submit.form!.requestSubmit(); });
  expect(container.querySelector("[role=alert]")?.textContent).toBe(contactMessage);
  expect(textarea.getAttribute("aria-invalid")).toBe("true");
  expect(document.activeElement).toBe(textarea);
  expect(container.querySelector('section[aria-labelledby="purchase-confirm-heading"]')).toBeNull();
  expect(mocks.api.mock.calls.some(([path]) => String(path).startsWith("/v1/uploads"))).toBe(false);

  await enterQuestion("後半に失速します。200mのラップは12.1 12.3です");
  expect(container.textContent).not.toContain(contactMessage);
  await act(async () => { submit.form!.requestSubmit(); });
  const confirm = container.querySelector('section[aria-labelledby="purchase-confirm-heading"]');
  expect(confirm?.textContent).toContain("動画添削1回（動画1本・担当 テストコーチ）");
  expect(confirm?.textContent).toContain("500円（税込）");
  expect(confirm?.textContent).toContain("カード・Apple Pay・Google Pay");
  expect(confirm?.textContent).toContain("申し込みが確定してから24時間以内に回答が来ます（再提出時は再提出から24時間以内）");
  expect(confirm?.textContent).not.toContain("依頼がコーチの対応中一覧に表示されてから");
  expect(confirm?.textContent).toContain("回答期限");
  expect(confirm?.textContent).not.toContain("返却期限");
  expect(confirm?.textContent).not.toContain("2026/11/06");
  expect(confirm?.querySelector('a[href="/legal/commerce"]')?.textContent).toBe("特定商取引法に基づく表記");
  expect(confirm?.textContent).toContain("返却時に確定します");
  expect(confirm?.textContent).not.toContain("確定のタイミング");
  expect(confirm?.textContent).toContain("キャンセル・返金");
  expect(confirm?.textContent).toContain("確保を取り消すので代金はかかりません");
  expect(confirm?.textContent).toContain("期限までに返らない場合やコーチ都合の返却では全額を返金します");
  expect(confirm?.textContent).toContain("添削返却後の返金は利用規約に定める場合に限ります");
  expect(confirm?.textContent).not.toContain("着手前に依頼画面でキャンセルした場合、期限までに添削が返らない場合、またはコーチが例外返却した場合は");
  expect(confirm?.textContent).not.toContain("当社・コーチ都合で受けられなかった場合の返金は、利用規約第6条のとおりです。");
  expect(mocks.api.mock.calls.some(([path]) => String(path).startsWith("/v1/uploads"))).toBe(false);
  expect([...confirm!.querySelectorAll("button")].map((b) => b.textContent)).toEqual(["確定してお支払いへ（500円）", "戻って直す"]);
});

const reviewCoach = (id: string) => ({ id, nickname: `コーチ${id}`, photo_url: "", events: ["100m"], years: 3, best_record: "", achievements: "" });

function mockReviewOpen(nominees: ReturnType<typeof reviewCoach>[]) {
  mocks.me = { demo: false, review_open: true, birth_month: "1995-01", is_minor: false, guardian: null, push_devices: 0 };
  mocks.api.mockImplementation(async (path: string) => {
    if (path === "/v1/me/coaching") return mocks.me;
    if (path.startsWith("/v1/coaching/nominees")) return { coaches: nominees };
    if (path === "/v1/me/coaching/tickets") return { balance: { available: 0, reserved: 0 }, can_purchase: false };
    if (path.startsWith("/v1/coaching/estimate")) return { price_yen: 500, payments: true, due_at: "2026-11-06T00:00:00Z" };
    throw new Error(`unexpected ${path}`);
  });
}

it("?coach= で受付中のコーチを初期選択する（本体 #643）", async () => {
  window.history.replaceState(null, "", "/coaching/new?coach=c1");
  mockReviewOpen([reviewCoach("c9"), reviewCoach("c1")]);
  await render();
  await act(async () => {});
  expect(container.textContent).not.toContain("このコーチは現在、動画添削を受け付けていません");
  expect(container.querySelector('a[href="/coaches"]')).toBeNull();
  expect(container.textContent).toContain("受付中のコーチには決済の確認後に依頼が届きます。");
  const textarea = container.querySelector("textarea")!;
  await act(async () => {
    const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!;
    setter.call(textarea, "後半に失速します。");
    textarea.dispatchEvent(new Event("input", { bubbles: true }));
  });
  const input = container.querySelector<HTMLInputElement>('input[type="file"]')!;
  Object.defineProperty(input, "files", { value: [new File(["x"], "run.mp4", { type: "video/mp4" })] });
  await act(async () => { input.dispatchEvent(new Event("change", { bubbles: true })); });
  await act(async () => { container.querySelector<HTMLInputElement>('input[type="checkbox"]')!.click(); });
  expect(container.querySelector<HTMLButtonElement>('button[type="submit"]')!.disabled).toBe(false);
});

it("?coach= のコーチが受付中でない場合は枠なしの明示を出して送信できない（本体 #643）", async () => {
  window.history.replaceState(null, "", "/coaching/new?coach=c1");
  mockReviewOpen([reviewCoach("c9")]);
  await render();
  await act(async () => {});
  const note = container.querySelector('[role="note"]');
  expect(note?.textContent).toBe("このコーチは現在、動画添削を受け付けていません");
  expect(note?.className).not.toMatch(/border|bg-/);
  expect(container.querySelector('a[href="/coaches"]')).not.toBeNull();
  expect(container.querySelector("form")).not.toBeNull();
  const submit = container.querySelector<HTMLButtonElement>('button[type="submit"]')!;
  expect(submit.disabled).toBe(true);
  const textarea = container.querySelector("textarea")!;
  await act(async () => {
    const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!;
    setter.call(textarea, "後半に失速します。");
    textarea.dispatchEvent(new Event("input", { bubbles: true }));
  });
  await act(async () => { submit.form!.requestSubmit(); });
  expect(container.textContent).toContain("コーチを選んでください。");
  expect(container.querySelector('section[aria-labelledby="purchase-confirm-heading"]')).toBeNull();
});

it("coach 指定なしの場合はコーチ一覧リンクを出し、送信時は既存のバリデーションで止まる（本体 #643）", async () => {
  mockReviewOpen([reviewCoach("c9")]);
  await render();
  await act(async () => {});
  expect(container.querySelector('[role="note"]')).toBeNull();
  expect(container.querySelector('a[href="/coaches"]')).not.toBeNull();
  const textarea = container.querySelector("textarea")!;
  await act(async () => {
    const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!;
    setter.call(textarea, "後半に失速します。");
    textarea.dispatchEvent(new Event("input", { bubbles: true }));
  });
  const submit = container.querySelector<HTMLButtonElement>('button[type="submit"]')!;
  await act(async () => { submit.form!.requestSubmit(); });
  expect(container.textContent).toContain("コーチを選んでください。");
  expect(container.querySelector('section[aria-labelledby="purchase-confirm-heading"]')).toBeNull();
});

it("受付中コーチ引き継ぎ時は種目チップを押しても選択が残る（種目は100mのみ）", async () => {
  window.history.replaceState(null, "", "/coaching/new?coach=c1");
  mockReviewOpen([reviewCoach("c9"), reviewCoach("c1")]);
  await render();
  await act(async () => {});
  const eventChip = [...container.querySelectorAll("button")].find((b) => b.textContent === "100m")!;
  await act(async () => { eventChip.click(); });
  await act(async () => {});
  expect(container.querySelector('[role="note"]')).toBeNull();
  expect(container.querySelector('a[href="/coaches"]')).toBeNull();
});

it("24時間お知らせは新文言で枠なしに出て、旧文言は入力画面に出ない", async () => {
  mockReviewOpen([reviewCoach("c9")]);
  await render();
  await act(async () => {});
  const notice = [...container.querySelectorAll("p")].find((p) => p.textContent === "申し込みが確定してから24時間以内に回答が来ます。");
  expect(notice).not.toBeUndefined();
  expect(notice?.className).not.toMatch(/border|bg-/);
  expect(container.textContent).not.toContain("依頼がコーチの対応中一覧に表示されてから24時間以内に返却します。");
});

it("コーチ引き継ぎ時は返金注記が文言そのままで枠なしに出る", async () => {
  window.history.replaceState(null, "", "/coaching/new?coach=c1");
  mockReviewOpen([reviewCoach("c9"), reviewCoach("c1")]);
  await render();
  await act(async () => {});
  const note = [...container.querySelectorAll("p")].find((p) => p.textContent?.includes("受付中のコーチには決済の確認後に依頼が届きます。"));
  expect(note?.textContent).toBe("受付中のコーチには決済の確認後に依頼が届きます。期限までに添削が届かなかった場合は、お支払いを取り消すか全額を返金します。チケットは返還します。");
  expect(note?.className).not.toMatch(/border|bg-/);
});

it("チケット払いで確認画面へ進み、確定で use_tickets: true を送る", async () => {
  window.history.replaceState(null, "", "/coaching/new?coach=c1");
  mocks.me = { demo: false, review_open: true, birth_month: "1995-01", is_minor: false, guardian: null, push_devices: 0 };
  mocks.api.mockImplementation(async (path: string) => {
    if (path === "/v1/me/coaching") return mocks.me;
    if (path.startsWith("/v1/coaching/nominees")) return { coaches: [{ id: "c1", nickname: "テストコーチ", photo_url: "", events: ["100m"], years: 3, best_record: "", achievements: "" }] };
    if (path === "/v1/me/coaching/tickets") return { balance: { available: 2, reserved: 0 }, next_expiry: { expires_at: "2027-05-01T00:00:00Z", available: 2 }, can_purchase: true };
    if (path.startsWith("/v1/coaching/estimate")) return { price_yen: 500, payments: true, due_at: "2026-11-06T00:00:00Z" };
    if (path.startsWith("/v1/uploads/coaching-video")) return { public_id: "vid1" };
    if (path === "/v1/coaching/requests") return { id: "r1" };
    throw new Error(`unexpected ${path}`);
  });
  await render();
  await act(async () => {});
  const tickets = [...container.querySelectorAll("button")].find((b) => b.textContent?.startsWith("チケット（6枚）"))!;
  expect(tickets.getAttribute("aria-pressed")).toBe("true");
  const textarea = container.querySelector("textarea")!;
  await act(async () => {
    const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!;
    setter.call(textarea, "後半に失速します。");
    textarea.dispatchEvent(new Event("input", { bubbles: true }));
  });
  const input = container.querySelector<HTMLInputElement>('input[type="file"]')!;
  const file = new File(["x"], "run.mp4", { type: "video/mp4" });
  Object.defineProperty(input, "files", { value: [file] });
  await act(async () => { input.dispatchEvent(new Event("change", { bubbles: true })); });
  await act(async () => {});
  const submit = container.querySelector<HTMLButtonElement>('button[type="submit"]')!;
  expect(submit.disabled).toBe(false);
  await act(async () => { submit.form!.requestSubmit(); });
  const confirm = container.querySelector('section[aria-labelledby="purchase-confirm-heading"]');
  expect(confirm?.textContent).toContain("確定してチケット1枚で送る");
  const confirmButton = [...confirm!.querySelectorAll("button")].find((b) => b.textContent === "確定してチケット1枚で送る")!;
  await act(async () => { confirmButton.click(); });
  await act(async () => {});
  const posted = mocks.api.mock.calls.find(([path]) => path === "/v1/coaching/requests");
  expect(posted).not.toBeUndefined();
  const body = JSON.parse(String((posted as unknown as [string, { body: string }])[1].body));
  expect(body.use_tickets).toBe(true);
  expect(body.method).toBe("tickets");
  expect(body.requested_coach_id).toBe("c1");
  expect(mocks.router.push).toHaveBeenCalledWith("/coaching/requests/r1");
});

it("ログインを挟んでも coach 指定が落ちない（戻り先にクエリを残す。本体 #643）", async () => {
  window.history.replaceState(null, "", "/coaching/new?coach=c1");
  mocks.auth.session = null;
  mocks.auth.token = null;
  await render();
  expect(mocks.router.replace).toHaveBeenCalledWith("/login?next=%2Fcoaching%2Fnew%3Fcoach%3Dc1");
});

async function renderTicketConfirm(nextExpiry: { expires_at: string; available: number } | null) {
  window.history.replaceState(null, "", "/coaching/new?coach=c1");
  mocks.me = { demo: false, review_open: true, birth_month: "1995-01", is_minor: false, guardian: null, push_devices: 0 };
  mocks.api.mockImplementation(async (path: string) => {
    if (path === "/v1/me/coaching") return mocks.me;
    if (path.startsWith("/v1/coaching/nominees")) return { coaches: [reviewCoach("c1")] };
    if (path === "/v1/me/coaching/tickets") return { balance: { available: 2, reserved: 0 }, next_expiry: nextExpiry, can_purchase: false };
    if (path.startsWith("/v1/coaching/estimate")) return { price_yen: 500, payments: true, due_at: "2026-11-06T00:00:00Z" };
    throw new Error(`unexpected ${path}`);
  });
  await render();
  await act(async () => {});
  const textarea = container.querySelector("textarea")!;
  await act(async () => {
    const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!;
    setter.call(textarea, "後半に失速します。200mのラップは12.1 12.3です");
    textarea.dispatchEvent(new Event("input", { bubbles: true }));
  });
  const input = container.querySelector<HTMLInputElement>('input[type="file"]')!;
  Object.defineProperty(input, "files", { value: [new File(["x"], "run.mp4", { type: "video/mp4" })] });
  await act(async () => { input.dispatchEvent(new Event("change", { bubbles: true })); });
  await act(async () => {});
  const submit = container.querySelector<HTMLButtonElement>('button[type="submit"]')!;
  expect(submit.textContent).toBe("申込み内容を確認する");
  await act(async () => { submit.form!.requestSubmit(); });
  const confirm = container.querySelector('section[aria-labelledby="purchase-confirm-heading"]');
  expect(confirm).not.toBeNull();
  return confirm!;
}

it("チケット版の確認は4項目で、有効期限はお支払い行の中に出る", async () => {
  const confirm = await renderTicketConfirm({ expires_at: "2027-05-01T00:00:00Z", available: 2 });
  expect([...confirm.querySelectorAll("dt")].map((d) => d.textContent)).toEqual([
    "内容",
    "回答期限",
    "お支払い",
    "キャンセル・チケットの戻り",
  ]);
  expect(confirm.textContent).toContain("動画添削1回（動画1本・担当 コーチc1）");
  expect(confirm.textContent).toContain("申し込みが確定してから24時間以内に回答が来ます（再提出時は再提出から24時間以内）");
  expect(confirm.textContent).toContain("有効期限");
  expect(confirm.textContent).toContain("2027");
  expect(confirm.textContent).toContain("追加の支払いなし");
  expect(confirm.textContent).toContain("送信時に1枚を確保し、返却時に使います");
  expect(confirm.textContent).toContain("着手前はキャンセルでき、その場合は1枚戻ります");
  expect(confirm.textContent).toContain("着手後はキャンセルできません");
  expect(confirm.textContent).not.toContain("使うチケットの有効期限");
  expect(confirm.textContent).not.toContain("確定のタイミング");
  expect(confirm.textContent).not.toContain("戻る場合");
  expect(confirm.textContent).not.toContain("依頼がコーチの対応中一覧に表示されてから");
  expect(confirm.textContent).not.toContain("着手前に依頼画面でキャンセルした場合、期限までに添削が返らない場合、またはコーチが例外返却した場合は");
  expect([...confirm.querySelectorAll("button")].map((b) => b.textContent)).toEqual(["確定してチケット1枚で送る", "戻って直す"]);
});

it("チケット版の利用規約第6条は項目ではなく小さい文字の注記で出る", async () => {
  const confirm = await renderTicketConfirm({ expires_at: "2027-05-01T00:00:00Z", available: 2 });
  const note = [...confirm.querySelectorAll("p")].find((p) => p.textContent?.includes("利用規約第6条のとおりです"));
  expect(note?.textContent).toBe("当社・コーチ都合で受けられなかった場合の返金は、利用規約第6条のとおりです。");
  expect(note?.className).toContain("text-xs");
  expect(note?.tagName).toBe("P");
});

it("チケット版の確認で戻って直すと送信せず入力に戻る", async () => {
  const confirm = await renderTicketConfirm({ expires_at: "2027-05-01T00:00:00Z", available: 2 });
  await act(async () => { [...confirm.querySelectorAll("button")].find((b) => b.textContent === "戻って直す")!.click(); });
  expect(container.querySelector('section[aria-labelledby="purchase-confirm-heading"]')).toBeNull();
  expect(container.querySelector("form")).not.toBeNull();
  expect(mocks.api.mock.calls.some(([path]) => String(path).startsWith("/v1/uploads"))).toBe(false);
});

it("有効期限のないチケットはお支払い行に期限なしと出る", async () => {
  const confirm = await renderTicketConfirm(null);
  expect(confirm.textContent).toContain("チケット1枚（有効期限 期限なし（以前に購入した分）・追加の支払いなし）");
});

// 本体 #687：コーチ詳細の入口（?pay=）と、チケットの内訳（通常／無料）の選択。
const normalExpiry = "2027-05-20T00:00:00Z";
const rewardExpiry = "2027-04-01T00:00:00Z";
function kindSummary(normal: number, reward: number) {
  return {
    balance: { available: normal + reward, reserved: 0 },
    next_expiry: normal + reward ? { expires_at: reward ? rewardExpiry : normalExpiry, available: 1 } : null,
    balance_by_kind: {
      normal: { available: normal, next_expires_at: normal ? normalExpiry : null },
      reward: { available: reward, next_expires_at: reward ? rewardExpiry : null },
    },
    can_purchase: true,
  };
}
async function renderKinds(search: string, normal: number, reward: number) {
  window.history.replaceState(null, "", `/coaching/new?coach=c1${search}`);
  mocks.me = { demo: false, review_open: true, birth_month: "1995-01", is_minor: false, guardian: null, push_devices: 0 };
  mocks.api.mockImplementation(async (path: string) => {
    if (path === "/v1/me/coaching") return mocks.me;
    if (path.startsWith("/v1/coaching/nominees")) return { coaches: [reviewCoach("c1")] };
    if (path === "/v1/me/coaching/tickets") return kindSummary(normal, reward);
    if (path.startsWith("/v1/coaching/estimate")) return { price_yen: 500, payments: true, due_at: "2026-11-06T00:00:00Z" };
    if (path.startsWith("/v1/uploads/coaching-video")) return { public_id: "vid1" };
    if (path === "/v1/coaching/requests") return { id: "r1" };
    throw new Error(`unexpected ${path}`);
  });
  await render();
  await act(async () => {});
}
const button = (prefix: string) => [...container.querySelectorAll("button")].find((b) => b.textContent?.startsWith(prefix));
const pressed = (prefix: string) => button(prefix)?.getAttribute("aria-pressed");
async function fillAndConfirm() {
  const textarea = container.querySelector("textarea")!;
  await act(async () => {
    const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!;
    setter.call(textarea, "後半に失速します。");
    textarea.dispatchEvent(new Event("input", { bubbles: true }));
  });
  const input = container.querySelector<HTMLInputElement>('input[type="file"]')!;
  Object.defineProperty(input, "files", { value: [new File(["x"], "run.mp4", { type: "video/mp4" })] });
  await act(async () => { input.dispatchEvent(new Event("change", { bubbles: true })); });
  await act(async () => {});
  await act(async () => { container.querySelector<HTMLButtonElement>('button[type="submit"]')!.form!.requestSubmit(); });
  return container.querySelector('section[aria-labelledby="purchase-confirm-heading"]')!;
}
async function postedBody(confirm: Element, label: string) {
  await act(async () => { [...confirm.querySelectorAll("button")].find((b) => b.textContent === label)!.click(); });
  await act(async () => {});
  const posted = mocks.api.mock.calls.find(([path]) => path === "/v1/coaching/requests");
  return JSON.parse(String((posted as unknown as [string, { body: string }])[1].body));
}

it("入口の指定がなければチケットを選び、内訳は両方あれば無料チケット。内訳は常に出る", async () => {
  await renderKinds("", 2, 1);
  expect(pressed("チケット（6枚）")).toBe("true");
  expect(button("通常チケット（残り 2 枚）")?.disabled).toBe(false);
  expect(pressed("通常チケット")).toBe("false");
  const free = button("無料チケット（残り 1 枚・いちばん早い有効期限 ")!;
  expect(free.getAttribute("aria-pressed")).toBe("true");
  expect(free.textContent).toContain("2027");
  expect(container.textContent).toContain("動画添削1回はチケット1枚です。");
  // 単発に切り替えると内訳は出ない。
  await act(async () => { button("単発")!.click(); });
  expect(button("通常チケット")).toBeUndefined();
});

it("片方だけ持っていれば、その側を初期選択する", async () => {
  await renderKinds("", 2, 0);
  expect(pressed("チケット（6枚）")).toBe("true");
  expect(pressed("通常チケット")).toBe("true");
  const free = button("無料チケット")!;
  expect(free.textContent).toBe("無料チケット（残り 0 枚）");
  expect(free.disabled).toBe(true);
});

it("?pay=card は単発、?pay=normal は通常チケット、?pay=reward は無料チケットを初期選択する", async () => {
  await renderKinds("&pay=card", 2, 1);
  expect(pressed("単発")).toBe("true");
  expect(pressed("チケット（6枚）")).toBe("false");
  await act(async () => root.unmount());
  root = createRoot(container);
  await renderKinds("&pay=normal", 2, 1);
  expect(pressed("チケット（6枚）")).toBe("true");
  expect(pressed("通常チケット")).toBe("true");
  expect(pressed("無料チケット")).toBe("false");
  await act(async () => root.unmount());
  root = createRoot(container);
  await renderKinds("&pay=reward", 2, 1);
  expect(pressed("チケット（6枚）")).toBe("true");
  expect(pressed("無料チケット")).toBe("true");
});

it("指定の種別が使えなくなっていたら使える側に直し、どちらもなければ単発にする", async () => {
  await renderKinds("&pay=reward", 2, 0);
  expect(pressed("チケット（6枚）")).toBe("true");
  expect(pressed("通常チケット")).toBe("true");
  expect(button("無料チケット")?.disabled).toBe(true);
  await act(async () => root.unmount());
  root = createRoot(container);
  await renderKinds("&pay=normal", 0, 0);
  expect(pressed("単発")).toBe("true");
  expect(button("通常チケット")).toBeUndefined();
});

it("無料チケットの確認は無料チケット1枚と特典の期限を出し、ticket_kind: reward を送る", async () => {
  await renderKinds("&pay=reward", 2, 1);
  const confirm = await fillAndConfirm();
  expect([...confirm.querySelectorAll("dt")].map((d) => d.textContent)).toEqual([
    "内容", "回答期限", "お支払い", "使うチケットの有効期限", "キャンセル・チケットの戻り",
  ]);
  expect(confirm.textContent).toContain("無料チケット1枚（このあとのお支払いはありません）");
  const dd = [...confirm.querySelectorAll("dt")].find((d) => d.textContent === "使うチケットの有効期限")!.nextElementSibling!;
  expect(dd.textContent).toContain("2027/04/01");
  const body = await postedBody(confirm, "確定して無料チケット1枚で送る");
  expect(body.method).toBe("tickets");
  expect(body.use_tickets).toBe(true);
  expect(body.ticket_kind).toBe("reward");
});

it("通常チケットの確認は通常ロットの最速期限を出し、ticket_kind: normal を送る", async () => {
  await renderKinds("&pay=normal", 2, 1);
  const confirm = await fillAndConfirm();
  expect([...confirm.querySelectorAll("dt")].map((d) => d.textContent)).toEqual([
    "内容", "回答期限", "お支払い", "キャンセル・チケットの戻り",
  ]);
  // 全体の最速（無料チケットの 4/1）ではなく、通常ロットの中の最速（5/20）。
  expect(confirm.textContent).toContain("2027/05/20");
  expect(confirm.textContent).not.toContain("2027/04/01");
  const body = await postedBody(confirm, "確定してチケット1枚で送る");
  expect(body.ticket_kind).toBe("normal");
});

