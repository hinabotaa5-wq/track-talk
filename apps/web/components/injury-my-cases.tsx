"use client";

import { useEffect, useState } from "react";
import { apiFetch, ApiRequestError } from "@/lib/api";
import type { InjuryMyCase } from "@/lib/injury-cases";

// 「データ」タブに送った自分の回答。本人だけに見え、ここから消せる。回答がなければ何も出さない。
export function InjuryMyCases({ token }: { token: string }) {
  const [cases, setCases] = useState<InjuryMyCase[]>([]);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  useEffect(() => {
    let active = true;
    apiFetch<{ cases: InjuryMyCase[] }>("/v1/injury/cases/mine", { token })
      .then((d) => {
        if (active) setCases(d.cases ?? []);
      })
      .catch(() => {});
    return () => {
      active = false;
    };
  }, [token]);

  async function remove(id: string) {
    if (!confirm("この回答を削除しますか？集計からも外れます。")) return;
    setDeletingId(id);
    try {
      await apiFetch(`/v1/injury/cases/${id}`, { method: "DELETE", token });
      setCases((current) => current.filter((item) => item.id !== id));
    } catch (e) {
      alert(e instanceof ApiRequestError ? e.message : "削除できませんでした");
    }
    setDeletingId(null);
  }

  if (cases.length === 0) return null;

  return (
    <section className="space-y-3">
      <h2 className="text-lg font-bold">「データ」に答えた内容</h2>
      <ul className="space-y-3">
        {cases.map((item) => (
          <li key={item.id} className="flex items-start gap-3 rounded-xl border border-white/20 glass bg-white/10 p-4 text-sm">
            <div className="min-w-0 flex-1 space-y-1">
              <p className="font-semibold text-slate-100">
                {item.body_part_label}（{item.timing_label}）
              </p>
              <p className="text-slate-200">状況: {item.situations.join("、")}</p>
              <p className="text-slate-200">どうすればよかったか: {item.preventions.join("、")}</p>
            </div>
            <button
              type="button"
              onClick={() => remove(item.id)}
              disabled={deletingId !== null}
              className="shrink-0 rounded-lg border border-red-400/40 px-3 py-1.5 font-semibold text-red-300 hover:bg-red-950/40 disabled:opacity-50"
            >
              削除
            </button>
          </li>
        ))}
      </ul>
    </section>
  );
}
