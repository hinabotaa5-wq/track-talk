"use client";

import { useAuth } from "@/components/auth-provider";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { apiFetch, apiErrorText } from "@/lib/api";
import { coachingEnabled } from "@/lib/features";
import { formatDateTime } from "@/lib/format";
import { useActionLock } from "@/lib/use-action-lock";
import type { Inquiry, Meta } from "@/lib/types";
import { inquiryCategories, inquiryCategoryLabels, inquirySchema } from "@/lib/validators";
import { zodResolver } from "@hookform/resolvers/zod";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import type { z } from "zod";

type Values = z.infer<typeof inquirySchema>;

export default function ContactPage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const [inquiries, setInquiries] = useState<Inquiry[]>([]);
  const [error, setError] = useState("");
  const [sent, setSent] = useState(false);
  const { pending, acquire, release } = useActionLock();
  const form = useForm<Values>({
    resolver: zodResolver(inquirySchema),
    defaultValues: { category: "bug", email: "", subject: "", body: "" },
  });

  // 特商法の表記のページから来たときは、開示請求を選んだ状態にする。
  useEffect(() => {
    if (coachingEnabled && new URLSearchParams(window.location.search).get("category") === "disclosure") {
      form.setValue("category", "disclosure");
    }
  }, [form]);

  useEffect(() => {
    if (!loading && !session) {
      const here = window.location.pathname + window.location.search;
      router.replace(`/login?next=${encodeURIComponent(here)}`);
    }
  }, [loading, session, router]);

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    apiFetch<{ inquiries: Inquiry[]; meta: Meta }>("/v1/me/inquiries", { token })
      .then((d) => {
        if (cancelled) return;
        setInquiries((current) => {
          const shown = new Set(current.map((inquiry) => inquiry.id));
          return [...current, ...(d.inquiries ?? []).filter((inquiry) => !shown.has(inquiry.id))];
        });
      })
      .catch(() => {
        if (!cancelled) setError("送信履歴の取得に失敗しました");
      });
    return () => {
      cancelled = true;
    };
  }, [token]);

  async function onSubmit(values: Values) {
    if (!acquire()) return;
    setError("");
    setSent(false);
    try {
      const created = await apiFetch<Inquiry>("/v1/me/inquiries", {
        method: "POST",
        token,
        body: JSON.stringify(values),
      });
      setInquiries((current) => [created, ...current]);
      form.reset({ category: values.category, email: values.email, subject: "", body: "" });
      setSent(true);
    } catch (err) {
      setError(apiErrorText(err, "送信に失敗しました"));
    } finally {
      release();
    }
  }

  if (loading || !session) {
    return <p className="text-sm text-slate-200">読み込み中...</p>;
  }

  return (
    <div className="mx-auto max-w-md space-y-8">
      <div className="space-y-2">
        <h1 className="text-2xl font-bold">お問い合わせ</h1>
        <p className="text-sm text-slate-200">不具合やアカウントの相談を運営へ送れます。</p>
      </div>
      <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-3">
        <label className="block space-y-1 text-sm">
          <span className="text-slate-300">種類</span>
          <select
            className="w-full rounded-md border border-white/25 glass bg-white/10 px-3 py-2 text-base text-slate-100 outline-none focus:border-blue-400"
            {...form.register("category")}
          >
            {/* 開示請求は、有料の機能と表記のページを出している間だけ選べる。 */}
            {inquiryCategories.filter((category) => coachingEnabled || category !== "disclosure").map((category) => (
              <option key={category} value={category}>
                {inquiryCategoryLabels[category]}
              </option>
            ))}
          </select>
        </label>
        <Input type="email" autoComplete="email" placeholder="返信先のメールアドレス" {...form.register("email")} />
        {form.formState.errors.email ? (
          <p className="text-sm text-red-400">{form.formState.errors.email.message}</p>
        ) : null}
        <Input placeholder="件名" {...form.register("subject")} />
        {form.formState.errors.subject ? (
          <p className="text-sm text-red-400">{form.formState.errors.subject.message}</p>
        ) : null}
        <Textarea rows={6} placeholder="内容" {...form.register("body")} />
        {form.formState.errors.body ? (
          <p className="text-sm text-red-400">{form.formState.errors.body.message}</p>
        ) : null}
        {error ? <p className="text-sm text-red-400">{error}</p> : null}
        {sent ? <p className="text-sm text-blue-400">送信しました。</p> : null}
        <Button type="submit" className="w-full" disabled={pending}>
          送信
        </Button>
      </form>
      <section className="space-y-3">
        <h2 className="text-lg font-semibold">送信履歴</h2>
        {inquiries.length === 0 ? (
          <p className="text-sm text-slate-200">まだお問い合わせはありません</p>
        ) : (
          <ul className="space-y-3">
            {inquiries.map((item) => (
              <li key={item.id} className="space-y-1 rounded-xl border border-white/20 glass bg-white/10 p-3">
                <p className="text-xs text-slate-200">
                  {inquiryCategoryLabels[item.category]} · {formatDateTime(item.created_at)}
                </p>
                <p className="font-medium">{item.subject}</p>
                <p className="whitespace-pre-wrap text-sm text-slate-300">{item.body}</p>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
