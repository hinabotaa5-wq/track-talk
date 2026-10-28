/** @vitest-environment jsdom */
import { renderToStaticMarkup } from "react-dom/server";
import { expect, it } from "vitest";
import { WorkOrderTermsView, WorkOrderView } from "./work-order-terms";
import type { WorkOrderTerms } from "@/lib/types";

const terms: WorkOrderTerms = {
  items: [{ label: "報酬の額", value: "840円（税込）" }, { label: "支払期日", value: "2026年12月15日" }],
  amount_yen: 840, deliver_by: "2026-11-02T03:00:00Z", payment_due: "2026-12-15T00:00:00+09:00", carried_due: "2027-01-15T00:00:00+09:00", consigned_at: "2026-11-01T03:00:00Z",
};

it("引き受ける前は、条件を開いた状態で見せ、引き受けると記録されることを伝える", () => {
  const html = renderToStaticMarkup(<WorkOrderTermsView terms={terms} />);
  expect(html).toContain("840円（税込）");
  expect(html).toContain("引き受ける前に確認してください");
  expect(html).toContain("<details");
  expect(html).toMatch(/<details[^>]* open/);
});

it("引き受けた後は、書き留めた通知として畳んで見せる", () => {
  const html = renderToStaticMarkup(<WorkOrderView order={{ id: "w", terms_version: 1, terms, created_at: "2026-11-01T03:00:00Z" }} />);
  expect(html).toContain("に引き受け");
  expect(html).not.toMatch(/<details[^>]* open/);
});
