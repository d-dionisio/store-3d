import { mkdir, existsSync } from "node:fs";
import path from "node:path";
import { promisify } from "node:util";
import { expect, test } from "@playwright/test";

const mkdirAsync = promisify(mkdir);
const runId = process.env.PW_RUN_ID;
if (!runId) throw new Error("PW_RUN_ID não foi inicializado pela configuração Playwright.");
const screenshotPath = path.resolve(
  process.env.PW_SCREENSHOT_PATH ??
    path.join(process.cwd(), "test-results", "visual-homologation", "runs", runId, "screenshots", "desktop", "production-dashboard.png"),
);

test("dashboard de produção abre com sessão administrativa legítima", async ({ page }, testInfo) => {
  let pageErrors = 0;
  let failedRequests = 0;
  const serverErrors: number[] = [];

  page.on("pageerror", () => { pageErrors += 1; });
  page.on("console", message => {
    if (message.type() === "error") pageErrors += 1;
  });
  page.on("requestfailed", () => { failedRequests += 1; });
  page.on("response", response => {
    if (response.status() >= 500) serverErrors.push(response.status());
  });

  await page.goto("/admin/producao", { waitUntil: "domcontentloaded" });
  if (new URL(page.url()).pathname === "/admin/login") {
    throw new Error("Sessão ausente, expirada ou revogada. Execute npm run test:e2e:auth e conclua o MFA manualmente.");
  }

  await expect(page).toHaveURL(/\/admin\/producao\/?$/);
  await expect(page.getByRole("heading", { name: "Produção 3D", exact: true })).toBeVisible();
  expect(serverErrors, "A página não deve receber respostas HTTP 500 ou superiores.").toEqual([]);
  expect(pageErrors, "A página não deve emitir erros JavaScript.").toBe(0);

  testInfo.annotations.push({
    type: "diagnóstico sanitizado",
    description: `HTTP 5xx: ${serverErrors.length}; erros JS/console: ${pageErrors}; falhas de requisição: ${failedRequests}. URLs, headers e corpos foram omitidos.`,
  });

  if (existsSync(screenshotPath)) {
    throw new Error("A captura de destino já existe. Defina um PW_SCREENSHOT_PATH novo; evidências anteriores não são sobrescritas.");
  }
  await mkdirAsync(path.dirname(screenshotPath), { recursive: true });
  await page.screenshot({ path: screenshotPath, fullPage: true, animations: "disabled" });
  testInfo.annotations.push({ type: "captura", description: "Screenshot real do dashboard salva localmente; pode conter informações administrativas." });
});
