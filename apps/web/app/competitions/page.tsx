import Link from "next/link";
import { apiFetch } from "@/lib/api";
import { formatDate, scopeLabel } from "@/lib/format";
import { EmptyState, ErrorState } from "@/components/empty-state";
import type { Competition, Meta } from "@/lib/types";

export const dynamic = "force-dynamic";

export default async function CompetitionsPage() {
  let items: Competition[] = [];
  let error = "";
  try {
    const data = await apiFetch<{ competitions: Competition[]; meta: Meta }>(
      "/v1/competitions?per_page=50",
    );
    items = data.competitions ?? [];
  } catch {
    error = "大会一覧を取得できませんでした。";
  }

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">大会</h1>
      <p className="text-sm text-zinc-600">
        初期は主要な国内・国際大会を手動登録しています。公式記録の転載ではありません。
      </p>
      {error ? <ErrorState message={error} /> : null}
      {items.length === 0 && !error ? (
        <EmptyState title="大会がありません" body="準備中です。" />
      ) : (
        <div className="space-y-3">
          {items.map((c) => (
            <Link
              key={c.id}
              href={`/competitions/${c.id}`}
              className="block rounded-xl border bg-white p-4 hover:border-orange-300"
            >
              <p className="text-xs text-zinc-500">{scopeLabel(c.scope)}</p>
              <h2 className="text-lg font-semibold">{c.name}</h2>
              <p className="text-sm text-zinc-600">
                {formatDate(c.starts_on)}
                {c.ends_on ? ` 〜 ${formatDate(c.ends_on)}` : ""} · {c.location}
              </p>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
