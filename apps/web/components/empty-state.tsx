export function EmptyState({ title, body }: { title: string; body: string }) {
  return (
    <div className="rounded-xl border border-dashed border-input-border bg-field p-8 text-center">
      <h2 className="font-semibold">{title}</h2>
      <p className="mt-2 text-sm text-muted">{body}</p>
    </div>
  );
}

export function ErrorState({ message }: { message: string }) {
  return (
    <div className="rounded-xl border border-danger/40 bg-danger-soft p-4 text-sm text-danger">
      {message}
    </div>
  );
}
