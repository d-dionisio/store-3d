import { expect, test } from "@playwright/test";
import { dynamicProductionRoutes, productionRoutes, productionViewports } from "../fixtures/production-routes";
import { assertNoServerErrors, annotateBrowserVersion, annotateDiagnostics, monitorPage } from "../helpers/diagnostics";
import { pageUnavailableReason, waitForPageOutcome } from "../helpers/availability";
import { saveVisualScreenshot, verifyKeyboardFocus, verifyResponsiveLayout } from "../helpers/responsive";

test.afterEach(async ({ page }, testInfo) => {
  if (testInfo.status === "failed") {
    const [kind, routeId, viewportId] = testInfo.title.split(" | ");
    if ((kind === "static" || kind === "dynamic") && routeId && viewportId && !page.isClosed()) {
      await saveVisualScreenshot(page, testInfo, routeId, viewportId);
    }
  }
});

for (const route of productionRoutes) {
  for (const viewport of productionViewports) {
    test(`static | ${route.id} | ${viewport.id}`, async ({ page }, testInfo) => {
      await page.setViewportSize({ width: viewport.width, height: viewport.height });
      annotateBrowserVersion(page, testInfo);
      const diagnostics = monitorPage(page);
      await page.goto(route.path, { waitUntil: "domcontentloaded" });
      if (new URL(page.url()).pathname === "/admin/login") {
        throw new Error("A sessão não foi aceita. Interrompa a homologação e renove-a manualmente pelo MFA.");
      }
      await expect(page).toHaveURL(new RegExp(`${route.path.replaceAll("/", "\\/")}\\/?$`));
      await waitForPageOutcome(page, route.heading);
      const unavailable = await pageUnavailableReason(page);
      if (unavailable) {
        testInfo.annotations.push({ type: "skip", description: unavailable });
        annotateDiagnostics(testInfo, diagnostics);
        await saveVisualScreenshot(page, testInfo, route.id, viewport.id);
        test.skip(true, unavailable);
      }
      await expect(page.getByRole("main")).toBeVisible();
      await expect(page.getByRole("heading", { level: 1, name: route.heading, exact: true })).toBeVisible();
      await expect(page.getByRole("navigation").first()).toBeVisible();
      await page.evaluate(() => document.fonts.ready);
      await verifyResponsiveLayout(page, testInfo);
      await verifyKeyboardFocus(page, testInfo);
      assertNoServerErrors(diagnostics);
      annotateDiagnostics(testInfo, diagnostics);
      await saveVisualScreenshot(page, testInfo, route.id, viewport.id);
    });
  }
}

for (const route of dynamicProductionRoutes) {
  for (const viewport of productionViewports) {
    test(`dynamic | ${route.id} | ${viewport.id}`, async ({ page }, testInfo) => {
      await page.setViewportSize({ width: viewport.width, height: viewport.height });
      annotateBrowserVersion(page, testInfo);
      const diagnostics = monitorPage(page);
      await page.goto(route.sourcePath, { waitUntil: "domcontentloaded" });
      if (new URL(page.url()).pathname === "/admin/login") {
        throw new Error("A sessão não foi aceita. Interrompa a homologação e renove-a manualmente pelo MFA.");
      }
      await waitForPageOutcome(page, route.sourceHeading);
      const unavailable = await pageUnavailableReason(page);
      if (unavailable) {
        testInfo.annotations.push({ type: "skip", description: unavailable });
        annotateDiagnostics(testInfo, diagnostics);
        await saveVisualScreenshot(page, testInfo, route.id, viewport.id);
        test.skip(true, unavailable);
      }
      const detailLink = page.getByRole("link", { name: route.linkName, exact: true }).first();
      if (await detailLink.count() === 0) {
        const reason = "Nenhum registro real disponível na lista; nenhum ID foi inventado e nenhum dado foi criado.";
        testInfo.annotations.push({ type: "skip", description: reason });
        annotateDiagnostics(testInfo, diagnostics);
        await saveVisualScreenshot(page, testInfo, route.id, viewport.id);
        test.skip(true, reason);
      }
      const href = await detailLink.getAttribute("href");
      if (!href?.startsWith(route.pathPrefix)) {
        throw new Error("O link de detalhe existente não corresponde à rota dinâmica esperada.");
      }
      await detailLink.click();
      const escapedPrefix = route.pathPrefix.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
      await expect(page).toHaveURL(new RegExp(`${escapedPrefix}[^/]+/?$`));
      const loadingHeading = page.getByRole("heading", { name: /^Carregando (produção|execução)/ });
      try {
        await loadingHeading.waitFor({ state: "hidden", timeout: 15_000 });
      } catch {
        const reason = "A rota dinâmica permaneceu no estado de carregamento; detalhe real indisponível para inspeção.";
        testInfo.annotations.push({ type: "skip", description: reason });
        annotateDiagnostics(testInfo, diagnostics);
        await saveVisualScreenshot(page, testInfo, route.id, viewport.id);
        test.skip(true, reason);
      }
      await expect(page.getByRole("main")).toBeVisible();
      await expect(page.getByRole("heading", { level: route.headingLevel })).toBeVisible();
      await page.evaluate(() => document.fonts.ready);
      await verifyResponsiveLayout(page, testInfo);
      await verifyKeyboardFocus(page, testInfo);
      assertNoServerErrors(diagnostics);
      annotateDiagnostics(testInfo, diagnostics);
      await saveVisualScreenshot(page, testInfo, route.id, viewport.id);
    });
  }
}
