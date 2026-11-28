import type { Metadata } from "next";
import Link from "next/link";
import { Rule } from "@/components/legal-prose";
import { supportEmail } from "@/lib/operator";

// 海外コーチ向けの英語の紹介ページ（本体 #538）。募集 DM B の冒頭に URL を入れる受け皿。ログインなしで読める。
// 本文は運営者確定の最終原稿そのまま。金額・条件は DM 最終版と /legal/coach-terms に揃えてあるので、
// 変えるときは運営者の承認を経て、Coach Terms と食い違わないようにすること。
// 英語固定の静的ページ（表示言語設定の対象にしない）。招待制の募集なので検索結果には出さない。
const title = "Coach Japanese sprinters online — video reviews and premium assessments";
const description =
  "TrackTalk is a Japanese service for sprinters — from high school athletes to serious amateurs — who want a coach's eye on their sprinting.";

export const metadata: Metadata = {
  title: { absolute: `${title} | TrackTalk` },
  description,
  robots: { index: false },
  openGraph: { title, description, locale: "en_US" },
};

const link = "text-blue-300 underline";

export default function ForCoachesPage() {
  return (
    <div lang="en" className="mx-auto max-w-2xl space-y-6 [overflow-wrap:anywhere]">
      <div className="space-y-3">
        <h1 className="text-2xl font-bold">{title}</h1>
        <p className="text-sm leading-7 text-slate-300">
          TrackTalk is a Japanese service for sprinters — from high school athletes to serious amateurs — who want a coach&apos;s eye on their sprinting. Most of our athletes already train with a school or club coach; what they get from you is an expert second opinion from outside their usual environment, in two formats — a single video review, or a premium assessment across up to 6 videos.
        </p>
        <p className="text-sm leading-7 text-slate-300">We&apos;re inviting a small number of sprint coaches from overseas to join us.</p>
      </div>

      <Rule title="Two ways athletes work with you">
        <p>
          <strong>1. Single video review (the entry point)</strong>
          <br />
          One video, one complete written review in a fixed format: what you see phase by phase, the top fixes in priority order, and drills to work on. A single review has no paid follow-up add-on; an athlete who wants a deeper look across several videos books the premium package.
        </p>
        <p>
          <strong>2. Premium package (the deeper option)</strong>
          <br />
          The athlete sends up to 6 videos at once, grouped by theme (for example race, start or a practice drill — practice videos alone are fine) and with a note on what they are focusing on in each group. You watch all of them and write freely for each group, citing the group by name, then finish with one overall summary — not a separate review of each video.
        </p>
        <p>TrackTalk sells and supports both offers. Your part is the coaching itself — athletes never contact you directly, and we handle all communication.</p>
      </Rule>

      <Rule title="No Japanese needed">
        <p>You write in English. Translation is handled by AI on our side, in both directions, and the original text is always shown alongside the translation — for you and for the athlete.</p>
      </Rule>

      <Rule title="Pay">
        <ul className="list-disc space-y-2 pl-5">
          <li>
            <strong>Reviews: athletes pay $65 (¥10,000) per review — you receive $50 of that.</strong>
          </li>
          <li>
            <strong>Premium: athletes pay $127 (¥20,000) per package — you receive $100 of that.</strong>
          </li>
          <li>These are starting figures — we want to agree the final rates with you directly, not just set them. For reference, in Japan a single review costs the athlete ¥1,000 and the coach receives ¥700; the figures for overseas coaches are set higher.</li>
          <li>
            Paid monthly via <strong>Wise</strong>, in USD: month-end close, paid by the 15th of the following month (during the pilot we aim to send it around the 7th). Both fees are set in US dollars, so the amount you receive does not move with the exchange rate. There is no minimum payout — everything you earn is paid out each month, and we cover the Wise transfer fees.
          </li>
        </ul>
      </Rule>

      <Rule title="Paid pilot">
        <ul className="list-disc space-y-2 pl-5">
          <li>We start small with a paid pilot, run manually with a few athletes.</li>
          <li>You can decline any request.</li>
          <li>Pilot coaches are first in line when the full service opens.</li>
        </ul>
      </Rule>

      <Rule title="What we handle">
        <ul className="list-disc space-y-2 pl-5">
          <li>Translation in both directions (Japanese ⇄ English)</li>
          <li>Sales, payments, and all administration</li>
          <li>All athlete communication and support</li>
        </ul>
      </Rule>

      <Rule title="What we never do">
        <ul className="list-disc space-y-2 pl-5">
          <li>Ask for exclusivity — you keep all your own coaching work</li>
          <li>Share your contact details with athletes</li>
        </ul>
      </Rule>

      <Rule title="The terms">
        <p>
          The full terms for overseas coaches are here:{" "}
          <Link href="/legal/coach-terms" className={link}>Terms of Engagement</Link> — and how we handle your personal data is here:{" "}
          <Link href="/legal/coach-privacy" className={link}>Privacy Notice for Overseas Coaches</Link>.
        </p>
      </Rule>

      <Rule title="Interested?">
        <p>
          Reply to the message that brought you here, or write to us at{" "}
          <a href={`mailto:${supportEmail}`} className={link}>
            <strong>{supportEmail}</strong>
          </a>
          . Happy to answer questions first — no commitment needed to start the conversation.
        </p>
        <p>— TrackTalk (operated by Hinata, Japan)</p>
      </Rule>
    </div>
  );
}
