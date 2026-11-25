/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { installEnglish, setActiveLocale } from "@/lib/i18n";
import { en } from "@/lib/i18n-en";
import NewReviewRequestPage from "./page";

const mocks = vi.hoisted(() => ({
  api: vi.fn(),
  upload: vi.fn(),
  me: { demo: true, birth_month: "2011-01", is_minor: true, guardian: null, push_devices: 0 } as Record<string, unknown>,
  router: { replace: vi.fn(), push: vi.fn() },
  auth: { session: {} as Record<string, unknown> | null, token: "token" as string | null, loading: false },
}));
vi.mock("next/link", () => ({ default: ({ href, children }: { href: string; children: ReactNode }) => <a href={href}>{children}</a> }));
vi.mock("next/navigation", () => ({ useRouter: () => mocks.router }));
vi.mock("@/lib/cloudinary-upload", () => ({ eagerVideoQuery: "", uploadVideoToCloudinary: (...args: unknown[]) => mocks.upload(...args), videoDuration: async () => 1, tooLong: () => false }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => mocks.auth }));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => mocks.api(...args) }));

let container: HTMLDivElement;
let root: Root;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
  mocks.api.mockReset();
  // アップロードは、受け取った署名の public_id で上がったことにする。
  mocks.upload.mockReset();
  mocks.upload.mockImplementation(async (_file: File, signed: { public_id: string }) => ({ public_id: signed.public_id }));
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
afterEach(async () => { await act(async () => root.unmount()); container.remove(); setActiveLocale("ja"); });

async function render() {
  await act(async () => root.render(<NewReviewRequestPage />));
  await act(async () => {});
}

const paymentFieldset = () => [...container.querySelectorAll("fieldset")].find((f) => f.querySelector("legend")?.textContent?.startsWith("お支払い"));
const paragraphExactly = (text: string) => [...container.querySelectorAll("p")].find((p) => p.textContent === text);
// a が b より DOM 上で後ろにある（b の中には無い）こと。
function expectAfter(a: Node | null | undefined, b: Node | null | undefined) {
  expect(a).toBeTruthy();
  expect(b).toBeTruthy();
  expect(b!.contains(a!)).toBe(false);
  expect(b!.compareDocumentPosition(a!) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
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

it("動画欄には補足文を出さず、ファイル入力に aria-describedby を付けない（本体 #841）", async () => {
  mocks.me = { demo: false, review_open: true, birth_month: "1995-01", is_minor: false, guardian: null, push_devices: 0 };
  await render();
  // 申込の動画は必須で、上限は60秒（#1036）。ラベルに「任意」は付けない。
  expect(container.textContent).toContain("動画（60秒まで）");
  expect(container.textContent).not.toContain("動画（任意・");
  const input = container.querySelector<HTMLInputElement>('input[type="file"]');
  expect(input).not.toBeNull();
  expect(input?.hasAttribute("aria-describedby")).toBe(false);
  expect(container.querySelector("#review-video-help")).toBeNull();
  expect(container.textContent).not.toContain("フォームを見てほしい場合は動画を添えてください。悩みだけでも申し込めます。");
});

it("フォーム上部に価格ボックス（動画添削（動画1本））を出さない", async () => {
  mocks.me = { demo: true, birth_month: "1995-01", is_minor: false, guardian: null, push_devices: 0 };
  await render();
  expect(container.textContent).not.toContain("動画添削（動画1本）");
});

it("デモの招待コード画面には販売者の表記とリンクを出さない", async () => {
  mocks.me = { demo: false, review_open: false, birth_month: "1995-01", is_minor: false, guardian: null, push_devices: 0 };
  await render();
  expect(container.querySelector("h1")?.textContent).toBe("スタンダード（フォーム添削や競技に関する悩み）");
  expect(container.textContent).not.toContain("コーチを選んで、動画を送ります");
  expect(container.textContent).toContain("スタンダードは準備中です");
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
  // チケットの販売は終了した（本体 #1031）。購入画面への入口は置かず、保有状況の画面へだけ案内する。
  expect(container.querySelector('a[href="/me/coaching/tickets"]')?.textContent).toContain("チケット・クーポン");
  expect(container.querySelector('a[href^="/me/coaching/tickets?return_to"]')).toBeNull();
  expect(container.textContent).toContain("チケット（残り 2 枚）");
  expect(container.textContent).not.toContain("チケット（6枚）");
  expect(container.textContent).not.toContain("5,000円");
});

it("チケット残高があるときは単発とチケットの二択で、切り替えで内訳と未成年チェックが出し分けられる", async () => {
  await renderWithTickets(true);
  const payment = [...container.querySelectorAll("fieldset")].find((f) => f.querySelector("legend")?.textContent?.startsWith("お支払い"));
  const single = [...(payment?.querySelectorAll("button") ?? [])].find((b) => b.textContent?.startsWith("単発"))!;
  const tickets = [...(payment?.querySelectorAll("button") ?? [])].find((b) => b.textContent?.startsWith("チケット（残り"))!;
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

it("チケットの残りがあっても購入・確認の入口（購入画面）は出さない（本体 #1031）", async () => {
  await renderWithTickets(true);
  expect(container.textContent).not.toContain("チケットを購入・確認する");
  expect(container.querySelector('a[href^="/me/coaching/tickets?return_to"]')).toBeNull();
});

it("支払いのある環境では、招待コードなしで申込みの画面を出す（招待コードのゲートを外した。本体 #569）", async () => {
  mocks.me = { demo: false, review_open: true, birth_month: "1995-01", is_minor: false, guardian: null, push_devices: 0 };
  await render();
  expect(container.querySelector("form")).not.toBeNull();
  expect(container.querySelector('input[aria-label="招待コード"]')).toBeNull();
  expect(container.textContent).not.toContain("スタンダードは準備中です");
});

it("チケットがなければ、販売終了後はお支払い方法にチケットの購入の入口を出さない（本体 #1031 S3）", async () => {
  mocks.me = { demo: false, review_open: true, birth_month: "1995-01", is_minor: false, guardian: null, push_devices: 0 };
  mocks.api.mockImplementation(async (path: string) => {
    if (path === "/v1/me/coaching") return mocks.me;
    if (path.startsWith("/v1/coaching/nominees")) return { coaches: [] };
    if (path === "/v1/me/coaching/tickets") return { balance: { available: 0, reserved: 0 }, can_purchase: false, sales_open: false, products: [] };
    if (path.startsWith("/v1/coaching/estimate")) return { price_yen: 1000, payments: true, due_at: "2026-11-06T00:00:00Z" };
    throw new Error(`unexpected ${path}`);
  });
  await render();
  const payment = paymentFieldset();
  const choices = [...(payment?.querySelectorAll("button") ?? [])].map((b) => b.textContent);
  expect(choices).toEqual(["単発 1,000円／回"]);
  expect(payment?.querySelector("a")).toBeNull();
  expect(container.textContent).not.toContain("チケット（6枚）");
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
  expect(container.querySelector('a[href="/coaching"]')).toBeNull();
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

  await enterQuestion("  後半に失速します。\n200mのラップは12.1 12.3です  ");
  expect(container.textContent).not.toContain(contactMessage);
  await act(async () => { submit.form!.requestSubmit(); });
  const confirm = container.querySelector('section[aria-labelledby="purchase-confirm-heading"]');
  expect(confirm?.textContent).toContain("スタンダード（動画1本・担当 テストコーチ）");
  expect(confirm?.textContent).toContain("500円（税込）");
  const confirmDts = [...confirm!.querySelectorAll("dt")];
  const rowValue = (label: string) => confirmDts.find((d) => d.textContent === label)!.nextElementSibling as HTMLElement;
  expect(confirmDts.slice(0, 5).map((d) => d.textContent)).toEqual(["内容", "種目", "相談内容", "動画", "回答期限"]);
  expect(rowValue("種目").textContent).toBe("100m");
  expect(rowValue("相談内容").textContent).toBe("後半に失速します。\n200mのラップは12.1 12.3です");
  expect(rowValue("動画").textContent).toBe("run.mp4（1 B）");
  expect(rowValue("相談内容").className).toContain("whitespace-pre-wrap");
  expect(confirmDts.filter((d) => (d.nextElementSibling as HTMLElement).className.includes("whitespace-pre-wrap")).length).toBe(1);
  expect(confirm?.textContent).toContain("カード・Apple Pay・Google Pay");
  expect(confirm?.textContent).toContain("申し込みが確定してから24時間以内に回答が来ます（再提出時は再提出から24時間以内）");
  expect(confirm?.textContent).not.toContain("依頼がコーチの対応中一覧に表示されてから");
  expect(confirm?.textContent).toContain("回答期限");
  expect(confirm?.textContent).not.toContain("返却期限");
  expect(confirm?.textContent).not.toContain("2026/11/06");
  expect(confirm?.querySelector('a[href="/legal/commerce"]')?.textContent).toBe("特定商取引法に基づく表記");
  expect(confirm?.textContent).toContain("回答が返った時に確定します");
  expect(confirm?.textContent).not.toContain("返却時に確定します");
  expect(confirm?.textContent).not.toContain("確定のタイミング");
  expect(confirm?.textContent).toContain("キャンセル・返金");
  expect(confirm?.textContent).toContain("期限までに返らない場合やコーチが返却した場合は、代金はかかりません（確定済みの場合は返金します）");
  expect(confirm?.textContent).not.toContain("確保を取り消すので代金はかかりません");
  expect(confirm?.textContent).not.toContain("決済が確定していない場合は仮押さえを取り消し（請求は確定しません）、確定している場合は返金します");
  expect(confirm?.textContent).toContain("回答返却後の返金は利用規約に定める場合に限ります");
  expect(confirm?.textContent).not.toContain("着手前に依頼画面でキャンセルした場合、期限までに添削が返らない場合、またはコーチが例外返却した場合は");
  expect(confirm?.textContent).not.toContain("当社・コーチ都合で受けられなかった場合の返金は、利用規約第6条のとおりです。");
  // 確認画面を開いた時点で署名だけは先取りする（本体 #917）が、確定を押すまで動画は上げず、依頼も作らない。
  expect(mocks.upload).not.toHaveBeenCalled();
  expect(mocks.api.mock.calls.some(([path]) => path === "/v1/coaching/requests")).toBe(false);
  expect([...confirm!.querySelectorAll("button")].map((b) => b.textContent)).toEqual(["確定してお支払いへ", "戻って直す"]);
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
  expect(container.textContent).not.toContain("このコーチは現在、スタンダードを受け付けていません");
  expect(container.querySelector('a[href="/coaching"]')).toBeNull();
  expect(container.textContent).toContain("期限までに回答が届かなかった場合は、お支払いを取り消すか全額を返金します。チケット・クーポンは返還します。");
  expect(container.textContent).not.toContain("受付中のコーチには決済の確認後に依頼が届きます。");
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
  expect(note?.textContent).toBe("このコーチは現在、スタンダードを受け付けていません");
  expect(note?.className).not.toMatch(/border|bg-/);
  expect(container.querySelector('a[href="/coaching"]')).not.toBeNull();
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
  expect(container.querySelector('a[href="/coaching"]')).not.toBeNull();
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
  expect(container.querySelector('a[href="/coaching"]')).toBeNull();
});

it("24時間お知らせは新文言で枠なしに、お支払い方法の下に出て、旧文言は入力画面に出ない（本体 #841）", async () => {
  mockReviewOpen([reviewCoach("c9")]);
  await render();
  await act(async () => {});
  const notice = paragraphExactly("申し込みが確定してから24時間以内に回答が来ます。");
  expect(notice).not.toBeUndefined();
  expect(notice?.className).not.toMatch(/border|bg-/);
  expectAfter(notice, paymentFieldset());
  expect(container.textContent).not.toContain("依頼がコーチの対応中一覧に表示されてから24時間以内に返却します。");
});

it("コーチ引き継ぎ時は返金注記が単独の文で枠なしに、24時間お知らせの下に出る（本体 #841）", async () => {
  window.history.replaceState(null, "", "/coaching/new?coach=c1");
  mockReviewOpen([reviewCoach("c9"), reviewCoach("c1")]);
  await render();
  await act(async () => {});
  const note = [...container.querySelectorAll("p")].find((p) => p.textContent?.includes("期限までに回答が届かなかった場合は"));
  expect(note?.textContent).toBe("期限までに回答が届かなかった場合は、お支払いを取り消すか全額を返金します。チケット・クーポンは返還します。");
  expect(note?.className).not.toMatch(/border|bg-/);
  expect(container.textContent).not.toContain("受付中のコーチには決済の確認後に依頼が届きます。");
  expectAfter(note, paymentFieldset());
  expectAfter(note, paragraphExactly("申し込みが確定してから24時間以内に回答が来ます。"));
});

it("カード払いでは「カード・Apple Pay・Google Pay」→24時間→返金注記→未成年チェックの順に並ぶ（本体 #841）", async () => {
  window.history.replaceState(null, "", "/coaching/new?coach=c1");
  mocks.me = { demo: false, review_open: true, birth_month: "1995-01", is_minor: false, guardian: null, push_devices: 0 };
  mocks.api.mockImplementation(async (path: string) => {
    if (path === "/v1/me/coaching") return mocks.me;
    if (path.startsWith("/v1/coaching/nominees")) return { coaches: [reviewCoach("c1")] };
    if (path === "/v1/me/coaching/tickets") return { balance: { available: 0, reserved: 0 }, can_purchase: true };
    if (path.startsWith("/v1/coaching/estimate")) return { price_yen: 500, payments: true, due_at: "2026-11-06T00:00:00Z" };
    throw new Error(`unexpected ${path}`);
  });
  await render();
  await act(async () => {});
  const card = paragraphExactly("カード・Apple Pay・Google Pay");
  const hours = paragraphExactly("申し込みが確定してから24時間以内に回答が来ます。");
  const refund = paragraphExactly("期限までに回答が届かなかった場合は、お支払いを取り消すか全額を返金します。チケット・クーポンは返還します。");
  const declaration = container.querySelector<HTMLInputElement>('input[type="checkbox"]')?.closest("label");
  expect(paymentFieldset()?.contains(card!)).toBe(true);
  expectAfter(hours, card);
  expectAfter(refund, hours);
  expectAfter(declaration, refund);
});

it("チケット払いを選んでも24時間お知らせと返金注記はお支払い方法の下に出る（本体 #841）", async () => {
  window.history.replaceState(null, "", "/coaching/new?coach=c1");
  mocks.me = { demo: false, review_open: true, birth_month: "1995-01", is_minor: false, guardian: null, push_devices: 0 };
  mocks.api.mockImplementation(async (path: string) => {
    if (path === "/v1/me/coaching") return mocks.me;
    if (path.startsWith("/v1/coaching/nominees")) return { coaches: [reviewCoach("c1")] };
    if (path === "/v1/me/coaching/tickets") return { balance: { available: 2, reserved: 0 }, next_expiry: { expires_at: "2027-05-01T00:00:00Z", available: 2 }, can_purchase: true };
    if (path.startsWith("/v1/coaching/estimate")) return { price_yen: 500, payments: true, due_at: "2026-11-06T00:00:00Z" };
    throw new Error(`unexpected ${path}`);
  });
  await render();
  await act(async () => {});
  const tickets = [...(paymentFieldset()?.querySelectorAll("button") ?? [])].find((b) => b.textContent?.startsWith("チケット（残り"))!;
  expect(tickets.getAttribute("aria-pressed")).toBe("true");
  expect(container.textContent).not.toContain("カード・Apple Pay・Google Pay");
  expectAfter(paragraphExactly("申し込みが確定してから24時間以内に回答が来ます。"), paymentFieldset());
  expectAfter(paragraphExactly("期限までに回答が届かなかった場合は、お支払いを取り消すか全額を返金します。チケット・クーポンは返還します。"), paymentFieldset());
});

it("英語表示では残した2文が英訳で出て、削除した3文の英訳は出ない（本体 #841）", async () => {
  installEnglish(en);
  setActiveLocale("en");
  window.history.replaceState(null, "", "/coaching/new?coach=c1");
  mocks.me = { demo: false, review_open: true, birth_month: "1995-01", is_minor: false, guardian: null, push_devices: 0 };
  mocks.api.mockImplementation(async (path: string) => {
    if (path === "/v1/me/coaching") return mocks.me;
    if (path.startsWith("/v1/coaching/nominees")) return { coaches: [reviewCoach("c1")] };
    if (path === "/v1/me/coaching/tickets") return { balance: { available: 0, reserved: 0 }, can_purchase: true };
    if (path.startsWith("/v1/coaching/estimate")) return { price_yen: 500, payments: true, due_at: "2026-11-06T00:00:00Z" };
    throw new Error(`unexpected ${path}`);
  });
  await render();
  await act(async () => {});
  const text = container.textContent ?? "";
  expect(paragraphExactly("You'll get a reply within 24 hours of your request being confirmed.")).toBeTruthy();
  expect(paragraphExactly("If feedback doesn't arrive by the deadline, we'll void or fully refund your payment, or return your ticket or coupon.")).toBeTruthy();
  expect(text).not.toContain("Attach a video if you would like form feedback");
  expect(text).not.toContain("per review (");
  expect(text).not.toContain("Paid in advance as a set of");
  expect(text).not.toContain("An available coach receives the request");
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
  const tickets = [...container.querySelectorAll("button")].find((b) => b.textContent?.startsWith("チケット（残り"))!;
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
    "種目",
    "相談内容",
    "動画",
    "回答期限",
    "お支払い",
    "キャンセル・チケットの戻り",
  ]);
  expect(confirm.textContent).toContain("スタンダード（動画1本・担当 コーチc1）");
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
  expect(mocks.upload).not.toHaveBeenCalled();
  expect(mocks.api.mock.calls.some(([path]) => path === "/v1/coaching/requests")).toBe(false);
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
  expect(pressed("チケット（残り")).toBe("true");
  expect(button("通常チケット（残り 2 枚）")?.disabled).toBe(false);
  expect(pressed("通常チケット")).toBe("false");
  const free = button("無料チケット（残り 1 枚・いちばん早い有効期限 ")!;
  expect(free.getAttribute("aria-pressed")).toBe("true");
  expect(free.textContent).toContain("2027");
  expect(container.textContent).toContain("スタンダード1件はチケット1枚です。");
  // 単発に切り替えると内訳は出ない。
  await act(async () => { button("単発")!.click(); });
  expect(button("通常チケット")).toBeUndefined();
});

it("片方だけ持っていれば、その側を初期選択する", async () => {
  await renderKinds("", 2, 0);
  expect(pressed("チケット（残り")).toBe("true");
  expect(pressed("通常チケット")).toBe("true");
  const free = button("無料チケット")!;
  expect(free.textContent).toBe("無料チケット（残り 0 枚）");
  expect(free.disabled).toBe(true);
});

it("?pay=card は単発、?pay=normal は通常チケット、?pay=reward は無料チケットを初期選択する", async () => {
  await renderKinds("&pay=card", 2, 1);
  expect(pressed("単発")).toBe("true");
  expect(pressed("チケット（残り")).toBe("false");
  await act(async () => root.unmount());
  root = createRoot(container);
  await renderKinds("&pay=normal", 2, 1);
  expect(pressed("チケット（残り")).toBe("true");
  expect(pressed("通常チケット")).toBe("true");
  expect(pressed("無料チケット")).toBe("false");
  await act(async () => root.unmount());
  root = createRoot(container);
  await renderKinds("&pay=reward", 2, 1);
  expect(pressed("チケット（残り")).toBe("true");
  expect(pressed("無料チケット")).toBe("true");
});

it("指定の種別が使えなくなっていたら使える側に直し、どちらもなければ単発にする", async () => {
  await renderKinds("&pay=reward", 2, 0);
  expect(pressed("チケット（残り")).toBe("true");
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
    "内容", "種目", "相談内容", "動画", "回答期限", "お支払い", "使うチケットの有効期限", "キャンセル・チケットの戻り",
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
    "内容", "種目", "相談内容", "動画", "回答期限", "お支払い", "キャンセル・チケットの戻り",
  ]);
  // 全体の最速（無料チケットの 4/1）ではなく、通常ロットの中の最速（5/20）。
  expect(confirm.textContent).toContain("2027/05/20");
  expect(confirm.textContent).not.toContain("2027/04/01");
  const body = await postedBody(confirm, "確定してチケット1枚で送る");
  expect(body.ticket_kind).toBe("normal");
});


// 動画は必須（#1036）なので、動画を付けたうえで相談内容の必須を確かめる。
it.each([true])("相談内容を必須にして申込を送る（動画あり=%s）", async (hasVideo) => {
  window.history.replaceState(null, "", "/coaching/new?coach=c1");
  mockReviewOpen([reviewCoach("c1")]);
  const mockRead = mocks.api.getMockImplementation()!;
  mocks.api.mockImplementation(async (path: string, options?: unknown) => {
    if (path === "/v1/uploads/coaching-video") return { public_id: "vid1" };
    if (path === "/v1/coaching/requests") return { id: "r1" };
    return mockRead(path, options);
  });
  await render();
  expect(container.textContent).toContain("動画（60秒まで）");
  expect(container.textContent).not.toContain("動画（任意・");
  expect(container.textContent).not.toContain("悩みだけでも申し込めます。");

  if (hasVideo) {
    const input = container.querySelector<HTMLInputElement>('input[type="file"]')!;
    Object.defineProperty(input, "files", { value: [new File(["x"], "run.mp4", { type: "video/mp4" })] });
    await act(async () => input.dispatchEvent(new Event("change", { bubbles: true })));
  }
  await act(async () => container.querySelector<HTMLInputElement>('input[type="checkbox"]')!.click());
  const form = container.querySelector("form")!;
  const textarea = form.querySelector("textarea")!;
  expect(form.checkValidity()).toBe(false);
  await act(async () => form.requestSubmit());
  expect(container.querySelector('section[aria-labelledby="purchase-confirm-heading"]')).toBeNull();
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!.call(textarea, "大会前の練習について相談したいです");
    textarea.dispatchEvent(new Event("input", { bubbles: true }));
  });
  await act(async () => form.requestSubmit());
  const confirm = container.querySelector('section[aria-labelledby="purchase-confirm-heading"]')!;
  expect(confirm.textContent).toContain(hasVideo ? "スタンダード（動画1本・担当 コーチc1）" : "スタンダード（悩みのみ・担当 コーチc1）");
  const videoDd = [...confirm.querySelectorAll("dt")].find((d) => d.textContent === "動画")!.nextElementSibling!;
  expect(videoDd.textContent).toBe(hasVideo ? "run.mp4（1 B）" : "なし");
  await act(async () => confirm.querySelector("button")!.click());
  const paths = mocks.api.mock.calls.map(([path]) => path);
  expect(paths.includes("/v1/uploads/coaching-video")).toBe(hasVideo);
  const request = mocks.api.mock.calls.find(([path]) => path === "/v1/coaching/requests")!;
  const body = JSON.parse(request[1].body);
  expect(body.question).toBe("大会前の練習について相談したいです");
  expect(body.video_public_id).toBe(hasVideo ? "vid1" : undefined);
  expect(mocks.router.push).toHaveBeenCalledWith("/coaching/requests/r1");
});

// 送信ボタンが押せない理由の表示（本体 #870）。ボタンの条件自体は変えない。
it("コーチ未選択では理由を表示し、コーチを選ぶと消える", async () => {
  mockReviewOpen([reviewCoach("c1")]);
  await render();
  await act(async () => {});
  const coachReason = paragraphExactly("コーチを選んでください。");
  expect(coachReason).toBeTruthy();
  // 理由の領域は注記トーン（text-xs）。
  expect(coachReason?.parentElement?.className).toContain("text-xs");
  expect(container.querySelector<HTMLButtonElement>('button[type="submit"]')!.disabled).toBe(true);
  // コーチを選ぶ（?coach= で開き直す）。
  await act(async () => root.unmount());
  root = createRoot(container);
  window.history.replaceState(null, "", "/coaching/new?coach=c1");
  mockReviewOpen([reviewCoach("c1")]);
  await render();
  await act(async () => {});
  expect(paragraphExactly("コーチを選んでください。")).toBeFalsy();
});

it("未成年宣言が必要で未チェックだと理由を表示し、チェックすると消える", async () => {
  window.history.replaceState(null, "", "/coaching/new?coach=c1");
  mockReviewOpen([reviewCoach("c1")]);
  await render();
  await act(async () => {});
  expect(paragraphExactly("コーチを選んでください。")).toBeFalsy();
  expect(paragraphExactly("未成年の方は、保護者の同意のチェックが必要です。")).toBeTruthy();
  await act(async () => { container.querySelector<HTMLInputElement>('input[type="checkbox"]')!.click(); });
  expect(paragraphExactly("未成年の方は、保護者の同意のチェックが必要です。")).toBeFalsy();
});

it("コーチ未選択かつ宣言未チェックのとき、理由が2件とも確認ボタンの直下に表示される", async () => {
  mockReviewOpen([reviewCoach("c1")]);
  await render();
  await act(async () => {});
  const coachReason = paragraphExactly("コーチを選んでください。");
  const declarationReason = paragraphExactly("未成年の方は、保護者の同意のチェックが必要です。");
  expect(coachReason).toBeTruthy();
  expect(declarationReason).toBeTruthy();
  const submit = container.querySelector<HTMLButtonElement>('button[type="submit"]')!;
  expect(submit.disabled).toBe(true);
  // 理由の領域は1つで、確認ボタンの直後にある。
  const region = submit.nextElementSibling;
  expect(region?.getAttribute("role")).toBe("status");
  expect(region?.contains(coachReason!)).toBe(true);
  expect(region?.contains(declarationReason!)).toBe(true);
});

it("条件が全てそろうと理由が表示されず、確認ボタンが有効になる", async () => {
  window.history.replaceState(null, "", "/coaching/new?coach=c1");
  mockReviewOpen([reviewCoach("c1")]);
  await render();
  await act(async () => {});
  await act(async () => { container.querySelector<HTMLInputElement>('input[type="checkbox"]')!.click(); });
  expect(container.querySelector<HTMLButtonElement>('button[type="submit"]')!.disabled).toBe(false);
  expect(paragraphExactly("コーチを選んでください。")).toBeFalsy();
  expect(paragraphExactly("未成年の方は、保護者の同意のチェックが必要です。")).toBeFalsy();
});

it("未成年宣言の理由は英語でも出る", async () => {
  installEnglish(en);
  setActiveLocale("en");
  window.history.replaceState(null, "", "/coaching/new?coach=c1");
  mockReviewOpen([reviewCoach("c1")]);
  await render();
  await act(async () => {});
  expect(paragraphExactly("Minors must check the parental consent box.")).toBeTruthy();
});

// ---- 本体 #917：確定後の待ち表示と、動画の署名の先取り ----

const signPath = "/v1/uploads/coaching-video";
const signCalls = () => mocks.api.mock.calls.filter(([path]) => path === signPath).length;
const confirmSection = () => container.querySelector('section[aria-labelledby="purchase-confirm-heading"]');
const pendingStatus = () => confirmSection()?.querySelector('[role="status"]');

// 申込みの画面を開いて、相談内容（と動画）を入れ、最終確認を開く。
// sign は署名の取得の応答、request は /v1/coaching/requests の応答を返す。
// 申込の動画は必須（#1036）なので、既定で動画を1本選んでから確認画面を開く。
async function openReviewConfirm({ tickets = false, video = new File(["x"], "run.mp4", { type: "video/mp4" }) as File | null, sign = async () => ({ public_id: "pre1" }) as unknown, request = async () => ({ id: "r1" }) as unknown }) {
  window.history.replaceState(null, "", "/coaching/new?coach=c1");
  mocks.me = { demo: false, review_open: true, birth_month: "1995-01", is_minor: false, guardian: null, push_devices: 0 };
  mocks.api.mockImplementation(async (path: string) => {
    if (path === "/v1/me/coaching") return mocks.me;
    if (path.startsWith("/v1/coaching/nominees")) return { coaches: [reviewCoach("c1")] };
    if (path === "/v1/me/coaching/tickets") return tickets
      ? { balance: { available: 2, reserved: 0 }, next_expiry: { expires_at: "2027-05-01T00:00:00Z", available: 2 }, can_purchase: true }
      : { balance: { available: 0, reserved: 0 }, can_purchase: false };
    if (path.startsWith("/v1/coaching/estimate")) return { price_yen: 1500, payments: true, due_at: "2026-11-06T00:00:00Z" };
    if (path === signPath) return sign();
    if (path === "/v1/coaching/requests") return request();
    throw new Error(`unexpected ${path}`);
  });
  await render();
  await act(async () => {});
  const textarea = container.querySelector("textarea")!;
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!.call(textarea, "後半に失速します。");
    textarea.dispatchEvent(new Event("input", { bubbles: true }));
  });
  if (video) await pickVideo(video);
  if (!tickets) await act(async () => container.querySelector<HTMLInputElement>('input[type="checkbox"]')!.click());
  await act(async () => container.querySelector("form")!.requestSubmit());
  await act(async () => {});
  expect(confirmSection()).not.toBeNull();
}

async function pickVideo(video: File) {
  const input = container.querySelector<HTMLInputElement>('input[type="file"]')!;
  Object.defineProperty(input, "files", { value: [video], configurable: true });
  await act(async () => input.dispatchEvent(new Event("change", { bubbles: true })));
  await act(async () => {});
}

async function pressConfirm() {
  await act(async () => confirmSection()!.querySelector("button")!.click());
  await act(async () => {});
}

it("A4: カード払いの確定後は「お支払い画面へ移動しています」とストップウォッチを出し、紹介IDの確認中から出る", async () => {
  await openReviewConfirm({ request: () => new Promise(() => {}) });
  expect(pendingStatus()).toBeFalsy();
  await pressConfirm();
  expect(pendingStatus()?.textContent).toBe("お支払い画面へ移動しています");
  expect(pendingStatus()?.querySelector("svg.screen-loader-watch")).not.toBeNull();
  expect([...confirmSection()!.querySelectorAll("button")].map((b) => b.disabled)).toEqual([true, true]);
});

it("A4: チケットで送る確定後は既定の「処理しています」を出す", async () => {
  await openReviewConfirm({ tickets: true, request: () => new Promise(() => {}) });
  expect(confirmSection()!.querySelector("button")!.textContent).toBe("確定してチケット1枚で送る");
  await pressConfirm();
  expect(pendingStatus()?.textContent).toBe("処理しています");
  expect(pendingStatus()?.querySelector("svg.screen-loader-watch")).not.toBeNull();
});

it("A4: 送れなかったときは待ち表示が消えてエラー文が出る（二重送信の防止は従来どおり）", async () => {
  let fail: (err: Error) => void = () => {};
  await openReviewConfirm({ request: () => new Promise((_, reject) => { fail = reject; }) });
  await pressConfirm();
  expect(pendingStatus()?.textContent).toBe("お支払い画面へ移動しています");
  // 待っているあいだにもう一度押しても、申込みは1回しか送らない。
  await act(async () => confirmSection()!.querySelector("button")!.click());
  expect(mocks.api.mock.calls.filter(([path]) => path === "/v1/coaching/requests")).toHaveLength(1);
  await act(async () => fail(new Error("送信に失敗しました")));
  await act(async () => {});
  expect(container.querySelector('[role="status"] svg.screen-loader-watch')).toBeNull();
  expect(container.textContent).not.toContain("お支払い画面へ移動しています");
  expect(container.textContent).toContain("送信に失敗しました");
});

it("B1: 動画ありで確認画面を開くと署名を1回だけ先取りし、確定後は取り直さずにその署名で上げる", async () => {
  const video = new File(["x"], "run.mp4", { type: "video/mp4" });
  await openReviewConfirm({ video });
  expect(signCalls()).toBe(1);
  expect(mocks.upload).not.toHaveBeenCalled();
  await pressConfirm();
  expect(signCalls()).toBe(1);
  expect(mocks.upload).toHaveBeenCalledTimes(1);
  expect(mocks.upload.mock.calls[0][0]).toBe(video);
  expect(mocks.upload.mock.calls[0][1]).toEqual({ public_id: "pre1" });
  const body = JSON.parse(mocks.api.mock.calls.find(([path]) => path === "/v1/coaching/requests")![1].body);
  expect(body.video_public_id).toBe("pre1");
  expect(mocks.router.push).toHaveBeenCalledWith("/coaching/requests/r1");
});

it("B1: 戻って同じ動画のまま確認を開き直しても、署名は取り直さない", async () => {
  await openReviewConfirm({ video: new File(["x"], "run.mp4", { type: "video/mp4" }) });
  await act(async () => [...confirmSection()!.querySelectorAll("button")].find((b) => b.textContent === "戻って直す")!.click());
  await act(async () => container.querySelector("form")!.requestSubmit());
  await act(async () => {});
  expect(signCalls()).toBe(1);
});

it("B2: 先取りに失敗しても画面に出さず、確定後に署名を取り直して続ける", async () => {
  let n = 0;
  await openReviewConfirm({
    video: new File(["x"], "run.mp4", { type: "video/mp4" }),
    sign: async () => { if (++n === 1) throw new Error("署名を取れませんでした"); return { public_id: "late1" }; },
  });
  expect(signCalls()).toBe(1);
  expect(container.querySelector('[role="alert"]')).toBeNull();
  expect(container.textContent).not.toContain("署名を取れませんでした");
  await pressConfirm();
  expect(signCalls()).toBe(2);
  expect(mocks.upload.mock.calls[0][1]).toEqual({ public_id: "late1" });
  expect(container.textContent).not.toContain("署名を取れませんでした");
  expect(mocks.router.push).toHaveBeenCalledWith("/coaching/requests/r1");
});

it("B2: 先取りから50分を過ぎていたら使わず、確定後に取り直す", async () => {
  const now = vi.spyOn(Date, "now");
  const start = Date.UTC(2026, 10, 18, 0, 0, 0);
  now.mockReturnValue(start);
  let n = 0;
  try {
    await openReviewConfirm({ video: new File(["x"], "run.mp4", { type: "video/mp4" }), sign: async () => ({ public_id: `sig${++n}` }) });
    expect(signCalls()).toBe(1);
    now.mockReturnValue(start + 50 * 60 * 1000);
    await pressConfirm();
    expect(signCalls()).toBe(2);
    expect(mocks.upload.mock.calls[0][1]).toEqual({ public_id: "sig2" });
  } finally {
    now.mockRestore();
  }
});

it("B3: 確認画面を開いた後に動画を選び直すと、先取り分は使わず新しい署名で送る", async () => {
  let n = 0;
  await openReviewConfirm({ video: new File(["x"], "first.mp4", { type: "video/mp4" }), sign: async () => ({ public_id: `sig${++n}` }) });
  expect(signCalls()).toBe(1);
  await act(async () => [...confirmSection()!.querySelectorAll("button")].find((b) => b.textContent === "戻って直す")!.click());
  const second = new File(["y"], "second.mp4", { type: "video/mp4" });
  await pickVideo(second);
  await act(async () => container.querySelector("form")!.requestSubmit());
  await act(async () => {});
  await pressConfirm();
  expect(signCalls()).toBe(2);
  expect(mocks.upload).toHaveBeenCalledTimes(1);
  expect(mocks.upload.mock.calls[0][0]).toBe(second);
  expect(mocks.upload.mock.calls[0][1]).toEqual({ public_id: "sig2" });
  const body = JSON.parse(mocks.api.mock.calls.find(([path]) => path === "/v1/coaching/requests")![1].body);
  expect(body.video_public_id).toBe("sig2");
});

it("先取りした署名でも、署名と違う public_id で上がったら従来どおり止める", async () => {
  mocks.upload.mockImplementation(async () => ({ public_id: "other" }));
  await openReviewConfirm({ video: new File(["x"], "run.mp4", { type: "video/mp4" }) });
  await pressConfirm();
  expect(mocks.api.mock.calls.some(([path]) => path === "/v1/coaching/requests")).toBe(false);
  expect(container.textContent).toContain("アップロードに失敗しました");
});

// W3（#1036）: 申込の動画は必須。動画を選ばないと確認画面へ進まず、署名も申込も送らない。カード・チケット・クーポンのどれでも同じ。
it.each([
  ["カード", "", 0, 0, "単発"],
  ["チケット", "", 0, 2, "チケット（残り"],
  ["クーポン", "&pay=coupon", 1, 0, "¥500クーポン"],
] as const)("W3: 動画未選択では確認画面へ進まず「動画を選んでください」を出す（%s）", async (_label, search, coupons, tickets, chip) => {
  await renderCoupons(search, coupons, tickets);
  expect(pressed(chip)).toBe("true");
  const declaration = container.querySelector<HTMLInputElement>('input[type="checkbox"]');
  if (declaration && !declaration.checked) await act(async () => declaration.click());
  const textarea = container.querySelector("textarea")!;
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!.call(textarea, "後半に失速します。");
    textarea.dispatchEvent(new Event("input", { bubbles: true }));
  });
  await act(async () => container.querySelector("form")!.requestSubmit());
  await act(async () => {});
  expect(confirmSection()).toBeNull();
  expect(container.textContent).toContain("動画を選んでください");
  expect(mocks.api.mock.calls.some(([path]) => String(path).startsWith("/v1/uploads/coaching-video"))).toBe(false);
  expect(mocks.api.mock.calls.some(([path]) => path === "/v1/coaching/requests")).toBe(false);
  // 動画を選べば従来どおり確認画面へ進める。
  await pickVideo(new File(["x"], "run.mp4", { type: "video/mp4" }));
  await act(async () => container.querySelector("form")!.requestSubmit());
  await act(async () => {});
  expect(confirmSection()).not.toBeNull();
  expect(container.textContent).not.toContain("動画を選んでください");
});

it("#959: 動画送信・取り込み・保存からStripe遷移まで確認画面は待ちオーバーレイだけを出す", async () => {
  let progress!: (value: { loaded: number; total: number; percent: number }) => void;
  let finishUpload!: (value: { public_id: string }) => void;
  let finishRequest!: (value: { id: string; checkout_url: string }) => void;
  const assign = vi.fn();
  try {
    mocks.upload.mockImplementation((_file: File, _signed: unknown, _token: unknown, onProgress: typeof progress) => {
      progress = onProgress;
      return new Promise((resolve) => { finishUpload = resolve; });
    });
    await openReviewConfirm({
      video: new File(["1234567890"], "run.mp4", { type: "video/mp4" }),
      request: () => new Promise((resolve) => { finishRequest = resolve; }),
    });
    vi.stubGlobal("location", { ...window.location, assign });
    await pressConfirm();
    const expectOverlayOnly = () => {
      expect(pendingStatus()?.textContent).toBe("お支払い画面へ移動しています");
      expect(container.querySelector('[role="progressbar"]')).toBeNull();
      for (const text of ["動画を送っています", "取り込み中", "投稿を保存しています"]) expect(container.textContent).not.toContain(text);
      // ファイル名は既存の申込み内容の行だけ。進捗カードには出さない。
      expect([...container.querySelectorAll("p")].some((p) => p.textContent === "run.mp4")).toBe(false);
    };
    expectOverlayOnly();
    await act(async () => progress({ loaded: 5, total: 10, percent: 50 }));
    expectOverlayOnly();
    await act(async () => progress({ loaded: 10, total: 10, percent: 100 }));
    expectOverlayOnly();
    await act(async () => finishUpload({ public_id: "pre1" }));
    expectOverlayOnly();
    expect(signCalls()).toBe(1);
    expect(mocks.upload).toHaveBeenCalledTimes(1);
    const body = JSON.parse(mocks.api.mock.calls.find(([path]) => path === "/v1/coaching/requests")![1].body);
    expect(body.video_public_id).toBe("pre1");
    await act(async () => finishRequest({ id: "r1", checkout_url: "https://checkout.stripe.com/test" }));
    expect(assign).toHaveBeenCalledWith("https://checkout.stripe.com/test");
    expectOverlayOnly();
  } finally { vi.unstubAllGlobals(); }
});

// 本体 #1013 補足 B4: 申込フォームに任意4欄（PB・目標タイム・風速・その時の体のコンディション）が出て、
// 入力した項目だけが確認画面の行と送信内容に入る。未入力の項目は確認行に出ず、送信内容では null。
it.each([true, false])("B4: 選手情報の任意4欄を確認画面と送信内容に渡す（入力あり=%s）", async (filled) => {
  window.history.replaceState(null, "", "/coaching/new?coach=c1");
  mockReviewOpen([reviewCoach("c1")]);
  const mockRead = mocks.api.getMockImplementation()!;
  mocks.api.mockImplementation(async (path: string, options?: unknown) => {
    if (path.startsWith("/v1/uploads/coaching-video")) return { public_id: "vid1" };
    if (path === "/v1/coaching/requests") return { id: "r1" };
    return mockRead(path, options);
  });
  await render();
  const form = container.querySelector("form")!;
  const info = [...form.querySelectorAll("fieldset")].find((f) => f.querySelector("legend")?.textContent === "選手情報（任意）")!;
  expect([...info.querySelectorAll("label > span")].map((s) => s.textContent)).toEqual(["PB（自己ベスト）", "目標タイム", "風速", "その時の体のコンディション"]);
  // 相談内容の下に置く。
  expectAfter(info, form.querySelector("#review-question"));
  const inputs = [...info.querySelectorAll<HTMLInputElement | HTMLTextAreaElement>("input, textarea")];
  expect(inputs.map((i) => i.name)).toEqual(["pb", "goal_time", "wind_speed", "body_condition"]);
  expect(inputs.map((i) => i.required)).toEqual([false, false, false, false]);
  expect(inputs.map((i) => i.maxLength)).toEqual([100, 100, 50, 300]);
  const setValue = async (el: HTMLInputElement | HTMLTextAreaElement, value: string) => {
    await act(async () => {
      const proto = el instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
      Object.getOwnPropertyDescriptor(proto, "value")!.set!.call(el, value);
      el.dispatchEvent(new Event("input", { bubbles: true }));
    });
  };
  await setValue(form.querySelector("textarea")!, "大会前の練習について相談したいです");
  if (filled) {
    await setValue(inputs[0], "11秒20（2025年）");
    await setValue(inputs[2], "+1.2");
    await setValue(inputs[3], "前日に脚が張っていた");
  }
  // 申込の動画は必須（#1036）。
  await pickVideo(new File(["x"], "run.mp4", { type: "video/mp4" }));
  await act(async () => container.querySelector<HTMLInputElement>('input[type="checkbox"]')!.click());
  await act(async () => form.requestSubmit());
  const confirm = container.querySelector('section[aria-labelledby="purchase-confirm-heading"]')!;
  const labels = [...confirm.querySelectorAll("dt")].map((d) => d.textContent);
  const at = labels.indexOf("相談内容");
  if (filled) {
    expect(labels.slice(at, at + 4)).toEqual(["相談内容", "PB（自己ベスト）", "風速", "その時の体のコンディション"]);
    const value = (label: string) => [...confirm.querySelectorAll("dt")].find((d) => d.textContent === label)!.nextElementSibling!.textContent;
    expect(value("PB（自己ベスト）")).toBe("11秒20（2025年）");
    expect(value("風速")).toBe("+1.2");
    expect(value("その時の体のコンディション")).toBe("前日に脚が張っていた");
  }
  for (const label of filled ? ["目標タイム"] : ["PB（自己ベスト）", "目標タイム", "風速", "その時の体のコンディション"]) {
    expect(labels).not.toContain(label);
  }
  await act(async () => confirm.querySelector("button")!.click());
  const request = mocks.api.mock.calls.find(([path]) => path === "/v1/coaching/requests")!;
  const body = JSON.parse(request[1].body);
  expect(body.question).toBe("大会前の練習について相談したいです");
  expect({ pb: body.pb, goal_time: body.goal_time, wind_speed: body.wind_speed, body_condition: body.body_condition }).toEqual(
    filled
      ? { pb: "11秒20（2025年）", goal_time: null, wind_speed: "+1.2", body_condition: "前日に脚が張っていた" }
      : { pb: null, goal_time: null, wind_speed: null, body_condition: null },
  );
  expect(body).not.toHaveProperty("goal_meet");
  expect(body).not.toHaveProperty("filming_context");
});

it("B4: 選手情報に連絡先らしき文字列があると確認画面に進まず、その欄に指摘が出る", async () => {
  window.history.replaceState(null, "", "/coaching/new?coach=c1");
  mockReviewOpen([reviewCoach("c1")]);
  await render();
  const form = container.querySelector("form")!;
  const textarea = form.querySelector("textarea")!;
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!.call(textarea, "相談です");
    textarea.dispatchEvent(new Event("input", { bubbles: true }));
  });
  const goal = form.querySelector<HTMLInputElement>('input[name="goal_time"]')!;
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!.call(goal, "090-1234-5678");
    goal.dispatchEvent(new Event("input", { bubbles: true }));
  });
  await act(async () => container.querySelector<HTMLInputElement>('input[type="checkbox"]')!.click());
  await act(async () => form.requestSubmit());
  expect(container.querySelector('section[aria-labelledby="purchase-confirm-heading"]')).toBeNull();
  expect(goal.closest("label")!.querySelector('[role="alert"]')?.textContent).toContain("外部の連絡先");
});

// ---- 本体 #1031：紹介特典の ¥500 クーポン ----
async function renderCoupons(search: string, coupons: number, tickets = 0, price = 1000) {
  window.history.replaceState(null, "", `/coaching/new?coach=c1${search}`);
  mocks.me = { demo: false, review_open: true, birth_month: "1995-01", is_minor: false, guardian: null, push_devices: 0 };
  mocks.api.mockImplementation(async (path: string) => {
    if (path === "/v1/me/coaching") return mocks.me;
    if (path.startsWith("/v1/coaching/nominees")) return { coaches: [reviewCoach("c1")] };
    if (path === "/v1/me/coaching/tickets") return {
      ...kindSummary(tickets, 0), can_purchase: false, sales_open: false,
      coupons: { available: coupons, next_expires_at: coupons ? "2027-05-14T09:00:00Z" : null, amount_yen: 500 },
    };
    if (path.startsWith("/v1/coaching/estimate")) return { price_yen: price, payments: true, due_at: "2026-11-06T00:00:00Z" };
    if (path.startsWith("/v1/uploads/coaching-video")) return { public_id: "vid1" };
    if (path === "/v1/coaching/requests") return { id: "r1" };
    throw new Error(`unexpected ${path}`);
  });
  await render();
  await act(async () => {});
}

it("クーポンを持っている人にはクーポンの選択肢を出し、選ぶと内訳（価格−クーポン＝お支払い）を出す", async () => {
  await renderCoupons("", 2);
  const chipButton = button("¥500クーポン（残り 2 枚）");
  expect(chipButton).toBeDefined();
  // 入口の指定がなければ、チケット→クーポン→単発の順に使えるものを選ぶ。
  expect(chipButton!.getAttribute("aria-pressed")).toBe("true");
  await act(async () => { button("単発")!.click(); });
  expect(container.querySelector('[aria-label="クーポンの内訳"]')).toBeNull();
  await act(async () => { chipButton!.click(); });
  expect(chipButton!.getAttribute("aria-pressed")).toBe("true");
  const breakdown = container.querySelector('[aria-label="クーポンの内訳"]');
  expect(breakdown?.textContent).toContain("価格 1,000円 − クーポン 500円 ＝ お支払い 500円");
  expect(breakdown?.textContent).toContain("使うクーポンの有効期限：");
  expect(breakdown?.textContent).toContain("カード・Apple Pay・Google Pay");
});

it("クーポンがなければクーポンの選択肢を出さない", async () => {
  await renderCoupons("", 0);
  expect(button("¥500クーポン")).toBeUndefined();
  expect(container.querySelector('[aria-label="クーポンの内訳"]')).toBeNull();
});

it("?pay=coupon はクーポンを初期選択し、確認画面に内訳を出して use_coupon で送る（紹介IDは入れない）", async () => {
  await renderCoupons("&pay=coupon", 1);
  expect(pressed("¥500クーポン")).toBe("true");
  // 差額をカードで払うので、購入ごとの確認のチェックが要る。
  await act(async () => { container.querySelector<HTMLInputElement>('input[type="checkbox"]')!.click(); });
  const confirm = await fillAndConfirm();
  const rows = confirm.textContent ?? "";
  expect(rows).toContain("価格1,000円（税込）");
  expect(rows).toContain("クーポン−500円（¥500クーポン1枚）");
  expect(rows).toContain("お支払い額500円（税込）");
  expect(confirm.querySelector('input[aria-label="紹介ID"]')).toBeNull();
  const body = await postedBody(confirm, "確定してお支払いへ");
  expect(body.method).toBe("card");
  expect(body.use_coupon).toBe(true);
  expect(body.use_tickets).toBe(false);
  expect(body.referral_code).toBeUndefined();
});

it("価格の全額をクーポンで割り引くときは、このあとのお支払いがないと示し、クーポンで送る", async () => {
  await renderCoupons("&pay=coupon", 1, 0, 500);
  expect(container.querySelector('[aria-label="クーポンの内訳"]')?.textContent).toContain("お支払い 0円");
  expect(container.textContent).toContain("クーポンで全額を払うので、このあとのお支払いはありません。");
  const confirm = await fillAndConfirm();
  expect(confirm.textContent).toContain("キャンセル・クーポンの戻り");
  expect(confirm.textContent).not.toContain("お支払い方法");
  const body = await postedBody(confirm, "確定してクーポンで送る");
  expect(body.use_coupon).toBe(true);
});

it("チケットとクーポンの両方があるときは、どちらか一方だけを選べる（併用しない）", async () => {
  await renderCoupons("&pay=coupon", 1, 2);
  expect(pressed("¥500クーポン")).toBe("true");
  expect(pressed("チケット（残り")).toBe("false");
  await act(async () => { button("チケット（残り")!.click(); });
  expect(pressed("¥500クーポン")).toBe("false");
  expect(pressed("チケット（残り")).toBe("true");
  expect(container.querySelector('[aria-label="クーポンの内訳"]')).toBeNull();
});
