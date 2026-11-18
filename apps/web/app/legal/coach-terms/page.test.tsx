/** @vitest-environment jsdom */
import { renderToStaticMarkup } from "react-dom/server";
import { expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { showsBackButton } from "@/components/bottom-nav";
import { operatorName } from "@/lib/operator";
import CoachTermsPage, { coachTermsVersion } from "./page";
vi.mock("next/link", () => ({ default: ({ href, children, className }: { href: string; children: ReactNode; className?: string }) => <a href={href} className={className}>{children}</a> }));

it("海外コーチの業務委託条件：決定済みの金額・支払・相殺・為替・期間を英語で示し、個別事項は work order に回す", () => {
  const html = renderToStaticMarkup(<CoachTermsPage />);
  expect(html).toContain('lang="en"');
  expect(html).toContain("¥16,500 per athlete per month");
  expect(html).toContain("75% of the tax-inclusive athlete price of ¥22,000");
  expect(html).toContain("$10 per completed review");
  expect(html).not.toContain("$12 per completed review");
  expect(html).toContain("with no set cap on the number of videos per athlete per month, replies targeted within about 36 hours");
  expect(html).not.toContain("up to about 28 videos");
  expect(html).toContain("15th of the following month");
  expect(html).toContain("TrackTalk bears the Wise transfer fees");
  expect(html).toContain("±15% band around ¥158.5");
  expect(html).toContain("Amounts the Coach disputes are excluded from offset");
  expect(html).toContain("absorbed by TrackTalk");
  expect(html).toContain("no payment is conditional on a form the Coach cannot truthfully complete");
  expect(html).toContain("the date the order was agreed, the services, the service period or delivery deadline, the fee, and the payment due date");
  expect(html).toContain(`These Terms are issued by <span lang="ja">${operatorName}</span>, the operator of TrackTalk`);
  expect(html).toContain('href="/legal/coach-privacy"');
  expect(html).toContain("11. Athlete data.");
  expect(html).toContain("solely to perform the Services");
  expect(html).toContain("without undue delay");
  expect(html).toContain("12. Responsibility and liability between the parties.");
  expect(html).toContain("Clause 12(b) does not affect clause 5");
  expect(html).toContain("13. Governing law and jurisdiction.");
  expect(html).toContain("Tokyo District Court shall have exclusive jurisdiction");
  expect(html.replace(/<[^>]*>/g, "").replace(operatorName, "")).not.toMatch(/\[|placeholder|TODO|[ぁ-んァ-ン一-龯]/);
  expect(showsBackButton("/legal/coach-terms")).toBe(true);
});

it("発注サービスだけに規約を適用し、標準額より個別の書面合意を優先する", () => {
  const container = document.createElement("div");
  container.innerHTML = renderToStaticMarkup(<CoachTermsPage />);
  const text = container.textContent ?? "";
  expect(text).toContain("The Coach provides only the services confirmed in an individual work order. Paragraphs (a) and (b) describe the services available under these Terms; neither paragraph obliges the Coach to provide, or TrackTalk to order, the other type of service, and a service type not stated in a work order is not part of that engagement.");
  expect(text).toContain("The amounts in (a) and (b) are the standard fees. If a different fee is agreed with the Coach individually in writing (including by direct message) or is stated in an individual work order, that fee applies instead of the standard fee for that service. Fees may be revised only by mutual agreement, and any revision applies only to services ordered after the revision is agreed (for a package, to future months).");
  expect(text).not.toContain("may be revised only for future months by mutual agreement");
  expect(text).toContain("and a training menu created at the start of the month and revised weekly");
  expect(text).toContain("single video reviews in TrackTalk's fixed format, targeted within about 24 hours of receipt");
  expect(text).toContain("The fee is denominated in Japanese yen and converted to USD at the Wise transfer rate on the payment date.");
});

it("実施州、為替提案の効力、単発を含む終了と相殺先を確定文面で表示する", () => {
  const container = document.createElement("div");
  container.innerHTML = renderToStaticMarkup(<CoachTermsPage />);
  const text = container.textContent ?? "";
  expect(text).toContain("The Coach performs all services online from the state in the United States stated in the individual work order.");
  expect(text).not.toContain("The Coach performs all services online from the United States.");
  expect(text).toContain("TrackTalk receives the services and deliverables in the designated workspace for each assignment within the TrackTalk app. Any in-person services in Japan require prior written agreement, as they may change the tax treatment of the related fees.");
  expect(text).toContain("If the USD/JPY rate moves outside a ±15% band around ¥158.5, TrackTalk may propose a fee revision effective from the following month. No revision applies retroactively. A proposal under this clause does not by itself change any fee, and no agreed revision applies to a service already ordered.");
  expect(text).toContain("Either side may end this engagement at any time, with effect for future work orders. An engagement already confirmed in a work order — for a package, a month already started; for a single review, a review already ordered — is completed and paid as above, unless the work order states otherwise.");
  expect(text).not.toContain("for future months; months already started");
  expect(text).toContain("The amount so determined — capped at the Coach's fee attributable to the undelivered services — may be offset against fees later payable to the Coach (including fees for later single reviews); any excess carries forward as a negative balance on the same basis.");
  expect(text).not.toContain("next month's fees");
});

it("export と表示の版を更新する", () => {
  expect(coachTermsVersion).toBe("2026-11-18");
  const html = renderToStaticMarkup(<CoachTermsPage />);
  expect(html).toContain("Version: 2026-11-18");
});
