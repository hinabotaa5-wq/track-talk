/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot } from "react-dom/client";
import { expect, it, vi } from "vitest";
import GuardianPage from "./page";

vi.mock("next/navigation", () => ({ useParams: () => ({ token: "consent-token" }) }));
vi.mock("@/lib/api", () => ({
  apiFetch: vi.fn().mockResolvedValue({ status: "pending", can_consent: true, allow_publish: false }),
}));

it("shows the proposal start date and inclusive 30-day period in the guardian consent notice", async () => {
  const container = document.createElement("div");
  const root = createRoot(container);
  try {
    await act(async () => root.render(<GuardianPage />));

    const text = container.textContent;
    expect(text).toContain("コーチの提案に記載された開始日から始まります");
    expect(text).toContain("開始日と終了日は契約に記録され、期間は開始日と終了日を含む30日間です");
    expect(text).not.toContain("お支払いが済んだ時から期間が始まります");
    expect(text).toContain("同意後に保護者の方がクレジットカードでお支払い");
    const planConsent = container.querySelector("section ol li:first-child")?.textContent;
    expect(planConsent).toContain("コーチによる1ヶ月間の継続サポート）を利用することに同意します");
    expect(planConsent).toContain("質問や動画のやりとりは、プランの料金に含まれます");
    expect(planConsent).toContain("別料金の追加相談はありません");
    expect(planConsent).not.toContain("と、その追加相談");
    expect(container.querySelector('button[type="submit"]')?.textContent).toBe("同意する");
  } finally {
    await act(async () => root.unmount());
  }
});
