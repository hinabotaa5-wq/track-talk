import Link from "next/link";

export function Footer() {
  return (
    <footer className="mt-auto border-t border-zinc-200 bg-white">
      <div className="mx-auto flex max-w-5xl flex-wrap gap-4 px-4 py-6 text-sm text-zinc-500">
        <Link href="/about">TRACK TALKについて</Link>
        <Link href="/guidelines">ガイドライン</Link>
        <Link href="/terms">利用規約</Link>
        <Link href="/privacy">プライバシー</Link>
      </div>
    </footer>
  );
}
