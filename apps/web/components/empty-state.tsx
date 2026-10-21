export function EmptyState({ title, body }: { title: string; body?: string }) {
  return (
    <div className="rounded-xl border border-dashed border-white/25 glass bg-white/8 p-8 text-center">
      <h2 className="font-semibold text-slate-100">{title}</h2>
      {body ? <p className="mt-2 text-sm text-slate-200">{body}</p> : null}
    </div>
  );
}

export function ErrorState({ message }: { message: string }) {
  return (
    <div className="rounded-xl border border-red-900/70 bg-red-950/40 p-4 text-sm text-red-300">
      {message}
    </div>
  );
}
