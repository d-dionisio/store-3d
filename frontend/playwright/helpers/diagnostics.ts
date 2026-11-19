import type { Page, TestInfo } from "@playwright/test";

export type PageDiagnostics = {
  javascriptErrors: number;
  failedRequests: number;
  httpStatuses: Record<string, number>;
};

export function monitorPage(page: Page): PageDiagnostics {
  const result: PageDiagnostics = { javascriptErrors: 0, failedRequests: 0, httpStatuses: {} };
  page.on("pageerror", () => { result.javascriptErrors += 1; });
  page.on("console", message => {
    if (message.type() === "error") result.javascriptErrors += 1;
  });
  page.on("requestfailed", () => { result.failedRequests += 1; });
  page.on("response", response => {
    const key = String(response.status());
    result.httpStatuses[key] = (result.httpStatuses[key] ?? 0) + 1;
  });
  return result;
}

export function annotateBrowserVersion(page: Page, testInfo: TestInfo): void {
  testInfo.annotations.push({
    type: "browser-version",
    description: page.context().browser()?.version() ?? "desconhecida",
  });
}

export function annotateDiagnostics(testInfo: TestInfo, diagnostics: PageDiagnostics): void {
  const serverErrors = Object.entries(diagnostics.httpStatuses)
    .filter(([status]) => Number(status) >= 500)
    .reduce((total, [, count]) => total + count, 0);
  testInfo.annotations.push({
    type: "diagnóstico sanitizado",
    description: `JS/console=${diagnostics.javascriptErrors}; requisições falhas=${diagnostics.failedRequests}; HTTP=${JSON.stringify(diagnostics.httpStatuses)}; URLs, headers e corpos omitidos.`,
  });
}

export function assertNoServerErrors(diagnostics: PageDiagnostics): void {
  const serverErrors = Object.entries(diagnostics.httpStatuses)
    .filter(([status]) => Number(status) >= 500)
    .reduce((total, [, count]) => total + count, 0);
  if (serverErrors > 0) throw new Error(`Foram observadas ${serverErrors} respostas HTTP 500 ou superiores.`);
  if (diagnostics.javascriptErrors > 0) throw new Error(`Foram observados ${diagnostics.javascriptErrors} erros JavaScript/console.`);
}
