"use client";
import { L } from "@/lib/i18n";

import { notFound } from "next/navigation";
import { use } from "react";
import { ComposerPage } from "@/components/composer-page";
import { InjuryForm } from "@/components/injury-form";
import { injuryBodyPartLabel, isInjuryBodyPart } from "@/lib/injury";

export default function InjuryGroupPostPage({ params }: { params: Promise<{ part: string }> }) {
  const { part } = use(params);
  if (!isInjuryBodyPart(part)) notFound();
  return (
    <ComposerPage title={L("{v0}のグループに投稿", { v0: injuryBodyPartLabel(part) })}>
      <InjuryForm groupPart={part} />
    </ComposerPage>
  );
}
