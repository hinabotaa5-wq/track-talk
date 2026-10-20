"use client";

import { notFound, useRouter } from "next/navigation";
import { use, useEffect, useState } from "react";
import { ComposerPage } from "@/components/composer-page";
import { ErrorState } from "@/components/empty-state";
import { InjuryStageChange } from "@/components/injury-stage-change";
import { useAuth } from "@/components/auth-provider";
import { apiFetch } from "@/lib/api";
import { injuryBodyPartLabel, injuryGroupPath, isInjuryBodyPart, type InjuryGroup } from "@/lib/injury";

// 段階を変える専用のページ。変えたらグループのページに戻る(初めて大会に復帰したときは、戻った先で祝う)。
export default function InjuryStagePage({ params }: { params: Promise<{ part: string }> }) {
  const { part } = use(params);
  if (!isInjuryBodyPart(part)) notFound();
  const { token, loading } = useAuth();
  const router = useRouter();
  const [group, setGroup] = useState<InjuryGroup | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    if (loading || !token) return;
    const controller = new AbortController();
    apiFetch<InjuryGroup>(`/v1/injury/groups/${part}`, { token, signal: controller.signal })
      .then(setGroup)
      .catch(() => {
        if (!controller.signal.aborted) setFailed(true);
      });
    return () => controller.abort();
  }, [loading, token, part]);

  const back = injuryGroupPath(part);
  return (
    <ComposerPage title={`${injuryBodyPartLabel(part)}の段階を変える`}>
      {failed ? (
        <ErrorState message="グループの取得に失敗しました。グループに入っているか確認してください。" />
      ) : group ? (
        <InjuryStageChange
          part={part}
          token={token}
          member={group.me}
          onChanged={(_member, celebrate) => router.push(celebrate ? `${back}?celebrate=1` : back)}
          onCancel={() => router.push(back)}
        />
      ) : (
        <div className="h-11" aria-hidden />
      )}
    </ComposerPage>
  );
}
