"use client";

import { useEffect, useState } from "react";
import { Sparkles } from "lucide-react";
import { Button } from "./ui/button";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { useActionLock } from "@/lib/use-action-lock";
import { senseSummaryMinNotes, senseSummaryNextLabel, senseSummaryRangeLabel, type SenseSummary, type SenseSummaryResponse } from "@/lib/sense-notes";

function SummaryList({ title, items }: { title: string; items: string[] }) {
  if (items.length === 0) return null;
  return (
    <div>
      <h3 className="text-xs font-semibold text-slate-300">{title}</h3>
      <ul className="mt-1 list-disc space-y-0.5 pl-5 text-sm text-slate-100">
        {items.map((item, i) => (
          <li key={i}>{item}</li>
        ))}
      </ul>
    </div>
  );
}

// 直近7日の感覚メモを AI でまとめるカード。まとめは本人だけに表示する。
// total は感覚メモの総数。メモが増えたら、直近7日の数を取り直す。
export function SenseSummaryCard({ token, total }: { token: string | null; total: number }) {
  const [summary, setSummary] = useState<SenseSummary | null>(null);
  const [nextAt, setNextAt] = useState<string | null>(null);
  const [weekNotes, setWeekNotes] = useState(0);
  const [minNotes, setMinNotes] = useState(senseSummaryMinNotes);
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState("");
  const { pending: busy, acquire, release } = useActionLock();

  useEffect(() => {
    if (!token) return;
    const controller = new AbortController();
    apiFetch<SenseSummaryResponse>("/v1/me/sense-notes/summary", { token, signal: controller.signal })
      .then((d) => {
        setSummary(d.summary);
        setNextAt(d.next_available_at);
        setWeekNotes(d.week_notes ?? 0);
        setMinNotes(d.min_notes ?? senseSummaryMinNotes);
        setLoaded(true);
      })
      .catch(() => {
        // まとめが読めなくても、感覚メモはそのまま使えるようにカードを出さない。
      });
    return () => controller.abort();
  }, [token, total]);

  async function create() {
    if (!acquire()) return;
    setError("");
    try {
      const d = await apiFetch<SenseSummaryResponse>("/v1/me/sense-notes/summary", { method: "POST", token });
      setSummary(d.summary);
      setNextAt(d.next_available_at);
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : "まとめを作れませんでした");
    } finally {
      release();
    }
  }

  if (!loaded) return null;

  const nextLabel = senseSummaryNextLabel(nextAt);
  const shortBy = Math.max(0, minNotes - weekNotes);
  const hasDetails = summary !== null && summary.summary.good.length + summary.summary.concerns.length + summary.summary.next.length > 0;
  // 書く欄とメモの一覧のあいだに置くので、縦に場所を取らないよう小さくまとめる。
  // ボタンは書く欄の「残す」と並ばないよう控えめな見た目にする。
  return (
    <section className="space-y-2 rounded-2xl border border-white/15 bg-white/6 px-4 py-3">
      <div className="flex items-center justify-between gap-3">
        <h2 className="flex min-w-0 items-center gap-1.5 text-sm font-semibold">
          <Sparkles className="h-4 w-4 shrink-0 text-amber-200" aria-hidden />
          1週間のふりかえり
          {summary ? <span className="truncate text-xs font-normal text-slate-300">{senseSummaryRangeLabel(summary)}</span> : null}
        </h2>
        {nextLabel ? (
          <span className="shrink-0 text-xs text-slate-300">次は{nextLabel}から</span>
        ) : shortBy > 0 ? (
          <span className="shrink-0 text-xs text-slate-300">あと{shortBy}個でまとめられます</span>
        ) : (
          <Button variant="secondary" className="shrink-0 px-3 py-1 text-xs" onClick={create} disabled={busy}>
            {busy ? "まとめています..." : "まとめる"}
          </Button>
        )}
      </div>

      {summary ? (
        <>
          <p className="text-sm leading-relaxed text-slate-100">{summary.summary.overview}</p>
          {hasDetails ? (
            <details>
              <summary className="cursor-pointer text-xs font-semibold text-slate-300 hover:text-white">くわしく見る</summary>
              <div className="mt-2 space-y-3">
                <SummaryList title="つかめてきた感覚" items={summary.summary.good} />
                <SummaryList title="気になる感覚" items={summary.summary.concerns} />
                <SummaryList title="来週意識すること" items={summary.summary.next} />
              </div>
            </details>
          ) : null}
          <p className="text-xs text-slate-400">{summary.note_count}個の感覚メモから AI がまとめた目安です。まとめは1週間に1回です。</p>
        </>
      ) : (
        <p className="text-xs text-slate-300">この1週間の感覚メモを AI がふりかえりにまとめます(1週間に1回)。</p>
      )}
      {error ? <p className="text-sm text-red-300">{error}</p> : null}
      <p className="text-xs text-slate-400">まとめるときだけ、直近7日の感覚メモを Google の Gemini に送ります。</p>
    </section>
  );
}
