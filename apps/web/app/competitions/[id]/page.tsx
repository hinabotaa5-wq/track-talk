import { apiFetch, ApiRequestError } from "@/lib/api";
import { formatDate, scopeLabel } from "@/lib/format";
import { ErrorState } from "@/components/empty-state";
import type { Competition } from "@/lib/types";
import { notFound } from "next/navigation";

export const dynamic = "force-dynamic";

export default async function CompetitionDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  let comp: Competition | null = null;
  let failed = false;
  try {
    comp = await apiFetch<Competition>(`/v1/competitions/${id}`);
  } catch (e) {
    if (e instanceof ApiRequestError && e.status === 404) notFound();
    failed = true;
  }
  if (failed || !comp) {
    return <ErrorState message="大会情報を取得できませんでした。" />;
  }
  return (
    <div className="space-y-6">
      <p className="text-xs text-zinc-500">{scopeLabel(comp.scope)}</p>
      <h1 className="text-3xl font-bold">{comp.name}</h1>
      <p className="text-sm text-zinc-600">
        {formatDate(comp.starts_on)}
        {comp.ends_on ? ` 〜 ${formatDate(comp.ends_on)}` : ""} · {comp.location}
      </p>
      <p className="whitespace-pre-wrap text-zinc-700">{comp.summary}</p>
    </div>
  );
}
