import { existsSync } from "node:fs";
import { homedir } from "node:os";
import path from "node:path";
import { chromium, type FullConfig } from "@playwright/test";

export default async function globalSetup(config: FullConfig): Promise<void> {
  const project = config.projects[0];
  const baseURL = String(project.use.baseURL ?? "http://localhost:3000");
  const authStatePath = path.resolve(
    process.env.PW_AUTH_STATE ??
      (process.env.LOCALAPPDATA
        ? path.join(process.env.LOCALAPPDATA, "Formo3D", "playwright", "admin.json")
        : path.join(homedir(), ".local", "share", "formo3d", "playwright", "admin.json")),
  );
  if (!existsSync(authStatePath)) {
    throw new Error("Estado Playwright ausente. Execute npm run test:e2e:auth e conclua o MFA manualmente; nenhum login automático foi tentado.");
  }

  const browser = await chromium.launch({ headless: true });
  try {
    const context = await browser.newContext({ storageState: authStatePath });
    const page = await context.newPage();
    try {
      await page.goto(new URL("/admin/producao", baseURL).toString(), { waitUntil: "domcontentloaded", timeout: 20_000 });
      if (new URL(page.url()).pathname === "/admin/login") {
        throw new Error("Sessão expirada ou revogada.");
      }
      await page.getByRole("heading", { name: "Produção 3D", exact: true }).waitFor({ state: "visible", timeout: 15_000 });
    } catch {
      throw new Error("Sessão administrativa não validada no dashboard. Interrompa e execute npm run test:e2e:auth para autenticar manualmente com MFA.");
    } finally {
      await context.close();
    }
  } finally {
    await browser.close();
  }
}
