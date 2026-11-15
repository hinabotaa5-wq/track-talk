/** @vitest-environment jsdom */
import { renderToStaticMarkup } from "react-dom/server";
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { CoachCommerceDisclosure, CoachCommerceSection } from "./coach-commerce";
import type { CoachCommerce } from "@/lib/types";

const mocks = vi.hoisted(() => ({ identity: vi.fn() }));
vi.mock("@/components/identity-session", () => ({
  IdentitySession: ({ children }: { children: (token: string) => ReactNode }) => children("vault-token"),
  identityFetch: (...args: unknown[]) => mocks.identity(...args),
}));

it("表記要のコーチは、氏名・住所・電話番号をプランのページに出す", () => {
  const html = renderToStaticMarkup(<CoachCommerceDisclosure commerce={{ required: true, registered: true, legal_name: "山田太郎", address: "東京都千代田区", phone: "090-1234-5678" }} />);
  expect(html).toContain("山田太郎");
  expect(html).toContain("東京都千代田区");
  expect(html).toContain("090-1234-5678");
  const container = document.createElement("div");
  container.innerHTML = html;
  const rows = Array.from(container.querySelectorAll("dl > div")).map((row) => [
    row.querySelector("dt")?.textContent,
    row.querySelector("dd")?.textContent,
  ]);
  expect(rows).toEqual([
    ["販売者", "山田太郎"],
    ["住所", "東京都千代田区"],
    ["電話番号", "090-1234-5678"],
    ["販売価格・提供期間・返金の条件", "このページのプランの内容のとおりです。"],
    ["支払方法・支払時期", "コーチが申し込みを承諾したあと、TrackTalk の決済画面でクレジットカード決済を行います。"],
  ]);
});

it("表記不要と申告したコーチは、表記がないことだけを伝える", () => {
  const html = renderToStaticMarkup(<CoachCommerceDisclosure commerce={{ required: false }} />);
  expect(html).toContain("表記はありません");
  expect(html).not.toContain("住所");
});

it("表記要でも本人が確認していないあいだは、電話番号を含め身元情報を出さない", () => {
  const html = renderToStaticMarkup(<CoachCommerceDisclosure commerce={{ required: true, registered: false }} />);
  expect(html).toContain("準備中");
  expect(html).not.toContain("電話番号");
  expect(html).not.toContain("住所");
});

// 合成データのみ。実在の個人情報は使わない。
function commerce(overrides: Partial<CoachCommerce> = {}): CoachCommerce {
  return {
    required: true,
    forced: false,
    exempt_declared_at: null,
    checklist_version: 7,
    checklist: ["合成条件1", "合成条件2"],
    identity: { legal_name: "合成太郎", address: "合成県合成市1-2-3", phone: "090-0000-0000" },
    identity_revision: "rev-1",
    plans: [],
    ...overrides,
  };
}

describe("進み具合と次にやること", () => {
  let container: HTMLDivElement;
  let root: Root;
  beforeEach(() => {
    (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    mocks.identity.mockReset();
    container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
  });
  afterEach(async () => { await act(async () => root.unmount()); container.remove(); });

  async function renderSection() {
    await act(async () => { root.render(<CoachCommerceSection />); });
  }

  it("身元未登録では、ステップ1が次はこれで、次にやることが身元登録を指す", async () => {
    mocks.identity.mockResolvedValue(commerce({ identity: null }));
    await renderSection();
    expect(container.textContent).toContain("ステップ1 身元情報の登録");
    expect(container.textContent).toContain("次はこれ");
    expect(container.textContent).toContain("次にやること：身元情報を登録してください");
  });

  it("電話番号だけ欠ける登録は未完了で登録し直しを促す", async () => {
    mocks.identity.mockResolvedValue(commerce({ identity: { legal_name: "合成太郎", address: "合成県合成市1-2-3", phone: "" } }));
    await renderSection();
    expect(container.textContent).toContain("電話番号の登録し直しが必要です");
    expect(container.textContent).toContain("次にやること：身元情報を登録してください");
  });

  it("表記不要なら電話番号が空でもステップ1は完了", async () => {
    mocks.identity.mockResolvedValue(commerce({ required: false, identity: { legal_name: "合成太郎", address: "合成県合成市1-2-3", phone: "" } }));
    await renderSection();
    expect(container.textContent).not.toContain("電話番号の登録し直しが必要です");
    expect(container.textContent).not.toContain("次はこれ");
  });

  it("身元が終わりプラン0件では、ステップ3が待ちで、次にやることがプラン待ちの案内になる", async () => {
    mocks.identity.mockResolvedValue(commerce());
    await renderSection();
    expect(container.textContent).toContain("ステップ3 プランの公開設定");
    expect(container.textContent).toContain("待ち");
    expect(container.textContent).toContain("プランがまだありません");
  });

  it("2件中1件だけ承認済みでは、ステップ3にあと1件と出て、次にやることが公開設定を指す", async () => {
    mocks.identity.mockResolvedValue(commerce({
      plans: [
        { id: "plan-1", title: "合成プラン1", authorized: true },
        { id: "plan-2", title: "合成プラン2", authorized: false },
      ],
    }));
    await renderSection();
    expect(container.textContent).toContain("あと1件");
    expect(container.textContent).toContain("次にやること：未承認のプランの公開設定をしてください");
  });

  it("全プラン承認済みでは、全ステップ完了で、次にやることが完了表示になる", async () => {
    mocks.identity.mockResolvedValue(commerce({
      plans: [
        { id: "plan-1", title: "合成プラン1", authorized: true },
        { id: "plan-2", title: "合成プラン2", authorized: true },
      ],
    }));
    await renderSection();
    expect(container.textContent).toContain("完了：公開設定まで終わっています");
    expect(container.textContent).not.toContain("次はこれ");
    expect(container.textContent).not.toContain("待ち");
    expect(container.textContent).not.toContain("あと1件");
  });

  it("プランが1件だけのときはプルダウンが初期選択済みで、確認チェックを入れれば保存できる", async () => {
    mocks.identity.mockResolvedValue(commerce({
      required: false,
      plans: [{ id: "plan-1", title: "合成プラン", authorized: false }],
    }));
    await renderSection();
    expect((container.querySelector("select") as HTMLSelectElement).value).toBe("plan-1");
    await act(async () => { (container.querySelector('input[type="checkbox"]') as HTMLInputElement).click(); });
    const save = Array.from(container.querySelectorAll("button")).find((b) => b.textContent === "公開設定を保存する") as HTMLButtonElement;
    expect(save.disabled).toBe(false);
    await act(async () => { save.click(); });
    expect(mocks.identity).toHaveBeenCalledWith("/v1/coach/disclosures/plan-1", "vault-token", "PUT", { confirm: true, identity_revision: "rev-1" });
  });

  it("申告済みの表示がステップ2に正しく出る", async () => {
    mocks.identity.mockResolvedValue(commerce({ required: false, exempt_declared_at: "2026-11-11T05:00:00.000Z" }));
    await renderSection();
    expect(container.textContent).toContain("表記しない（申告済み）");
    expect(container.textContent).toContain("申告しました");
  });

  it("運営指定の表示がステップ2に正しく出る", async () => {
    mocks.identity.mockResolvedValue(commerce({ required: true, forced: true }));
    await renderSection();
    expect(container.textContent).toContain("表記する");
    expect(container.textContent).toContain("運営指定");
  });

  async function flush() {
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });
  }

  function bulkForm() {
    const forms = Array.from(container.querySelectorAll("form"));
    return forms.find((f) => f.textContent?.includes("未承認のプランを一括で承認する")) as HTMLFormElement;
  }

  it("未承認が2件以上で一括ブロックが出て、対象タイトルが全件表示される。確認チェックを入れるまでボタンが押せない", async () => {
    mocks.identity.mockResolvedValue(commerce({
      plans: [
        { id: "plan-1", title: "合成プラン1", authorized: true },
        { id: "plan-2", title: "合成プラン2", authorized: false },
        { id: "plan-3", title: "合成プラン3", authorized: false },
      ],
    }));
    await renderSection();
    const bulk = bulkForm();
    expect(bulk).not.toBeUndefined();
    expect(bulk.textContent).toContain("合成プラン1");
    expect(bulk.textContent).toContain("合成プラン2");
    expect(bulk.textContent).toContain("合成プラン3");
    expect(bulk.textContent).toContain("上のプランすべてに、登録した氏名・住所と表示用電話番号を公開する");
    const check = bulk.querySelector('input[type="checkbox"]') as HTMLInputElement;
    const save = Array.from(bulk.querySelectorAll("button")).find((b) => b.textContent === "一括で公開設定を保存する") as HTMLButtonElement;
    expect(save.disabled).toBe(true);
    await act(async () => { check.click(); });
    expect(save.disabled).toBe(false);
  });

  it("表記不要のときは一括ブロックの確認文言が表記なしになる", async () => {
    mocks.identity.mockResolvedValue(commerce({
      required: false,
      plans: [
        { id: "plan-1", title: "合成プラン1", authorized: false },
        { id: "plan-2", title: "合成プラン2", authorized: false },
      ],
    }));
    await renderSection();
    expect(bulkForm().textContent).toContain("上のプランすべてを、身元情報の表記なしで公開する");
  });

  it("一括保存で confirm と現在の identity_revision が送られ、再取得後に全件「設定済み」になる", async () => {
    const before = commerce({
      plans: [
        { id: "plan-1", title: "合成プラン1", authorized: false },
        { id: "plan-2", title: "合成プラン2", authorized: false },
      ],
    });
    const after = commerce({
      plans: [
        { id: "plan-1", title: "合成プラン1", authorized: true },
        { id: "plan-2", title: "合成プラン2", authorized: true },
      ],
    });
    mocks.identity
      .mockResolvedValueOnce(before)
      .mockResolvedValueOnce({})
      .mockResolvedValue(after);
    await renderSection();
    const bulk = bulkForm();
    const check = bulk.querySelector('input[type="checkbox"]') as HTMLInputElement;
    const save = Array.from(bulk.querySelectorAll("button")).find((b) => b.textContent === "一括で公開設定を保存する") as HTMLButtonElement;
    await act(async () => { check.click(); });
    await act(async () => { save.click(); });
    await flush();
    expect(mocks.identity).toHaveBeenCalledWith("/v1/coach/disclosures", "vault-token", "PUT", { confirm: true, identity_revision: "rev-1" });
    expect(container.textContent).not.toContain("未承認のプランを一括で承認する");
    expect(container.textContent).toContain("完了：公開設定まで終わっています");
  });

  it("未承認が1件以下では一括ブロックが出ず、個別の承認・取り消しが従来どおり動く", async () => {
    const current = commerce({
      plans: [
        { id: "plan-1", title: "合成プラン1", authorized: true },
        { id: "plan-2", title: "合成プラン2", authorized: false },
      ],
    });
    mocks.identity.mockResolvedValue(current);
    await renderSection();
    expect(container.textContent).not.toContain("未承認のプランを一括で承認する");
    // 個別の承認：未承認プランを選んで確認して保存する。
    const planForm = Array.from(container.querySelectorAll("form")).find((f) => f.textContent?.includes("表示を設定するプラン")) as HTMLFormElement;
    const select = planForm.querySelector("select") as HTMLSelectElement;
    await act(async () => {
      select.value = "plan-2";
      select.dispatchEvent(new Event("change", { bubbles: true }));
    });
    const check = planForm.querySelector('input[type="checkbox"]') as HTMLInputElement;
    await act(async () => { check.click(); });
    const save = Array.from(planForm.querySelectorAll("button")).find((b) => b.textContent === "公開設定を保存する") as HTMLButtonElement;
    expect(save.disabled).toBe(false);
    await act(async () => { save.click(); });
    expect(mocks.identity).toHaveBeenCalledWith("/v1/coach/disclosures/plan-2", "vault-token", "PUT", { confirm: true, identity_revision: "rev-1" });
    // 個別の取り消し：承認済みプランを選ぶと取り消しが出る。
    await act(async () => {
      select.value = "plan-1";
      select.dispatchEvent(new Event("change", { bubbles: true }));
    });
    const revoke = Array.from(planForm.querySelectorAll("button")).find((b) => b.textContent === "公開を取り消す") as HTMLButtonElement;
    expect(revoke).not.toBeUndefined();
    await act(async () => { revoke.click(); });
    expect(mocks.identity).toHaveBeenCalledWith("/v1/coach/disclosures/plan-1", "vault-token", "DELETE", undefined);
  });

  it("身元保存や要否切替のあとの再取得で、承認が外れた状態がステップ3に反映される", async () => {
    const before = commerce({ plans: [{ id: "plan-1", title: "合成プラン", authorized: true }] });
    const after = commerce({ plans: [{ id: "plan-1", title: "合成プラン", authorized: false }] });
    mocks.identity
      .mockResolvedValueOnce(before)
      .mockResolvedValueOnce({})
      .mockResolvedValue(after);
    await renderSection();
    expect(container.textContent).toContain("完了：公開設定まで終わっています");
    await act(async () => {
      container.querySelectorAll("form")[0].dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    });
    expect(mocks.identity).toHaveBeenCalledWith("/v1/coach/identity", "vault-token", "PUT", before.identity);
    expect(container.textContent).toContain("あと1件");
    expect(container.textContent).toContain("次にやること：未承認のプランの公開設定をしてください");
  });
});
