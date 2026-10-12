"use client";

import { ComposerPage } from "@/components/composer-page";
import { RecruitmentForm } from "@/components/recruitment-form";

export default function RecruitPostPage() {
  return (
    <ComposerPage title="クラブチームの募集" lead="チームの活動と募集条件を書いて、メンバーを募ります。">
      <RecruitmentForm />
    </ComposerPage>
  );
}
