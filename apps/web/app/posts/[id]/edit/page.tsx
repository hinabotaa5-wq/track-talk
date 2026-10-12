import { EditPost } from "./edit-post";

export default async function EditPostPage({
  params,
  searchParams,
}: {
  params: Promise<{ id: string }>;
  searchParams: Promise<{ from?: string }>;
}) {
  const { id } = await params;
  const { from } = await searchParams;
  return <EditPost id={id} returnTo={from === "me" ? "me" : undefined} />;
}
