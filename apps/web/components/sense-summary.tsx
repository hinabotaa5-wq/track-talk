"use client";

import { useEffect, useState } from "react";
import { Sparkles } from "lucide-react";
import { Button } from "./ui/button";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { useActionLock } from "@/lib/use-action-lock";
import { senseSummaryNextLabel, senseSummaryRangeLabel, type SenseSummary, type SenseSummaryResponse } from "@/lib/sense-notes";

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
export function SenseSummaryCard({ token }: { token: string | null }) {
  const [summary, setSummary] = useState<SenseSummary | null>(null);
  const [nextAt, setNextAt] = useState<string | null>(null);
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
        setLoaded(true);
      })
      .catch(() => {
        // まとめが読めなくても、感覚メモはそのまま使えるようにカードを出さない。
      });
    return () => controller.abort();
  }, [token]);

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
  return (
    <section className="space-y-3 rounded-2xl border border-white/20 glass bg-white/10 p-4">
      <div className="flex items-center justify-between gap-3">
        <h2 className="flex items-center gap-1.5 font-semibold">
          <Sparkles className="h-4 w-4 text-amber-200" aria-hidden />
          1週間のふりかえり
        </h2>
        {summary ? <span className="text-xs text-slate-300">{senseSummaryRangeLabel(summary)}</span> : null}
      </div>

      {summary ? (
        <div className="space-y-3">
          <p className="text-sm leading-relaxed text-slate-100">{summary.summary.overview}</p>
          <SummaryList title="つかめてきた感覚" items={summary.summary.good} />
          <SummaryList title="気になる感覚" items={summary.summary.concerns} />
          <SummaryList title="来週意識すること" items={summary.summary.next} />
          <p className="text-xs text-slate-400">{summary.note_count}個の感覚メモから AI がまとめました。内容はメモをもとにした目安です。</p>
        </div>
      ) : (
        <p className="text-sm text-slate-200">この1週間に書いた感覚メモを、AI がふりかえりの形にまとめます。</p>
      )}

      <div className="flex flex-wrap items-center gap-3">
        <Button onClick={create} disabled={busy || nextLabel !== null}>
          {busy ? "まとめています..." : summary ? "今週の分をまとめる" : "この1週間をまとめる"}
        </Button>
        <span className="text-xs text-slate-300">{nextLabel ? `次は${nextLabel}から作れます` : "まとめは1週間に1回です"}</span>
      </div>
      {error ? <p className="text-sm text-red-300">{error}</p> : null}
      <p className="text-xs text-slate-400">まとめるときだけ、直近7日の感覚メモを Google の Gemini に送ります。</p>
    </section>
  );
}
