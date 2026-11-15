import type { ReactNode } from "react";

export function Rule({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="space-y-2">
      <h2 className="text-base font-semibold">{title}</h2>
      <div className="space-y-2 text-sm leading-7 text-slate-300">{children}</div>
    </section>
  );
}

export function Article({ n, title, id, children }: { n: number; title: string; id?: string; children: ReactNode }) {
  return (
    <section className="space-y-2">
      <h2 id={id} className="text-base font-semibold target:scroll-mt-20">
        第{n}条（{title}）
      </h2>
      <div className="space-y-2 text-sm leading-7 text-slate-300">{children}</div>
    </section>
  );
}
