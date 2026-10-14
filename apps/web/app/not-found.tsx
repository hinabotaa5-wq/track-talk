import Link from "next/link";

export default function NotFound() {
  return (
    <div className="space-y-3">
      <h1 className="text-2xl font-bold">ページが見つかりません</h1>
      <Link href="/" className="inline-block text-blue-400 transition active:scale-95">
        トップへ戻る
      </Link>
    </div>
  );
}
