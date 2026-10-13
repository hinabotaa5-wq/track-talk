import { cn } from "@/lib/utils";
import type { TextareaHTMLAttributes } from "react";

export function Textarea({
  className,
  ...props
}: TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return (
    <textarea
      className={cn(
        "w-full rounded-md border border-input-border bg-field px-3 py-2 text-base text-foreground placeholder:text-muted outline-none transition focus:border-accent focus:ring-2 focus:ring-accent/20",
        className,
      )}
      {...props}
    />
  );
}
