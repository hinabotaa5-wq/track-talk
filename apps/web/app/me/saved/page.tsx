"use client";

import { L } from "@/lib/i18n";
import { SavedFeed } from "@/components/saved-feed";

export default function SavedPostsPage() {
  return <div className="space-y-4"><h1 className="text-2xl font-bold">{L("保存した投稿")}</h1><SavedFeed /></div>;
}
