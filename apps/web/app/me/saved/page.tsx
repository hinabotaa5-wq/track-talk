import { SavedFeed } from "@/components/saved-feed";

export default async function SavedPostsPage({ searchParams }: { searchParams: Promise<{ page?: string }> }) {
  const sp = await searchParams;
  const page = Math.max(1, Number(sp.page) || 1);

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">保存した投稿</h1>
      <SavedFeed key={page} page={page} />
    </div>
  );
}
