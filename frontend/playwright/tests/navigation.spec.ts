import { expect, test } from "@playwright/test";
import { productionViewports } from "../fixtures/production-routes";
import { assertNoServerErrors, annotateBrowserVersion, annotateDiagnostics, monitorPage } from "../helpers/diagnostics";
import { pageUnavailableReason, waitForPageOutcome } from "../helpers/availability";

async function skipIfUnavailable(page: import("@playwright/test").Page, testInfo: import("@playwright/test").TestInfo, expectedHeading?: string): Promise<void> {
  await waitForPageOutcome(page, expectedHeading);
  const unavailable = await pageUnavailableReason(page);
  if (unavailable) {
    testInfo.annotations.push({ type: "skip", description: unavailable });
    test.skip(true, unavailable);
  }
}

for (const viewport of productionViewports) {
  test(`navigation | production-costs-history | ${viewport.id}`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width: viewport.width, height: viewport.height });
    annotateBrowserVersion(page, testInfo);
    const diagnostics = monitorPage(page);
    await page.goto("/admin/producao", { waitUntil: "domcontentloaded" });
    if (new URL(page.url()).pathname === "/admin/login") {
      throw new Error("A sessão não foi aceita. Interrompa a homologação e renove-a manualmente pelo MFA.");
    }
    await skipIfUnavailable(page, testInfo, "Produção 3D");
    await page.getByRole("link", { name: "Custos de fabricação", exact: true }).click();
    await skipIfUnavailable(page, testInfo, "Custos de fabricação");
    await expect(page.getByRole("heading", { level: 1, name: "Custos de fabricação", exact: true })).toBeVisible();
    await page.getByRole("link", { name: "Perfis por impressora", exact: true }).click();
    await skipIfUnavailable(page, testInfo, "Custos por impressora");
    await expect(page.getByRole("heading", { level: 1, name: "Custos por impressora", exact: true })).toBeVisible();
    await page.getByRole("link", { name: "Execuções", exact: true }).click();
    await skipIfUnavailable(page, testInfo, "Custos por execução");
    await expect(page.getByRole("heading", { level: 1, name: "Custos por execução", exact: true })).toBeVisible();
    await page.getByRole("link", { name: "Rentabilidade", exact: true }).click();
    await skipIfUnavailable(page, testInfo, "Rentabilidade");
    await expect(page.getByRole("heading", { level: 1, name: "Rentabilidade", exact: true })).toBeVisible();
    await page.getByRole("link", { name: "Produção", exact: true }).click();
    await skipIfUnavailable(page, testInfo, "Produção 3D");
    await expect(page.getByRole("heading", { level: 1, name: "Produção 3D", exact: true })).toBeVisible();
    await page.getByRole("link", { name: "Histórico", exact: true }).click();
    await skipIfUnavailable(page, testInfo, "Histórico de impressões");
    await expect(page.getByRole("heading", { level: 1, name: "Histórico de impressões", exact: true })).toBeVisible();
    await page.getByRole("link", { name: "Desempenho", exact: true }).click();
    await skipIfUnavailable(page, testInfo, "Desempenho da produção");
    await expect(page.getByRole("heading", { level: 1, name: "Desempenho da produção", exact: true })).toBeVisible();
    assertNoServerErrors(diagnostics);
    annotateDiagnostics(testInfo, diagnostics);
  });
}
