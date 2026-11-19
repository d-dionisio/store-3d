import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./playwright/tests",
  testMatch: "admin-login-mfa-isolation.spec.ts",
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 30_000,
  expect: { timeout: 10_000 },
  outputDir: "test-results/admin-mfa-isolation",
  reporter: "list",
  use: {
    ...devices["Desktop Chrome"],
    browserName: "chromium",
    baseURL: process.env.PW_BASE_URL ?? "http://localhost:3000",
    screenshot: "off",
    trace: "off",
    video: "off",
  },
});
