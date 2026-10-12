"use client";

import { ComposerPage } from "@/components/composer-page";
import { PostForm } from "@/components/post-form";

export default function NewPostPage() {
  return (
    <ComposerPage title="文章">
      <PostForm />
    </ComposerPage>
  );
}
