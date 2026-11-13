/** @vitest-environment jsdom */
import { renderToStaticMarkup } from "react-dom/server";
import { expect, it } from "vitest";
import { WorkOrderTermsView, WorkOrderView } from "./work-order-terms";
import type { WorkOrderTerms } from "@/lib/types";

const terms: WorkOrderTerms = {
  items: [{ label: "報酬", value: "¥350" }, { label: "支払", value: "月末締・翌月15日" }],
  amount_yen: 350, deliver_by: "2026-11-02T03:00:00Z", payment_due: "2026-12-15T00:00:00+09:00", consigned_at: "2026-11-01T03:00:00Z",
};

it("引き受ける前は、折りたたみで条件を見せ、引き受けると記録されることを伝える", () => {
  const html = renderToStaticMarkup(<WorkOrderTermsView terms={terms} offered />);
  expect(html).toContain("¥350");
  expect(html).toContain("取引条件");
  expect(html).toContain("<details");
  expect(html).not.toMatch(/<details[^>]* open/);
  expect(html).toContain("「引き受ける」を押すと");
});

it("引き受けた後は、折りたたみで書き留めた通知を見せ、受諾案内は出さない", () => {
  const html = renderToStaticMarkup(<WorkOrderView order={{ id: "w", terms_version: 5, terms, created_at: "2026-11-01T03:00:00Z" }} />);
  expect(html).toContain("¥350");
  expect(html).not.toMatch(/<details[^>]* open/);
  expect(html).not.toContain("「引き受ける」を押すと");
});

it("配信記録は表示に出さず、知らせから開く場所(work-order)を持つ", () => {
  const html = renderToStaticMarkup(
    <WorkOrderView order={{ id: "w", terms_version: 5, terms, created_at: "2026-11-01T03:00:00Z", delivered_at: "2026-11-01T03:00:00Z" }} />,
  );
  expect(html).toContain('id="work-order"');
  expect(html).not.toContain("にお知らせで届けました");
});
