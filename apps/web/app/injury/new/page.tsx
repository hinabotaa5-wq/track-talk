"use client";

import { ComposerPage } from "@/components/composer-page";
import { InjuryForm } from "@/components/injury-form";

export default function InjuryPostPage() {
  return (
    <ComposerPage title="ケガの投稿">
      <InjuryForm />
    </ComposerPage>
  );
}
