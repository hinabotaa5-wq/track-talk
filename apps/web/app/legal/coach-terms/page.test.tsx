/** @vitest-environment jsdom */
import { renderToStaticMarkup } from "react-dom/server";
import { expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { showsBackButton } from "@/components/bottom-nav";
import { operatorName } from "@/lib/operator";
import CoachTermsPage from "./page";
vi.mock("next/link", () => ({ default: ({ href, children, className }: { href: string; children: ReactNode; className?: string }) => <a href={href} className={className}>{children}</a> }));

it("海外コーチの業務委託条件：決定済みの金額・支払・相殺・為替・期間を英語で示し、個別事項は work order に回す", () => {
  const html = renderToStaticMarkup(<CoachTermsPage />);
  expect(html).toContain('lang="en"');
  expect(html).toContain("¥16,500 per athlete per month");
  expect(html).toContain("75% of the tax-inclusive athlete price of ¥22,000");
  expect(html).toContain("$12 per completed review");
  expect(html).toContain("15th of the following month");
  expect(html).toContain("TrackTalk bears the Wise transfer fees");
  expect(html).toContain("±15% band around ¥158.5");
  expect(html).toContain("Amounts the Coach disputes are excluded from offset");
  expect(html).toContain("absorbed by TrackTalk");
  expect(html).toContain("no payment is conditional on a form the Coach cannot truthfully complete");
  expect(html).toContain("the date the order was agreed, the services, the service period or delivery deadline, the fee, and the payment due date");
  expect(html).toContain(`These Terms are issued by <span lang="ja">${operatorName}</span>, the operator of TrackTalk`);
  expect(html).toContain('href="/legal/coach-privacy"');
  expect(html.replace(/<[^>]*>/g, "").replace(operatorName, "")).not.toMatch(/\[|placeholder|TODO|[ぁ-んァ-ン一-龯]/);
  expect(showsBackButton("/legal/coach-terms")).toBe(true);
});
