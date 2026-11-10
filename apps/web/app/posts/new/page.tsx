"use client";
import { L } from "@/lib/i18n";

import { ComposerPage } from "@/components/composer-page";
import { PostForm } from "@/components/post-form";

export default function NewPostPage() {
  return (
    <ComposerPage title={L("文章・写真")}>
      <PostForm />
    </ComposerPage>
  );
}
