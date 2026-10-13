"use client";

import { motion } from "motion/react";
import { Heart, Bookmark } from "lucide-react";
import { cn } from "@/lib/utils";

export function LikeButton({
  liked,
  count,
  disabled,
  onClick,
}: {
  liked: boolean;
  count: number;
  disabled?: boolean;
  onClick: () => void;
}) {
  return (
    <motion.button
      type="button"
      whileTap={{ scale: 0.92 }}
      disabled={disabled}
      onClick={onClick}
      className={cn(
        "inline-flex items-center gap-2 rounded-full px-4 py-2 text-sm font-semibold transition-colors duration-200 shadow-xs cursor-pointer disabled:opacity-50",
        liked
          ? "bg-like-soft text-like border border-like/40 hover:bg-like-soft/80"
          : "border border-border bg-surface text-foreground hover:bg-surface-raised"
      )}
    >
      <motion.span
        key={String(liked)}
        initial={liked ? { scale: 0.6 } : { scale: 1 }}
        animate={{ scale: 1 }}
        transition={{ type: "spring", stiffness: 500, damping: 15 }}
      >
        <Heart
          className={cn(
            "size-4 transition-colors",
            liked ? "fill-like text-like" : "text-muted"
          )}
        />
      </motion.span>
      <span>いいね</span>
      <span className="font-bold tabular-nums">{count}</span>
    </motion.button>
  );
}

export function SaveButton({
  saved,
  disabled,
  onClick,
}: {
  saved: boolean;
  disabled?: boolean;
  onClick: () => void;
}) {
  return (
    <motion.button
      type="button"
      whileTap={{ scale: 0.92 }}
      disabled={disabled}
      onClick={onClick}
      className={cn(
        "inline-flex items-center gap-2 rounded-full px-4 py-2 text-sm font-semibold transition-colors duration-200 shadow-xs cursor-pointer disabled:opacity-50",
        saved
          ? "bg-accent-soft text-accent border border-border hover:bg-surface-raised"
          : "border border-border bg-surface text-foreground hover:bg-surface-raised"
      )}
    >
      <motion.span
        key={String(saved)}
        initial={saved ? { scale: 0.6 } : { scale: 1 }}
        animate={{ scale: 1 }}
        transition={{ type: "spring", stiffness: 500, damping: 15 }}
      >
        <Bookmark
          className={cn(
            "size-4 transition-colors",
            saved ? "fill-accent text-accent" : "text-muted"
          )}
        />
      </motion.span>
      <span>{saved ? "保存済み" : "保存"}</span>
    </motion.button>
  );
}
