import { existsSync } from "node:fs";
import { randomUUID } from "node:crypto";
import { homedir } from "node:os";
import path from "node:path";
import { defineConfig, devices } from "@playwright/test";

const baseURL = process.env.PW_BASE_URL ?? "http://localhost:3000";
const parsedBaseURL = new URL(baseURL);
if (parsedBaseURL.username || parsedBaseURL.password || parsedBaseURL.search || parsedBaseURL.hash) {
  throw new Error("PW_BASE_URL deve conter apenas a origem e, opcionalmente, o caminho base.");
}
const visualRunId = `${new Date().toISOString().replaceAll(":", "-")}-${randomUUID()}`;
process.env.PW_RUN_ID = visualRunId;
const authStatePath = path.resolve(
  process.env.PW_AUTH_STATE ??
    (process.env.LOCALAPPDATA
      ? path.join(process.env.LOCALAPPDATA, "Formo3D", "playwright", "admin.json")
      : path.join(homedir(), ".local", "share", "formo3d", "playwright", "admin.json")),
);

export default defineConfig({
  testDir: "./playwright/tests",
  globalSetup: "./playwright/global-setup.ts",
  fullyParallel: false,
  workers: 1,
  forbidOnly: Boolean(process.env.CI),
  retries: 0,
  timeout: 90_000,
  expect: { timeout: 15_000 },
  outputDir: path.join("test-results", "playwright-run-artifacts", visualRunId),
  reporter: [
    ["list"],
    ["html", { outputFolder: path.join("playwright-report", "visual-homologation-runs", visualRunId), open: "never" }],
    ["./playwright/reporters/visual-summary-reporter.ts"],
  ],
  use: {
    ...devices["Desktop Chrome"],
    browserName: "chromium",
    baseURL,
    storageState: existsSync(authStatePath) ? authStatePath : undefined,
    viewport: { width: 1440, height: 1200 },
    deviceScaleFactor: 1,
    actionTimeout: 15_000,
    navigationTimeout: 30_000,
    screenshot: "only-on-failure",
    trace: "retain-on-failure",
    video: "off",
    ignoreHTTPSErrors: false,
  },
});
