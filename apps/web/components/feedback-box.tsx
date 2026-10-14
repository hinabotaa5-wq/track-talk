"use client";

import { apiFetch, ApiRequestError } from "@/lib/api";
import { useActionLock } from "@/lib/use-action-lock";
import { Box } from "lucide-react";
import { useId, useRef, useState } from "react";
import { AnchoredPanel } from "./anchored-panel";
import { useAuth } from "./auth-provider";
import { Button } from "./ui/button";
import { Textarea } from "./ui/textarea";

const prompt = "思ったことをなんでも書いてね(欲しい機能、修正点、このアプリの悪口などなど)";

function subjectFrom(message: string) {
  return Array.from(message).slice(0, 80).join("");
}

export function FeedbackBox() {
  const { token } = useAuth();
  const panelID = useId();
  const rootRef = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [sent, setSent] = useState(false);
  const { pending, acquire, release } = useActionLock();

  async function send() {
    const body = message.trim();
    if (!body) {
      setSent(false);
      setError("書いてから送ってね");
      return;
    }
    if (Array.from(body).length > 2000) {
      setSent(false);
      setError("2000文字以内で書いてね");
      return;
    }
    if (!token || !acquire()) return;
    setError("");
    setSent(false);
    try {
      await apiFetch("/v1/me/inquiries", {
        method: "POST",
        token,
        body: JSON.stringify({
          category: "other",
          source: "feedback",
          subject: subjectFrom(body),
          body,
        }),
      });
      setMessage("");
      setSent(true);
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.message : "送れなかった。もう一度試してね");
    } finally {
      release();
    }
  }

  return (
    <div ref={rootRef} className="relative">
      <button
        type="button"
        aria-label="ご意見"
        aria-expanded={open}
        aria-controls={panelID}
        onClick={() => setOpen((current) => !current)}
        className="inline-flex size-11 items-center justify-center rounded-full text-slate-300 transition hover:bg-white/16 active:scale-95"
      >
        <Box className="size-5" strokeWidth={2} aria-hidden />
      </button>
      <AnchoredPanel
        open={open}
        anchorRef={rootRef}
        panelID={panelID}
        rightShiftRem={3}
        onClose={() => setOpen(false)}
        className="w-80 max-w-[calc(100vw-2rem)] overflow-hidden rounded-xl border border-white/20 bg-[#16243a] p-3 shadow-xl"
      >
          <p className="text-balance text-sm font-semibold leading-snug text-slate-100">{prompt}</p>
          <form
            className="mt-3 space-y-2"
            onSubmit={(event) => {
              event.preventDefault();
              void send();
            }}
          >
            <Textarea
              rows={4}
              value={message}
              placeholder="ここに書いてね"
              aria-label={prompt}
              onChange={(event) => {
                setMessage(event.target.value);
                setSent(false);
              }}
            />
            {error ? <p className="text-sm text-red-400">{error}</p> : null}
            {sent ? <p className="text-sm text-blue-400">受け取ったよ</p> : null}
            <Button type="submit" className="w-full" disabled={pending}>
              送る
            </Button>
          </form>
      </AnchoredPanel>
    </div>
  );
}
