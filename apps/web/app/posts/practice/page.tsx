"use client";
import { L } from "@/lib/i18n";

import { ComposerPage } from "@/components/composer-page";
import { PracticeForm } from "@/components/practice-form";

export default function PracticePostPage() {
  return (
    <ComposerPage title={L("練習相手募集")}>
      <PracticeForm />
    </ComposerPage>
  );
}
