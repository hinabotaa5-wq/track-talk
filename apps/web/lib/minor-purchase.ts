import { useCallback, useRef, useState } from "react";

// (1) の全ユーザーに求める購入前チェック。
// 文案は運営者が 2026-11-13 に確定。変えたら API の purchaseNoticeVersion と文面の定数も上げる・直す
// （API は記録に文面を写し、テストでこのファイルの文面と突き合わせる）。
export const purchaseNoticeLabel = "未成年の方は保護者に許可を取ってください";

// 宣言の表示・操作の時刻（証跡として API に送り、記録に残す）。端末の時計による。
export type MinorDeclarationLog = { shownAt?: string; checkedAt?: string };

type MinorDeclarationBody = {
  guardian_permission?: boolean;
  guardian_permission_shown_at?: string;
  guardian_permission_checked_at?: string;
};

// チェックが必要な決済操作でAPIに送る本文。
export function minorDeclarationBody(required: boolean, checked: boolean, log: MinorDeclarationLog = {}): MinorDeclarationBody {
  if (!required) return {};
  const body: MinorDeclarationBody = { guardian_permission: checked };
  if (log.shownAt) body.guardian_permission_shown_at = log.shownAt;
  if (checked && log.checkedAt) body.guardian_permission_checked_at = log.checkedAt;
  return body;
}

// useMinorDeclaration は、宣言のチェックの状態と、表示した時刻・チェックを入れた時刻を持つ。
// 表示した時刻は最初に出した時のまま、チェックの時刻は最後にチェックを入れた時にする（外したら消す）。
export function useMinorDeclaration() {
  const [checked, setCheckedState] = useState(false);
  const log = useRef<MinorDeclarationLog>({});
  const setChecked = useCallback((value: boolean) => {
    setCheckedState(value);
    log.current = { ...log.current, checkedAt: value ? new Date().toISOString() : undefined };
  }, []);
  const markShown = useCallback(() => {
    if (!log.current.shownAt) log.current = { ...log.current, shownAt: new Date().toISOString() };
  }, []);
  const body = useCallback((required: boolean) => minorDeclarationBody(required, checked, log.current), [checked]);
  return { checked, setChecked, markShown, body };
}
