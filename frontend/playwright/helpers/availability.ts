import type { Page } from "@playwright/test";

export async function waitForPageOutcome(page: Page, expectedHeading?: string): Promise<void> {
  // Admin pages may stream their header before the server/API result. Wait for
  // the route's initial requests to settle before classifying an error state.
  await page.waitForLoadState("networkidle", { timeout: 5_000 }).catch(() => undefined);
  await page.waitForFunction((expected) => {
    const headings = [...document.querySelectorAll("h1,h2")].map(item => item.textContent?.trim() ?? "");
    return headings.includes("Produção indisponível") || headings.includes("Relatórios indisponíveis") ||
      headings.includes("Impressoras indisponíveis") || headings.includes("Histórico indisponível") ||
      headings.includes("Acesso não autorizado") || Boolean(expected && headings.includes(expected));
  }, expectedHeading, { timeout: 20_000 });
}

/** Returns only a safe category; never copies server-provided error details into reports. */
export async function pageUnavailableReason(page: Page): Promise<string | null> {
  return page.evaluate(() => {
    const visible = (element: Element) => {
      const style = getComputedStyle(element);
      const rect = element.getBoundingClientRect();
      return style.display !== "none" && style.visibility !== "hidden" && Number(style.opacity) > 0 && rect.width > 0 && rect.height > 0;
    };
    const headings = [...document.querySelectorAll("h1,h2")].filter(visible).map(element => element.textContent?.trim() ?? "");
    if (headings.includes("Produção indisponível")) return "A interface exibiu o estado de indisponibilidade da API operacional.";
    if (headings.includes("Relatórios indisponíveis") || headings.includes("Impressoras indisponíveis") || headings.includes("Histórico indisponível")) {
      return "A interface exibiu o estado de indisponibilidade dos relatórios ou recursos.";
    }
    if (headings.includes("Acesso não autorizado")) return "A interface exibiu estado de autorização negada.";
    return null;
  });
}
