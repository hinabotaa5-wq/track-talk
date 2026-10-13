import type { ReactNode } from "react";

export function Rule({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="space-y-2">
      <h2 className="text-base font-semibold">{title}</h2>
      <div className="space-y-2 text-sm leading-7 text-foreground">{children}</div>
    </section>
  );
}

export function Article({ n, title, children }: { n: number; title: string; children: ReactNode }) {
  return (
    <section className="space-y-2">
      <h2 className="text-base font-semibold">
        第{n}条（{title}）
      </h2>
      <div className="space-y-2 text-sm leading-7 text-foreground">{children}</div>
    </section>
  );
}
