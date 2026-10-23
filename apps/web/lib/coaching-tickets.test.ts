import { expect, it } from "vitest";
import { ticketReturnPath } from "./coaching-tickets";

it("チケット購入の戻り先で外部URL・保護者トークン・別機能への転送を許可しない", () => {
  const request = "/coaching/requests/12345678-1234-1234-1234-123456789abc";
  expect(ticketReturnPath(request)).toBe(request);
  expect(ticketReturnPath("/coaching/new")).toBe("/coaching/new");
  for (const path of ["https://evil.test", "//evil.test", "/\\evil.test", "/guardian/secret", "/coaching/requests/not-a-uuid", request + "?next=https://evil.test", "/coaching/new\n", "/admin"]) {
    expect(ticketReturnPath(path)).toBe("/me/coaching/tickets");
  }
});
