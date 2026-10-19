import Link from "next/link";

const tabs = [
  { id: "share", href: "/injury", label: "ケガ共有" },
  { id: "data", href: "/injury/data", label: "データ" },
] as const;

// ケガタブの中の切り替え。「ケガ共有」は同じケガの仲間のグループ、「データ」は予防に役立てる匿名の集計。
export function InjuryTabs({ current }: { current: (typeof tabs)[number]["id"] }) {
  return (
    <nav aria-label="ケガタブの切り替え" className="grid grid-cols-2 gap-1 rounded-xl border border-white/20 glass bg-white/8 p-1">
      {tabs.map((tab) => (
        <Link
          key={tab.id}
          href={tab.href}
          aria-current={current === tab.id ? "page" : undefined}
          className={`rounded-lg py-2 text-center text-sm font-semibold transition ${
            current === tab.id ? "bg-blue-950/70 text-white" : "text-slate-200 hover:bg-white/10"
          }`}
        >
          {tab.label}
        </Link>
      ))}
    </nav>
  );
}
