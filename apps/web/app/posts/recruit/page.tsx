"use client";

import { ComposerPage } from "@/components/composer-page";
import { RecruitmentForm } from "@/components/recruitment-form";

export default function RecruitPostPage() {
  return (
    <ComposerPage title="クラブチームの募集">
      <RecruitmentForm />
    </ComposerPage>
  );
}
