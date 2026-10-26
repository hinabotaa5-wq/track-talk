import { xPostPermalink } from "@/lib/x-url";

export function XPostLink({ id }: { id: string }) {
  const href = xPostPermalink(id);
  if (!href) return null;
  return <a href={href} target="_blank" rel="noopener noreferrer" className="block rounded-xl border border-white/20 bg-white/10 p-5 text-sm text-sky-200 hover:bg-white/15">
    <span className="block text-lg font-bold text-white">X</span>
    <span>Xで元の投稿を開く ↗</span>
  </a>;
}
