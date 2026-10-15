"use client";

import { Check } from "lucide-react";

const optionClass =
  "w-full rounded-xl border px-4 py-3 text-left text-base font-medium transition active:scale-[0.98] disabled:cursor-not-allowed disabled:opacity-50";
const optionIdle = "border-white/20 glass bg-white/10 text-slate-100 hover:bg-white/16";
const optionActive = "border-blue-400 bg-blue-950/60 text-white";

// ケガ部の質問で使う、ボタンを並べた選択肢。1つ選ぶ質問にも、いくつでも選べる質問にも使う。
export function InjuryChoices<T extends string>({
  label,
  options,
  selected,
  onSelect,
  disabled,
  note,
}: {
  label: string;
  options: readonly { value: T; label: string; detail?: string }[];
  selected: (value: T) => boolean;
  onSelect: (value: T) => void;
  disabled?: (value: T) => boolean;
  note?: (value: T) => string;
}) {
  return (
    <div className="space-y-2" role="group" aria-label={label}>
      {options.map((item) => (
        <button
          key={item.value}
          type="button"
          aria-pressed={selected(item.value)}
          disabled={disabled?.(item.value)}
          onClick={() => onSelect(item.value)}
          className={`${optionClass} flex items-center justify-between gap-3 ${selected(item.value) ? optionActive : optionIdle}`}
        >
          <span>
            {item.label}
            {note?.(item.value) ? <span className="text-sm font-normal text-slate-300">{note(item.value)}</span> : null}
            {item.detail ? <span className="block text-sm font-normal text-slate-300">{item.detail}</span> : null}
          </span>
          {selected(item.value) ? <Check aria-hidden className="size-5 shrink-0 text-blue-300" /> : null}
        </button>
      ))}
    </div>
  );
}
