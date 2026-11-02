"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { X } from "lucide-react";
import { apiFetch } from "@/lib/api";
import type { InjuryTally } from "@/lib/injury";

export type InjuryPrevention = {
  event: string;
  event_label: string;
  min_people: number;
  body_parts: InjuryTally;
  pre_changes: InjuryTally;
  hindsight: InjuryTally;
};

// 閉じたら7日は出さない。覚えておくのはこの端末だけ。
const dismissKey = "tt:injury-prevention-dismissed-at";
const dismissDays = 7;

function dismissedRecently(now: number): boolean {
  try {
    const at = Number(window.localStorage.getItem(dismissKey));
    return Number.isFinite(at) && at > 0 && now - at < dismissDays * 24 * 60 * 60 * 1000;
  } catch {
    return false;
  }
}

function rememberDismiss(now: number) {
  try {
    window.localStorage.setItem(dismissKey, String(now));
  } catch {
    // 保存できない環境では、次に開いたときにまた出るだけ。
  }
}

function top(tally: InjuryTally) {
  return tally.items[0]?.label ?? null;
}

// preventionLines は、カードに出す行。5人に満たない項目は API が空で返すので、ここでは出さないだけ。
export function preventionLines(data: InjuryPrevention) {
  const lines: { label: string; value: string }[] = [];
  const part = top(data.body_parts);
  const change = top(data.pre_changes);
  const hindsight = top(data.hindsight);
  if (part) lines.push({ label: "多いケガ", value: part });
  if (change) lines.push({ label: "ケガの前に多かった変化", value: change });
  if (hindsight) lines.push({ label: "いま思うと", value: hindsight });
  return lines;
}

export function InjuryPreventionCardView({ data, onDismiss }: { data: InjuryPrevention; onDismiss?: () => void }) {
  const lines = preventionLines(data);
  if (lines.length === 0) return null;
  return (
    <section className="relative space-y-2 rounded-2xl border border-white/20 glass bg-white/10 p-4" aria-labelledby="injury-prevention-title">
      {onDismiss ? (
        <button
          type="button"
          onClick={onDismiss}
          aria-label="ケガ予防のカードを閉じる"
          className="absolute right-2 top-2 inline-flex min-h-11 min-w-11 items-center justify-center rounded-full text-slate-300 hover:text-white"
        >
          <X aria-hidden className="h-4 w-4" />
        </button>
      ) : null}
      <h2 id="injury-prevention-title" className="pr-10 text-base font-semibold text-slate-100">
        {data.event_label}の人のケガの傾向
      </h2>
      <dl className="space-y-1.5 text-sm">
        {lines.map((line) => (
          <div key={line.label} className="flex justify-between gap-3">
            <dt className="text-slate-300">{line.label}</dt>
            <dd className="text-right font-medium text-slate-100">{line.value}</dd>
          </div>
        ))}
      </dl>
      <p className="text-xs text-slate-300">
        ケガ共有のグループの回答を、名前を出さずに集計したものです。{data.min_people}人に満たない項目は出していません。
      </p>
      <Link href="/injury/data" className="inline-block text-sm font-medium text-sky-200 underline underline-offset-4">
        ケガのデータを見る
      </Link>
    </section>
  );
}

// 感覚メモに出す、自分の種目の人のケガの傾向。プロフィールの種目だけで決め、本人のケガの記録は使わない。
export function InjuryPreventionCard({ event }: { event: string | null | undefined }) {
  const [data, setData] = useState<InjuryPrevention | null>(null);
  const [hidden, setHidden] = useState(true);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setHidden(dismissedRecently(Date.now()));
  }, []);

  useEffect(() => {
    if (!event || hidden) return;
    const controller = new AbortController();
    apiFetch<InjuryPrevention>(`/v1/injury/prevention?event=${encodeURIComponent(event)}`, { signal: controller.signal })
      .then((next) => {
        if (!controller.signal.aborted) setData(next);
      })
      .catch(() => {
        // 予防のヒントは補足の情報なので、取得できなければ何も出さない。
      });
    return () => controller.abort();
  }, [event, hidden]);

  if (hidden) return null;
  if (!event) {
    return (
      <p className="rounded-xl border border-white/15 bg-white/5 p-3 text-sm text-slate-200">
        <Link href="/me/profile" className="font-medium text-sky-200 underline underline-offset-4">種目を登録</Link>
        すると、あなたの種目の人に多いケガの傾向がここに出ます。
      </p>
    );
  }
  if (!data || data.event !== event) return null;
  return (
    <InjuryPreventionCardView
      data={data}
      onDismiss={() => {
        rememberDismiss(Date.now());
        setHidden(true);
      }}
    />
  );
}
