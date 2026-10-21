import { describe, expect, it } from "vitest";
import { callbackErrorMessages, callbackExchangeError, callbackParamsError } from "./auth-callback";

describe("callbackParamsError", () => {
  it("goes on to the exchange when only a code is present", () => {
    expect(callbackParamsError(new URLSearchParams("code=abc"))).toBeNull();
  });

  it("tells an expired email link apart from a Google failure", () => {
    expect(callbackParamsError(new URLSearchParams("error=access_denied&error_code=otp_expired&error_description=Email+link+is+invalid"))).toBe(
      "link-expired",
    );
    expect(callbackParamsError(new URLSearchParams("error=access_denied&error_description=cancelled"))).toBe("google");
    expect(callbackParamsError(new URLSearchParams(""))).toBe("google");
  });

  it("has a message for every kind", () => {
    for (const kind of ["google", "link-expired", callbackExchangeError]) {
      expect(callbackErrorMessages[kind]).toBeTruthy();
    }
  });
});
