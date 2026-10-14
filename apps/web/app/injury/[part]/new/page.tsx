"use client";

import { notFound } from "next/navigation";
import { use } from "react";
import { ComposerPage } from "@/components/composer-page";
import { InjuryForm } from "@/components/injury-form";
import { injuryBodyPartLabel, isInjuryBodyPart } from "@/lib/injury";

export default function InjuryGroupPostPage({ params }: { params: Promise<{ part: string }> }) {
  const { part } = use(params);
  if (!isInjuryBodyPart(part)) notFound();
  return (
    <ComposerPage title={`${injuryBodyPartLabel(part)}のグループに投稿`}>
      <InjuryForm groupPart={part} />
    </ComposerPage>
  );
}
