import { afterEach, describe, expect, it } from "vitest";
import { surfaceAllowed } from "./proxy";

const originalPublic = process.env.PUBLIC_HOSTNAME;
const originalAdmin = process.env.ADMIN_HOSTNAME;

afterEach(() => {
  process.env.PUBLIC_HOSTNAME = originalPublic;
  process.env.ADMIN_HOSTNAME = originalAdmin;
});

describe("surfaceAllowed", () => {
  it("blocks admin pages on the public host", () => {
    process.env.PUBLIC_HOSTNAME = "www.example.test";
    process.env.ADMIN_HOSTNAME = "admin.example.test";
    expect(surfaceAllowed("www.example.test", "/admin")).toBe(false);
    expect(surfaceAllowed("www.example.test", "/admin/users")).toBe(false);
    expect(surfaceAllowed("www.example.test", "/produtos")).toBe(true);
  });

  it("exposes only admin pages and assets on the admin host", () => {
    process.env.PUBLIC_HOSTNAME = "www.example.test";
    process.env.ADMIN_HOSTNAME = "admin.example.test";
    expect(surfaceAllowed("admin.example.test", "/admin/orcamentos")).toBe(true);
    expect(surfaceAllowed("admin.example.test", "/_next/static/app.js")).toBe(true);
    expect(surfaceAllowed("admin.example.test", "/minha-conta")).toBe(false);
  });
});
