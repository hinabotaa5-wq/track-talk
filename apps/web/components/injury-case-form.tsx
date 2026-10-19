"use client";

import Link from "next/link";
import { useEffect, useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { injuryBodyParts } from "@/lib/injury";
import { toggleChoice, type InjuryCaseOptions } from "@/lib/injury-cases";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { useActionLock } from "@/lib/use-action-lock";
import { useAuth } from "./auth-provider";
import { Button } from "./ui/button";
import { InjuryChoices } from "./injury-choices";

// ケガをしたときの状況と、どうすればよかったかを、選ぶだけで答える。自由記述は無い。
export function InjuryCaseForm() {
  const { token } = useAuth();
  const router = useRouter();
  const [options, setOptions] = useState<InjuryCaseOptions | null>(null);
  const [part, setPart] = useState("");
  const [timing, setTiming] = useState("");
  const [situations, setSituations] = useState<string[]>([]);
  const [preventions, setPreventions] = useState<string[]>([]);
  const [error, setError] = useState("");
  const [done, setDone] = useState(false);
  const { pending, acquire, release } = useActionLock();

  useEffect(() => {
    const controller = new AbortController();
    apiFetch<InjuryCaseOptions>("/v1/injury/cases/options", { signal: controller.signal })
      .then(setOptions)
      .catch(() => {
        if (!controller.signal.aborted) setError("選択肢の取得に失敗しました。もう一度開いてください。");
      });
    return () => controller.abort();
  }, []);

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    if (!options) return;
    setError("");
    if (!part) return setError("ケガを選んでください");
    if (!timing) return setError("ケガをしたときを選んでください");
    if (situations.length === 0) return setError("そのときの状況を選んでください");
    if (preventions.length === 0) return setError("どうすればよかったかを選んでください");
    if (!acquire()) return;
    try {
      await apiFetch("/v1/injury/cases", {
        method: "POST",
        token,
        body: JSON.stringify({ body_part: part, timing, situations, preventions }),
      });
      setDone(true);
    } catch (e) {
      if (e instanceof ApiRequestError) {
        const field = e.fields ? Object.values(e.fields)[0] : "";
        setError(field || e.message);
      } else {
        setError("送信に失敗しました");
      }
      release();
    }
  }

  if (done) {
    return (
      <div className="space-y-4 rounded-xl border border-white/20 glass bg-white/10 p-5">
        <p className="font-semibold">答えてくれてありがとうございます。</p>
        <p className="text-sm text-slate-200">あなたの経験は、まだケガをしていない人の予防に使われます。</p>
        <div className="flex gap-3">
          <Button type="button" onClick={() => router.push("/injury/data")}>データを見る</Button>
          <Button
            type="button"
            variant="secondary"
            onClick={() => {
              setPart("");
              setTiming("");
              setSituations([]);
              setPreventions([]);
              setDone(false);
              release();
            }}
          >
            別のケガを答える
          </Button>
        </div>
      </div>
    );
  }

  const max = options?.max_choices ?? 4;

  return (
    <form onSubmit={onSubmit} className="space-y-6">
      <p className="text-sm text-slate-200">
        選ぶだけで答えられます。名前・日付・学校・場所は聞かず、回答は集計だけに使い、ほかの人には見えません。
        自分の回答は <Link href="/me/injury" className="underline">マイページ</Link> から消せます。
      </p>
      <fieldset className="space-y-2">
        <legend className="text-sm font-medium">どのケガですか</legend>
        <InjuryChoices
          label="ケガ"
          options={injuryBodyParts}
          selected={(v) => part === v}
          onSelect={setPart}
        />
      </fieldset>
      {options ? (
        <>
          <fieldset className="space-y-2">
            <legend className="text-sm font-medium">ケガをしたのは、いつですか</legend>
            <InjuryChoices label="ケガをしたとき" options={options.timings} selected={(v) => timing === v} onSelect={setTiming} />
          </fieldset>
          <fieldset className="space-y-2">
            <legend className="text-sm font-medium">そのときの状況(あてはまるものを{max}つまで)</legend>
            <InjuryChoices
              label="そのときの状況"
              options={options.situations}
              selected={(v) => situations.includes(v)}
              onSelect={(v) => setSituations((cur) => toggleChoice(cur, v, max))}
              disabled={(v) => !situations.includes(v) && situations.length >= max}
            />
          </fieldset>
          <fieldset className="space-y-2">
            <legend className="text-sm font-medium">いま思うと、どうすればよかったですか({max}つまで)</legend>
            <InjuryChoices
              label="どうすればよかったか"
              options={options.preventions}
              selected={(v) => preventions.includes(v)}
              onSelect={(v) => setPreventions((cur) => toggleChoice(cur, v, max))}
              disabled={(v) => !preventions.includes(v) && preventions.length >= max}
            />
          </fieldset>
        </>
      ) : null}
      {error ? <p className="text-sm text-red-400">{error}</p> : null}
      <Button type="submit" className="w-full" disabled={pending || !options}>
        {pending ? "送っています" : "答えを送る"}
      </Button>
    </form>
  );
}
