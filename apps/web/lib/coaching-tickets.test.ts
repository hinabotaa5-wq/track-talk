import { expect, it } from "vitest";
import { readFileSync } from "node:fs";
import path from "node:path";
import { coachTicketPurchaseHref, initialReviewPayment, parseReviewPayParam, ticketExpiryFor, ticketKindCounts, ticketReturnPath } from "./coaching-tickets";

it("チケット購入の戻り先で外部URL・保護者トークン・別機能への転送を許可しない", () => {
  const request = "/coaching/requests/12345678-1234-1234-1234-123456789abc";
  expect(ticketReturnPath(request)).toBe(request);
  expect(ticketReturnPath("/coaching/new")).toBe("/coaching/new");
  for (const path of ["https://evil.test", "//evil.test", "/\\evil.test", "/guardian/secret", "/coaching/requests/not-a-uuid", request + "?next=https://evil.test", "/coaching/new\n", "/admin"]) {
    expect(ticketReturnPath(path)).toBe("/me/coaching/tickets");
  }
});

// 本体 #691：API（coaching_ticket.go の ticketReturnPath）と同じ表で、許可・不許可が一致することを固定する。
const sharedReturnPaths = JSON.parse(readFileSync(path.resolve(process.cwd(), "../api/internal/httpx/testdata/ticket_return_paths.json"), "utf8")) as { allowed: string[]; rejected: string[] };

it("チケット購入の戻り先の判定が API と同じ表で一致する（コーチ詳細を含む）", () => {
  expect(sharedReturnPaths.allowed).toContain("/coaches/12345678-1234-1234-1234-123456789abc");
  for (const p of sharedReturnPaths.allowed) expect(ticketReturnPath(p), p).toBe(p);
  for (const p of sharedReturnPaths.rejected) expect(ticketReturnPath(p), JSON.stringify(p)).toBe("/me/coaching/tickets");
  expect(ticketReturnPath(null)).toBe("/me/coaching/tickets");
  expect(ticketReturnPath(undefined)).toBe("/me/coaching/tickets");
});

it("コーチ詳細から購入画面へ進むリンクにそのコーチ詳細への戻り先を付ける", () => {
  const id = "12345678-1234-1234-1234-123456789abc";
  expect(coachTicketPurchaseHref(id)).toBe("/me/coaching/tickets?return_to=%2Fcoaches%2F12345678-1234-1234-1234-123456789abc");
  // ID が分からない・UUID でないときは戻り先を付けない（従来の遷移先）。
  for (const bad of [undefined, null, "", "not-a-uuid", "../admin", id + "/x"]) expect(coachTicketPurchaseHref(bad)).toBe("/me/coaching/tickets");
});

// 本体 #687：入口の指定（?pay=）と種別ごとの残りから、申込画面の初期値を決める。
const summary = (normal: number, reward: number) => ({
  balance: { available: normal + reward, reserved: 0 },
  next_expiry: { expires_at: "2027-01-01T00:00:00Z", available: 1 },
  balance_by_kind: {
    normal: { available: normal, next_expires_at: normal ? "2027-03-01T00:00:00Z" : null },
    reward: { available: reward, next_expires_at: reward ? "2027-01-01T00:00:00Z" : null },
  },
});

it("入口の指定は card / normal / reward だけを受け取る", () => {
  expect(parseReviewPayParam("card")).toBe("card");
  expect(parseReviewPayParam("normal")).toBe("normal");
  expect(parseReviewPayParam("reward")).toBe("reward");
  for (const v of [null, undefined, "", "tickets", "paypay", "REWARD"]) expect(parseReviewPayParam(v)).toBeNull();
});

it("指定がなければ残りがあるときチケット、内訳は無料チケットを優先する", () => {
  expect(initialReviewPayment(summary(2, 1), null)).toEqual({ method: "tickets", kind: "reward" });
  expect(initialReviewPayment(summary(2, 0), null)).toEqual({ method: "tickets", kind: "normal" });
  expect(initialReviewPayment(summary(0, 1), null)).toEqual({ method: "tickets", kind: "reward" });
  expect(initialReviewPayment(summary(0, 0), null)).toEqual({ method: "card", kind: null });
  expect(initialReviewPayment(null, null)).toEqual({ method: "card", kind: null });
});

it("指定の種別を優先し、使えなければ使える側、どちらもなければ単発", () => {
  expect(initialReviewPayment(summary(2, 1), "card")).toEqual({ method: "card", kind: "reward" });
  expect(initialReviewPayment(summary(2, 1), "normal")).toEqual({ method: "tickets", kind: "normal" });
  expect(initialReviewPayment(summary(2, 1), "reward")).toEqual({ method: "tickets", kind: "reward" });
  expect(initialReviewPayment(summary(2, 0), "reward")).toEqual({ method: "tickets", kind: "normal" });
  expect(initialReviewPayment(summary(0, 1), "normal")).toEqual({ method: "tickets", kind: "reward" });
  expect(initialReviewPayment(summary(0, 0), "normal")).toEqual({ method: "card", kind: null });
  // 取得に失敗したら単発。
  expect(initialReviewPayment(null, "reward")).toEqual({ method: "card", kind: null });
});

it("種別のない古い API では内訳を出さず、従来どおり残りがあればチケット", () => {
  const old = { balance: { available: 2, reserved: 0 } };
  expect(ticketKindCounts(old)).toBeNull();
  expect(initialReviewPayment(old, "reward")).toEqual({ method: "tickets", kind: null });
  expect(initialReviewPayment(old, "card")).toEqual({ method: "card", kind: null });
});

it("確保される1枚の期限は、選んだ種別の中の最速。種別なしは全体の最速", () => {
  expect(ticketExpiryFor(summary(2, 1), "normal")).toBe("2027-03-01T00:00:00Z");
  expect(ticketExpiryFor(summary(2, 1), "reward")).toBe("2027-01-01T00:00:00Z");
  expect(ticketExpiryFor(summary(2, 1), null)).toBe("2027-01-01T00:00:00Z");
  expect(ticketExpiryFor(summary(0, 1), "normal")).toBeNull();
});
