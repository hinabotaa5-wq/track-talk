"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { EmptyState, ErrorState } from "@/components/empty-state";
import type { MajorHost } from "@/lib/types";

export function MajorHostsPanel({
  hosts,
  error,
}: {
  hosts: MajorHost[];
  error: string;
}) {
  const [open, setOpen] = useState(false);

  return (
    <div className="space-y-3">
      <Button
        type="button"
        variant="secondary"
        className="h-11 w-full"
        aria-expanded={open}
        onClick={() => setOpen((value) => !value)}
      >
        今後の世界陸上・オリンピック
      </Button>
      {open ? <MajorHostList hosts={hosts} error={error} /> : null}
    </div>
  );
}

function MajorHostList({ hosts, error }: { hosts: MajorHost[]; error: string }) {
  if (error) return <ErrorState message={error} />;
  if (hosts.length === 0) {
    return <EmptyState title="今後の開催地はまだありません" body="開催地が決まると、ここに表示されます。" />;
  }
  return (
    <div className="grid gap-4 sm:grid-cols-2">
      {hosts.map((host) => (
        <article key={`${host.kind}-${host.year}`} className="overflow-hidden rounded-xl border bg-white">
          <HostPhoto url={host.image_url} alt={cityName(host)} />
          <div className="space-y-1 p-4">
            <h2 className="text-2xl font-bold">{cityName(host)}</h2>
            <p className="font-medium">
              {host.year}年・{kindLabel(host.kind)}
            </p>
            <p className="text-sm text-zinc-600">{scheduleRange(host.schedule_label)}</p>
            {host.page_url ? (
              <a href={host.page_url} target="_blank" rel="noopener noreferrer" className="inline-block text-sm text-blue-700">
                Wikipedia
              </a>
            ) : null}
          </div>
        </article>
      ))}
    </div>
  );
}

function cityName(host: MajorHost) {
  const name = host.city || host.country;
  if (name.endsWith("市") && name.length > 1) return name.slice(0, -1);
  return name;
}

function scheduleRange(label: string) {
  const rest = label.replace(/^\d{4}年/, "");
  if (rest.includes("日")) return rest;
  if (/^\d{1,2}月$/.test(rest)) return `${rest}（日にち未定）`;
  return "日程未定";
}

function HostPhoto({ url, alt }: { url: string; alt: string }) {
  const [failed, setFailed] = useState(false);
  const visible = url !== "" && !failed;
  return (
    <div className="relative aspect-[4/3] bg-zinc-100">
      {visible ? (
        <>
          <img src={url} alt={alt} className="h-full w-full object-cover" onError={() => setFailed(true)} />
          <p className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/55 to-transparent px-4 pb-2 pt-8 text-xs text-white">
            写真: Wikimedia Commons
          </p>
        </>
      ) : null}
    </div>
  );
}

function kindLabel(kind: MajorHost["kind"]) {
  return kind === "olympics" ? "オリンピック" : "世界陸上";
}
