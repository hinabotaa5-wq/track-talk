"use client";

import { useAuth } from "@/components/auth-provider";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { formatDateTime } from "@/lib/format";
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
  const form = useForm<Values>({
    resolver: zodResolver(inquirySchema),
    defaultValues: { category: "bug", subject: "", body: "" },
  });

  useEffect(() => {
    if (!loading && !session) router.replace("/login");
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
    setError("");
    setSent(false);
    try {
      const created = await apiFetch<Inquiry>("/v1/me/inquiries", {
        method: "POST",
        token,
        body: JSON.stringify(values),
      });
      setInquiries((current) => [created, ...current]);
      form.reset({ category: values.category, subject: "", body: "" });
      setSent(true);
    } catch (err) {
      if (err instanceof ApiRequestError) {
        const field = err.fields ? Object.values(err.fields)[0] : "";
        setError(field || err.message);
        return;
      }
      setError("送信に失敗しました");
    }
  }

  if (loading || !session) {
    return <p className="text-sm text-zinc-600">読み込み中...</p>;
  }

  return (
    <div className="mx-auto max-w-md space-y-8">
      <div className="space-y-2">
        <h1 className="text-2xl font-bold">お問い合わせ</h1>
        <p className="text-sm text-zinc-600">不具合やアカウントの相談を運営へ送れます。</p>
      </div>
      <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-3">
        <label className="block space-y-1 text-sm">
          <span className="text-zinc-700">種類</span>
          <select
            className="w-full rounded-md border border-zinc-300 bg-white px-3 py-2 text-base outline-none focus:border-blue-500"
            {...form.register("category")}
          >
            {inquiryCategories.map((category) => (
              <option key={category} value={category}>
                {inquiryCategoryLabels[category]}
              </option>
            ))}
          </select>
        </label>
        <Input placeholder="件名" {...form.register("subject")} />
        {form.formState.errors.subject ? (
          <p className="text-sm text-red-600">{form.formState.errors.subject.message}</p>
        ) : null}
        <Textarea rows={6} placeholder="内容" {...form.register("body")} />
        {form.formState.errors.body ? (
          <p className="text-sm text-red-600">{form.formState.errors.body.message}</p>
        ) : null}
        {error ? <p className="text-sm text-red-600">{error}</p> : null}
        {sent ? <p className="text-sm text-blue-700">送信しました。</p> : null}
        <Button type="submit" className="w-full" disabled={form.formState.isSubmitting}>
          送信
        </Button>
      </form>
      <section className="space-y-3">
        <h2 className="text-lg font-semibold">送信履歴</h2>
        {inquiries.length === 0 ? (
          <p className="text-sm text-zinc-600">まだお問い合わせはありません</p>
        ) : (
          <ul className="space-y-3">
            {inquiries.map((item) => (
              <li key={item.id} className="space-y-1 rounded-md border border-zinc-200 p-3">
                <p className="text-xs text-zinc-500">
                  {inquiryCategoryLabels[item.category]} · {formatDateTime(item.created_at)}
                </p>
                <p className="font-medium">{item.subject}</p>
                <p className="whitespace-pre-wrap text-sm text-zinc-700">{item.body}</p>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
