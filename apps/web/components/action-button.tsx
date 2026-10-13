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
          ? "bg-rose-50 text-rose-600 border border-rose-200 hover:bg-rose-100"
          : "border border-zinc-200 bg-white text-zinc-700 hover:bg-zinc-50 hover:text-zinc-900"
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
            liked ? "fill-rose-500 text-rose-500" : "text-zinc-500"
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
          ? "bg-blue-50 text-blue-600 border border-blue-200 hover:bg-blue-100"
          : "border border-zinc-200 bg-white text-zinc-700 hover:bg-zinc-50 hover:text-zinc-900"
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
            saved ? "fill-blue-600 text-blue-600" : "text-zinc-500"
          )}
        />
      </motion.span>
      <span>{saved ? "保存済み" : "保存"}</span>
    </motion.button>
  );
}
