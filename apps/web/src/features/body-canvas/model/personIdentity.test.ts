import { describe, expect, it } from "vitest";
import { emailLocalPart, personDisplayName } from "./personIdentity";

describe("personDisplayName", () => {
  it("prefers the Profile nickname", () => {
    expect(
      personDisplayName({ nickname: " BakerSean ", accountEmail: "b@x.test" }),
    ).toEqual({ name: "BakerSean", source: "profile" });
  });

  it("falls back to the email local part, not the whole address", () => {
    expect(
      personDisplayName({ nickname: "  ", accountEmail: "baker.sean@x.test" }),
    ).toEqual({ name: "baker.sean", source: "account" });
  });

  it("uses a placeholder when neither exists", () => {
    expect(personDisplayName({ nickname: null, accountEmail: "" })).toEqual({
      name: "未设置昵称",
      source: "placeholder",
    });
  });

  it("extracts the local part defensively", () => {
    expect(emailLocalPart("a@b.c")).toBe("a");
    expect(emailLocalPart("@b.c")).toBe("@b.c");
    expect(emailLocalPart("no-at")).toBe("no-at");
    expect(emailLocalPart(undefined)).toBe("");
  });
});
