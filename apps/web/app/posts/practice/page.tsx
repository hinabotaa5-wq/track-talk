"use client";

import { ComposerPage } from "@/components/composer-page";
import { PracticeForm } from "@/components/practice-form";

export default function PracticePostPage() {
  return (
    <ComposerPage title="練習相手募集" lead="練習場所と日時を書いて、一緒に練習する相手を募ります。">
      <PracticeForm />
    </ComposerPage>
  );
}
