import { expect, it } from "vitest";
import { installEnglish, setActiveLocale } from "./i18n";
import { en } from "./i18n-en";
import { ticketPackConfirmRows } from "./purchase-confirm";

it("distinguishes a returned reservation from a monetary refund in English", () => {
  installEnglish(en);
  setActiveLocale("en");
  try {
    const refund = ticketPackConfirmRows({ quantity: 6, amountYen: 2500, validityMonths: 6 })
      .find((row) => row.label === "Refunds")?.value;
    expect(refund).toContain("the one reserved ticket is returned");
    expect(refund).toContain("you receive a refund of ¥500 per ticket even after expiry");
    expect(refund).not.toContain("returned or refunded");
  } finally {
    setActiveLocale("ja");
  }
});
