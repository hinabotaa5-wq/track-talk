import type { Metadata } from "next";
import Link from "next/link";
import { Rule } from "@/components/legal-prose";

// 海外コーチ向けのプライバシー通知（英語、#531）。日本語のプライバシーポリシーのうち、コーチ自身の情報に関わる部分の抜粋。
// 内容を変えるときは、日本語のプライバシーポリシー（components/privacy-policy.tsx）と食い違わないようにし、運営者の承認を経ること。
export const metadata: Metadata = {
  title: "Privacy Notice for Overseas Coaches",
  description: "How TrackTalk handles the personal data of coaches who live outside Japan.",
  robots: { index: false },
};

const updated = "November 11, 2026";

export default function CoachPrivacyPage() {
  return (
    <div lang="en" className="mx-auto max-w-2xl space-y-6 [overflow-wrap:anywhere]">
      <div className="space-y-2">
        <h1 className="text-2xl font-bold">Privacy Notice for Overseas Coaches</h1>
        <p className="text-sm leading-7 text-slate-300">
          This notice explains how TrackTalk handles the personal data of coaches who live outside Japan (&quot;overseas coaches&quot;). It is a summary of the parts of our{" "}
          <Link href="/privacy" className="text-blue-300 underline">Privacy Policy (Japanese)</Link> that concern you as a coach. If this notice and the Japanese Privacy Policy differ, the Japanese Privacy Policy prevails.
        </p>
        <p className="text-xs text-slate-300">Last updated: {updated}</p>
      </div>

      <Rule title="1. Who we are">
        <p>
          TrackTalk is run by an individual operator in Japan (&quot;TrackTalk&quot;, &quot;we&quot;). We handle personal data under the Act on the Protection of Personal Information of Japan. You can contact us through the{" "}
          <Link href="/contact" className="text-blue-300 underline">contact page</Link>.
        </p>
      </Rule>

      <Rule title="2. Data we collect about you">
        <ul className="list-disc space-y-2 pl-5">
          <li>Account: your email address. If you sign in with Google, the identifier and email address needed to sign you in.</li>
          <li>Application: the name you coach under, your events, records, achievements and coaching experience, 1 to 3 links to your social media or team website, your profile photo, and the status of your application.</li>
          <li>Coaching: your plans and the messages, videos and training menus you exchange with athletes, together with their translations.</li>
          <li>Payment: the recipient details that Wise needs to send your fees, and the records of your fees, any offsets and the amounts sent.</li>
          <li>Technical data: the cookie that keeps you signed in, and the notification address of a device where you turned on notifications.</li>
        </ul>
      </Rule>

      <Rule title="3. Why we use it">
        <ul className="list-disc space-y-2 pl-5">
          <li>To review your application and contact you about it.</li>
          <li>To show your coach profile to athletes and let them request your coaching.</li>
          <li>To deliver coaching between you and athletes, including translating messages between Japanese and English.</li>
          <li>To pay your fees by Wise transfer and keep the records we need for payments, refunds and disputes.</li>
          <li>To prevent misuse of the service, handle reports, and respond to your requests.</li>
          <li>To improve translation quality, as explained in section 5.</li>
        </ul>
      </Rule>

      <Rule title="4. What athletes see">
        <p>
          Athletes can see the name you coach under, your events, records, achievements and coaching experience, your profile photo, and your social media or website links. Your email address and payment details are not shown to athletes.
        </p>
      </Rule>

      <Rule title="5. AI translation">
        <p>
          Messages between you and athletes are translated by AI, and the original text is shown next to the translation. We use Anthropic (Claude API, United States) for translation and send the text of the messages to it for that purpose.
        </p>
        <p>Anthropic does not use the text we send to train its AI models.</p>
        <p>
          Separately, TrackTalk uses source texts, translations, our corrected translations and candidate terms to improve our translation glossary and instructions. Before doing so, we remove names and other information that could identify a person, and we never use information about injuries or physical condition. We select and record these texts by hand and do not collect conversations automatically.
        </p>
      </Rule>

      <Rule title="6. Service providers">
        <p>We entrust the handling of personal data to the following providers, only as far as needed for the purposes above. The country of each company and the countries where data is stored or processed are shown in brackets.</p>
        <ul className="list-disc space-y-2 pl-5">
          <li>Supabase (United States; stored in Japan): sign-in and database.</li>
          <li>Cloudinary (United States; stored in the United States, delivered through a CDN): photos and videos.</li>
          <li>Netlify (United States; CDN): hosting of the web app.</li>
          <li>Google (United States; API processed in Japan): API hosting on Google Cloud and Google sign-in.</li>
          <li>Anthropic (United States): translation of messages between you and athletes.</li>
          <li>Wise (United Kingdom and other countries): sending your fees.</li>
          <li>Google, Mozilla, Apple and Microsoft (United States): delivering notifications to devices where you turned them on.</li>
        </ul>
        <p>Translation and Wise transfers are not used until the overseas coach program starts.</p>
      </Rule>

      <Rule title="7. How long we keep it">
        <p>
          Messages, videos and menus of a support plan become unavailable to both you and the athlete when the plan ends. Videos are deleted 30 days after the plan ends, and messages and menus are deleted 1 year after the end of the coaching period. Videos for video reviews are deleted 7 days after the review exchange ends. Records needed for payments, refunds and disputes are kept until those purposes are fulfilled. Items under an open report or dispute are kept until we finish handling it.
        </p>
      </Rule>

      <Rule title="8. Your rights">
        <p>
          You may ask us to disclose, correct, stop using or delete your personal data, or to disclose the records of its provision to third parties. Send your request through the{" "}
          <Link href="/contact" className="text-blue-300 underline">contact page</Link>. We will confirm your identity and respond as required by Japanese law.
        </p>
      </Rule>
    </div>
  );
}
