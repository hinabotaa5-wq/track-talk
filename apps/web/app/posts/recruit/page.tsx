"use client";
import { L } from "@/lib/i18n";

import { ComposerPage } from "@/components/composer-page";
import { RecruitmentForm } from "@/components/recruitment-form";

export default function RecruitPostPage() {
  return (
    <ComposerPage title={L("クラブチームの募集")}>
      <RecruitmentForm />
    </ComposerPage>
  );
}
