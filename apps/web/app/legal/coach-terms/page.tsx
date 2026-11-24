import type { Metadata } from "next";
import Link from "next/link";
import { Rule } from "@/components/legal-prose";
import { operatorName } from "@/lib/operator";

// 海外コーチとの業務委託の条件（英語、#531 項目1）。フリーランス法3条の明示に対応する文書1（2026-11-11 運営者決定を反映した草案）を、
// 業務委託事業者の氏名は特商法表記と同じ operatorName をページ内に直接書く。コーチごと・依頼ごとの事項（コーチ名・委託日・期間・報酬額・支払期日）は個別の work order に回す形で、常設のページにしたもの。
// 条件を変えるときは運営者の承認を経ること。版（version）を変えたら work order が参照する版も合わせる。
export const metadata: Metadata = {
  title: "Terms of Engagement for Overseas Coaches",
  description: "The terms on which coaches who live outside Japan provide coaching services through TrackTalk.",
  robots: { index: false },
};

export const coachTermsVersion = "2026-11-25";

export default function CoachTermsPage() {
  return (
    <div lang="en" className="mx-auto max-w-2xl space-y-6 [overflow-wrap:anywhere]">
      <div className="space-y-2">
        <h1 className="text-2xl font-bold">Terms of Engagement — TrackTalk Coaching Services (Contractor)</h1>
        <p className="text-sm leading-7 text-slate-300">
          These Terms are issued by <span lang="ja">{operatorName}</span>, the operator of TrackTalk, an individual business in Japan (&quot;TrackTalk&quot;), to coaches who live outside Japan and provide services as independent contractors (&quot;Coach&quot;). The Coach&apos;s name, the date each order is agreed, and the details of each engagement are stated in the individual work order (clause 10).
        </p>
        <p className="text-xs text-slate-300">Version: {coachTermsVersion}</p>
      </div>

      <Rule title="1. Services">
        <p>
          The Coach provides, through the TrackTalk app, online via information and communications technology: (a) premium packages — one overall written assessment covering up to 10 training and race videos submitted by the athlete at once, without a separate review of each video; the delivery deadline for a premium package is confirmed in the individual work order; and (b) single video reviews in TrackTalk&apos;s fixed format, targeted within about 24 hours of receipt. All communication is asynchronous and in-app. Athletes never contact the Coach directly. The specific services, service period, and deadlines for each engagement are confirmed in an individual work order issued under these Terms (see clause 10). The Coach provides only the services confirmed in an individual work order. Paragraphs (a) and (b) describe the services available under these Terms; neither paragraph obliges the Coach to provide, or TrackTalk to order, the other type of service, and a service type not stated in a work order is not part of that engagement.
        </p>
      </Rule>

      <Rule title="2. Fees">
        <p>
          (a) Premium package: <strong>$100 per package</strong> (the tax-inclusive athlete price is ¥20,000). The fee is denominated in US dollars. (b) Single review: <strong>$50 per completed review</strong> (the tax-inclusive athlete price is ¥10,000). The amounts in (a) and (b) are the standard fees. If a different fee is agreed with the Coach individually in writing (including by direct message) or is stated in an individual work order, that fee applies instead of the standard fee for that service. Fees may be revised only by mutual agreement, and any revision applies only to services ordered after the revision is agreed (for a premium package, to packages ordered after the revision is agreed).
        </p>
      </Rule>

      <Rule title="2A. Place of performance and receipt">
        <p>
          The Coach performs all services online from the state in the United States stated in the individual work order. TrackTalk receives the services and deliverables in the designated workspace for each assignment within the TrackTalk app. Any in-person services in Japan require prior written agreement, as they may change the tax treatment of the related fees.
        </p>
      </Rule>

      <Rule title="3. Payment">
        <p>
          TrackTalk closes accounts on the last calendar day of each month (Japan Standard Time) and pays the fees for that month by the <strong>15th of the following month</strong> via <strong>Wise</strong> to the Coach&apos;s designated bank account. There is no minimum payout. TrackTalk bears the Wise transfer fees and any intermediary charges; if such a charge reduces the credited amount below the agreed fee after the stated currency conversion, TrackTalk will promptly pay the shortfall. The applicable payment date for each engagement is confirmed in its work order.
        </p>
      </Rule>

      <Rule title="4. FX review">
        <p>
          If the USD/JPY rate moves outside a ±15% band around ¥158.5, TrackTalk may propose a fee revision effective from the following month. No revision applies retroactively. A proposal under this clause does not by itself change any fee, and no agreed revision applies to a service already ordered.
        </p>
      </Rule>

      <Rule title="5. Refunds and offsets">
        <p>
          If a refund to an athlete results from the Coach&apos;s material failure to perform an express work-order obligation, TrackTalk will give the Coach a written explanation and a calculation of the amount attributable to the undelivered services, with prior notice before any offset. A response-time target alone does not establish a failure. The amount so determined — capped at the Coach&apos;s fee attributable to the undelivered services — may be offset against fees later payable to the Coach (including fees for later single reviews); any excess carries forward as a negative balance on the same basis. Amounts the Coach disputes are excluded from offset until the dispute is resolved. A balance remaining when the Coach&apos;s engagement ends is absorbed by TrackTalk.
        </p>
      </Rule>

      <Rule title="6. Tax documentation">
        <p>
          Each party is responsible for its own tax obligations. TrackTalk will request tax documentation through a secure channel only where needed to meet applicable reporting or withholding obligations; any Form W-9 request is limited to a Coach who is eligible to provide it, and no payment is conditional on a form the Coach cannot truthfully complete. TrackTalk issues an annual earnings summary each January (a payment summary, separate from any statutory tax form).
        </p>
      </Rule>

      <Rule title="7. Eligibility declaration">
        <p>The Coach declares that they are not a current student athlete and are legally able to receive coaching fees in the United States.</p>
      </Rule>

      <Rule title="8. No exclusivity">
        <p>The Coach may continue all of their own coaching and other work without restriction.</p>
      </Rule>

      <Rule title="9. Term and ending">
        <p>Either side may end this engagement at any time, with effect for future work orders. An engagement already confirmed in a work order — for a premium package, a package already ordered; for a single review, a review already ordered — is completed and paid as above, unless the work order states otherwise.</p>
      </Rule>

      <Rule title="10. Work orders and written delivery">
        <p>
          Each engagement is confirmed in an individual work order (by email or addressed in-app message) stating the date the order was agreed, the services, the service period or delivery deadline, the fee, and the payment due date, under the version of these Terms identified in the order. The Coach may request a paper copy of these Terms and of any work order through the{" "}
          <Link href="/contact" className="text-blue-300 underline">contact page</Link>, including after account closure, and TrackTalk will provide it without delay where required by applicable law.
        </p>
      </Rule>

      <Rule title="11. Athlete data.">
        <p>
          (a) TrackTalk provides the Coach with access to athlete data (submitted videos, messages, profile information, and any health-related information an athlete enters in the designated fields) <strong>solely to perform the Services</strong> under these Terms and the applicable work order.
        </p>
        <p>
          (b) The Coach must not use athlete data for any other purpose, and must not disclose, sell, or otherwise provide it to any third party. The Coach must not delegate or sub-contract any handling of athlete data to another person without TrackTalk&apos;s prior written consent.
        </p>
        <p>
          (c) The Coach must access athlete data only through the Coach&apos;s own TrackTalk account and only within the TrackTalk app. The Coach must not download, copy, or store athlete data outside the app, except for temporary copies that are technically unavoidable while performing the Services; any such copy must be deleted promptly once it is no longer needed, and in any event when the relevant engagement ends.
        </p>
        <p>
          (d) The Coach must take reasonable security measures for the devices and account the Coach uses (including keeping login credentials confidential and not sharing the account with any other person).
        </p>
        <p>
          (e) If the Coach becomes aware of any loss, leakage, unauthorised access, or other incident involving athlete data, the Coach must notify TrackTalk <strong>without undue delay</strong> after becoming aware of it, and must cooperate with TrackTalk&apos;s response.
        </p>
        <p>
          (f) When the Coach&apos;s engagement ends (or earlier, when TrackTalk so requests), the Coach must promptly stop all access to athlete data, delete any athlete data in the Coach&apos;s possession or control, and confirm the deletion to TrackTalk on request.
        </p>
        <p>
          (g) The Coach must cooperate with TrackTalk&apos;s periodic checks on how athlete data is handled, and must provide the information TrackTalk reasonably requests to confirm the measures in this clause and to respond to athlete requests for explanation.
        </p>
        <p>
          (h) This clause survives the end of the engagement.
        </p>
      </Rule>

      <Rule title="12. Responsibility and liability between the parties.">
        <p>
          (a) TrackTalk is the contracting party vis-à-vis athletes and will respond to athletes&apos; claims in the first instance. Nothing in these Terms limits any liability that TrackTalk or the Coach owes to an athlete, and no term of the athlete-facing terms will state that TrackTalk bears no responsibility whatsoever for the Services.
        </p>
        <p>
          (b) As between TrackTalk and the Coach, the Coach is not liable to TrackTalk for damages arising from an athlete&apos;s injury or other harm in connection with practising in accordance with the Coach&apos;s feedback, <strong>except</strong> to the extent the harm was caused by the Coach&apos;s wilful misconduct or gross negligence.
        </p>
        <p>
          (c) Clause 12(b) does not affect clause 5 (refunds and offsets), which concerns fee adjustments for services not performed, not damages for injury or other harm.
        </p>
      </Rule>

      <Rule title="13. Governing law and jurisdiction.">
        <p>
          These Terms, and each work order issued under them, are governed by the laws of Japan. The Tokyo District Court shall have exclusive jurisdiction as the court of first instance over any dispute arising out of or in connection with these Terms or any work order.
        </p>
      </Rule>

      <Rule title="Privacy">
        <p>
          How TrackTalk handles the Coach&apos;s personal data is explained in the{" "}
          <Link href="/legal/coach-privacy" className="text-blue-300 underline">Privacy Notice for Overseas Coaches</Link>.
        </p>
      </Rule>
    </div>
  );
}
