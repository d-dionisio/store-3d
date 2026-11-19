import { mkdir } from "node:fs/promises";
import { homedir } from "node:os";
import path from "node:path";
import { chromium } from "@playwright/test";

const baseURL = process.env.PW_BASE_URL ?? "http://localhost:3000";
const authStatePath = path.resolve(
  process.env.PW_AUTH_STATE ??
    (process.env.LOCALAPPDATA
      ? path.join(process.env.LOCALAPPDATA, "Formo3D", "playwright", "admin.json")
      : path.join(homedir(), ".local", "share", "formo3d", "playwright", "admin.json")),
);
const timeoutMs = Number(process.env.PW_AUTH_TIMEOUT_MS ?? 600_000);

if (!Number.isSafeInteger(timeoutMs) || timeoutMs < 60_000) {
  throw new Error("PW_AUTH_TIMEOUT_MS deve ser um inteiro de pelo menos 60000 ms.");
}

const base = new URL(baseURL);
if (base.username || base.password || base.search || base.hash) {
  throw new Error("PW_BASE_URL deve conter somente origem e caminho, sem credenciais ou parâmetros.");
}

const browser = await chromium.launch({ headless: false });
try {
  const context = await browser.newContext({ viewport: { width: 1440, height: 1200 } });
  const page = await context.newPage();
  await page.goto(new URL("/admin/login", base).toString(), { waitUntil: "domcontentloaded" });
  console.log("Conclua manualmente o login e o MFA na janela Chromium. Nenhum dado digitado será lido pelo script.");

  await page.waitForURL(url => url.pathname !== "/admin/login", { timeout: timeoutMs });
  await page.goto(new URL("/admin/producao", base).toString(), { waitUntil: "domcontentloaded" });
  if (new URL(page.url()).pathname === "/admin/login") {
    throw new Error("A sessão não foi aceita ou expirou; nenhuma sessão foi salva.");
  }
  await page.getByRole("heading", { name: "Produção 3D", exact: true }).waitFor({ state: "visible" });

  await mkdir(path.dirname(authStatePath), { recursive: true });
  await context.storageState({ path: authStatePath });
  console.log("Sessão autenticada salva pelo storageState do Playwright em armazenamento local; conteúdo omitido.");
  console.log("A janela Chromium será fechada. Não compartilhe o arquivo de sessão nem traces/relatórios.");
} finally {
  await browser.close();
}
