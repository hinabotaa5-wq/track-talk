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
    <div className="space-y-3">
      {hosts.map((host) => (
        <article key={`${host.kind}-${host.year}`} className="overflow-hidden rounded-xl border bg-white">
          <HostPhoto url={host.image_url} alt={host.city} />
          <div className="space-y-1 p-4">
            <p className="text-xs text-zinc-500">{kindLabel(host.kind)}</p>
            <h2 className="text-lg font-semibold">{host.name}</h2>
            <p className="text-sm text-zinc-600">{placeLabel(host)}</p>
            <p className="text-sm text-zinc-600">{host.schedule_label}</p>
            {host.page_url ? (
              <a href={host.page_url} target="_blank" rel="noopener noreferrer" className="text-sm text-blue-700">
                Wikipedia
              </a>
            ) : null}
          </div>
        </article>
      ))}
    </div>
  );
}

function HostPhoto({ url, alt }: { url: string; alt: string }) {
  const [failed, setFailed] = useState(false);
  const visible = url !== "" && !failed;
  return (
    <div>
      <div className="aspect-video bg-zinc-100">
        {visible ? (
          <img src={url} alt={alt} className="h-full w-full object-cover" onError={() => setFailed(true)} />
        ) : null}
      </div>
      {visible ? <p className="px-4 pt-2 text-xs text-zinc-500">写真: Wikipedia</p> : null}
    </div>
  );
}

function kindLabel(kind: MajorHost["kind"]) {
  return kind === "olympics" ? "オリンピック" : "世界陸上";
}

function placeLabel(host: MajorHost) {
  const city = host.country ? `${host.city}（${host.country}）` : host.city;
  return host.venue ? `${city} ${host.venue}` : city;
}
