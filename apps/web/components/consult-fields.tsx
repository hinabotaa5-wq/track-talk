"use client";
import { L } from "@/lib/i18n";

import type { Choice } from "@/lib/support-consult";
import type { ReactNode } from "react";

// 無料相談のフォームで使う入力部品。選ぶだけで答えられる形にそろえる。

const chip = (on: boolean) =>
  `rounded-full border px-3 py-1.5 text-sm transition active:scale-[0.98] ${
    on ? "border-blue-400 bg-blue-500/30 text-pure-white" : "border-white/25 bg-white/10 text-slate-200"
  }`;

export function FieldBlock({ label, error, children }: { label: string; error?: string; children: ReactNode }) {
  return (
    <div className="space-y-1.5">
      <p className="text-sm font-semibold">{label}</p>
      {children}
      {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null}
    </div>
  );
}

export function YesNo({ label, value, onChange, error }: { label: string; value: boolean | null; onChange: (v: boolean) => void; error?: string }) {
  return (
    <FieldBlock label={label} error={error}>
      <div className="flex gap-2" role="group" aria-label={label}>
        <button type="button" aria-pressed={value === true} className={chip(value === true)} onClick={() => onChange(true)}>{L("はい")}</button>
        <button type="button" aria-pressed={value === false} className={chip(value === false)} onClick={() => onChange(false)}>{L("いいえ")}</button>
      </div>
    </FieldBlock>
  );
}

export function SingleChoice({ label, choices, value, onChange, error }: { label: string; choices: Choice[]; value: string; onChange: (v: string) => void; error?: string }) {
  return (
    <FieldBlock label={label} error={error}>
      <div className="flex flex-wrap gap-2" role="group" aria-label={label}>
        {choices.map((x) => (
          <button key={x.value} type="button" aria-pressed={value === x.value} className={chip(value === x.value)} onClick={() => onChange(x.value)}>{x.label}</button>
        ))}
      </div>
    </FieldBlock>
  );
}

export function MultiChoice({ label, choices, values, onChange, error }: { label: string; choices: Choice[]; values: string[]; onChange: (v: string[]) => void; error?: string }) {
  return (
    <FieldBlock label={L("{label}（複数選べます）", { label })} error={error}>
      <div className="flex flex-wrap gap-2" role="group" aria-label={label}>
        {choices.map((x) => {
          const on = values.includes(x.value);
          return (
            <button key={x.value} type="button" aria-pressed={on} className={chip(on)}
              onClick={() => onChange(on ? values.filter((v) => v !== x.value) : [...values, x.value])}>{x.label}</button>
          );
        })}
      </div>
    </FieldBlock>
  );
}

export const selectClass = "rounded border border-white/25 bg-slate-900 p-2 text-sm";
