import { PostDetail } from "./post-detail";

export default async function PostPage({
  params,
  searchParams,
}: {
  params: Promise<{ id: string }>;
  searchParams: Promise<{ from?: string; play?: string }>;
}) {
  const { id } = await params;
  const { from, play } = await searchParams;
  return <PostDetail id={id} returnTo={from === "me" ? "me" : undefined} autoplay={play === "1"} />;
}
