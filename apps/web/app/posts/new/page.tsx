"use client";

import { ComposerPage } from "@/components/composer-page";
import { PostForm } from "@/components/post-form";

export default function NewPostPage() {
  return (
    <ComposerPage
      title="文章"
      lead="公式記録の転載ではなく、自分の視点での議論を書いてください。"
    >
      <PostForm />
    </ComposerPage>
  );
}
