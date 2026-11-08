/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot } from "react-dom/client";
import { expect, it, vi } from "vitest";
import { CoachEarningsSection } from "./coach-payouts";

const apiFetch = vi.hoisted(() => vi.fn());
vi.mock("@/lib/api", () => ({ apiFetch }));

async function renderEarnings(payouts_enabled: boolean, stripe_linked: boolean) {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  apiFetch.mockResolvedValue({
    payments: true,
    payouts_enabled,
    stripe_linked,
    this_month_yen: 0,
    unpaid_yen: 0,
    review_share: 70,
    personal_fee: 20,
    min_payout_yen: 3000,
    payouts: [],
    next_payout_date: null,
    next_payout_yen: null,
  });

  const container = document.createElement("div");
  document.body.appendChild(container);
  const root = createRoot(container);
  try {
    await act(async () => root.render(<CoachEarningsSection token="test" onReady={() => {}} />));
    return container.textContent ?? "";
  } finally {
    await act(async () => root.unmount());
    container.remove();
    vi.unstubAllGlobals();
  }
}

it.each([
  { payouts_enabled: false, stripe_linked: false, label: "受け取り口座: 未登録", button: "受け取り口座を登録する" },
  { payouts_enabled: false, stripe_linked: true, label: "受け取り口座: 登録の途中・確認中", button: "登録を続ける" },
  { payouts_enabled: true, stripe_linked: true, label: "受け取り口座: 登録済み", button: null },
])("shows the payout account status clearly ($label)", async ({ payouts_enabled, stripe_linked, label, button }) => {
  const text = await renderEarnings(payouts_enabled, stripe_linked);
  expect(text).toContain(label);
  if (button) {
    expect(text).toContain(button);
  } else {
    expect(text).not.toContain("受け取り口座を登録する");
    expect(text).not.toContain("登録を続ける");
  }
});

it.each([
  { date: "2026-12-15", yen: 1050 },
  { date: null, yen: null },
])("shows the approved payout copy only when a schedule is available ($date)", async ({ date, yen }) => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  apiFetch.mockResolvedValue({
    payments: true,
    payouts_enabled: true,
    this_month_yen: 0,
    unpaid_yen: 1050,
    review_share: 70,
    personal_fee: 20,
    min_payout_yen: 3000,
    payouts: [],
    next_payout_date: date,
    next_payout_yen: yen,
  });

  const container = document.createElement("div");
  const root = createRoot(container);
  try {
    await act(async () => root.render(<CoachEarningsSection token="test" onReady={() => {}} />));
    expect(container.textContent).toContain("動画添削の報酬は1本1,050円、添削後の追加相談は1件420円です");
    expect(container.textContent).toContain("サポートプランの受取額は、代金から手数料20%を差し引いた額です。");
    expect(container.textContent).toContain("翌月15日にStripeを通じて登録済みの銀行口座へ振り込みます。");
    expect(container.textContent).toContain("3,000円未満の場合は1回だけ繰り越し、翌々月15日に金額にかかわらず支払います。");
    expect(container.textContent).not.toContain("チケットのまとめ買い割引");
    if (date) {
      expect(container.textContent).toContain("次回のお支払い予定日：2026年12月15日");
      expect(container.textContent).toContain("お支払い予定額：1,050円");
      expect(container.textContent).toContain("金額と日付は予定です。返金の精算や口座の状態によって変わることがあります。");
    } else {
      expect(container.textContent).not.toContain("次回のお支払い予定日");
      expect(container.textContent).not.toContain("お支払い予定額");
    }
  } finally {
    await act(async () => root.unmount());
    vi.unstubAllGlobals();
  }
});
