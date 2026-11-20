import { mkdir, readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import type { FullConfig, FullResult, Reporter, Suite, TestCase, TestResult } from "@playwright/test/reporter";
import { dynamicProductionRoutes, productionRoutes, productionViewports } from "../fixtures/production-routes";

type VisualRecord = {
  kind: string;
  route: string;
  viewport: string;
  status: string;
  durationMs: number;
  errorCount: number;
  screenshot: string | null;
  diagnostics: string | null;
  layout: string | null;
  keyboard: string | null;
  browserVersion: string | null;
  blockedReason: string | null;
};

const routeLabels: Record<string, string> = {
  "production-dashboard": "Produção",
  "production-planning": "Planejamento",
  "production-optimization": "Otimização",
  "production-printers": "Impressoras",
  "production-materials": "Materiais",
  "production-compatibility": "Compatibilidade",
  "production-history": "Histórico",
  "production-performance": "Desempenho",
  "costs-overview": "Custos",
  "costs-printer-profiles": "Perfis de custos",
  "costs-executions": "Execuções de custos",
  "costs-profitability": "Rentabilidade",
  ...Object.fromEntries(dynamicProductionRoutes.map(route => [route.id, route.label])),
};
const viewportLabels: Record<string, string> = {
  desktop: "Desktop 1440×1200",
  tablet: "Tablet 820×1180",
  mobile: "Mobile 390×844",
  "mobile-compact": "Mobile compacto 320×720",
};

export default class VisualSummaryReporter implements Reporter {
  private startedAt = new Date();
  private planned = 0;
  private playwrightVersion = "desconhecida";
  private browserName = "chromium";
  private baseOrigin = "não informado";
  private records: VisualRecord[] = [];

  async onBegin(config: FullConfig, suite: Suite): Promise<void> {
    this.startedAt = new Date();
    this.planned = suite.allTests().length;
    this.browserName = String(config.projects[0]?.use.browserName ?? "chromium");
    const configuredBase = String(config.projects[0]?.use.baseURL ?? "http://localhost:3000");
    try {
      this.baseOrigin = new URL(configuredBase).origin;
    } catch {
      this.baseOrigin = "inválida";
    }
    try {
      const packageFile = path.resolve(process.cwd(), "node_modules", "@playwright", "test", "package.json");
      const packageInfo = JSON.parse(await readFile(packageFile, "utf8")) as { version?: string };
      this.playwrightVersion = packageInfo.version ?? "desconhecida";
    } catch {
      this.playwrightVersion = "desconhecida";
    }
  }

  onTestEnd(test: TestCase, result: TestResult): void {
    const [kind = "test", route = "unknown", viewport = "unknown"] = test.title.split(" | ").map(value => value.trim());
    const annotation = (type: string) => result.annotations.find(item => item.type === type)?.description ?? null;
    const outcome = result.status === "passed" ? "APROVADO"
      : result.status === "skipped" ? "BLOQUEADO"
        : result.status === "interrupted" ? "NÃO EXECUTADO" : "REPROVADO";
    this.records.push({
      kind,
      route,
      viewport,
      status: outcome,
      durationMs: result.duration,
      errorCount: result.errors.length,
      screenshot: annotation("screenshot"),
      diagnostics: annotation("diagnóstico sanitizado"),
      layout: annotation("layout"),
      keyboard: annotation("keyboard"),
      browserVersion: annotation("browser-version"),
      blockedReason: annotation("skip"),
    });
  }

  async onEnd(result: FullResult): Promise<void> {
    const finishedAt = new Date();
    const runId = process.env.PW_RUN_ID;
    if (!runId) throw new Error("PW_RUN_ID não foi inicializado pela configuração Playwright.");
    const folder = path.resolve(process.cwd(), "test-results", "visual-homologation", "runs", runId);
    await mkdir(folder, { recursive: true });
    const approved = this.records.filter(record => record.status === "APROVADO").length;
    const rejected = this.records.filter(record => record.status === "REPROVADO").length;
    const blocked = this.records.filter(record => record.status === "BLOQUEADO").length;
    const notExecuted = Math.max(0, this.planned - this.records.length) + this.records.filter(record => record.status === "NÃO EXECUTADO").length;
    const counts = {
      planned: this.planned,
      executed: this.records.length,
      evaluated: approved + rejected,
      approved,
      rejected,
      blocked,
      notExecuted,
    };
    const report = {
      generatedAt: finishedAt.toISOString(),
      durationMs: finishedAt.getTime() - this.startedAt.getTime(),
      playwright: this.playwrightVersion,
      browser: this.browserName,
      browserVersion: this.records.find(record => record.browserVersion)?.browserVersion ?? "global setup validou Chromium; versão não capturada nos casos executados",
      baseOrigin: this.baseOrigin,
      authentication: "storageState local presente; global setup validou acesso ao dashboard; credenciais não lidas",
      status: result.status,
      counts,
      plannedCombinations: {
        staticRoutes: productionRoutes.length,
        staticPageViewports: productionRoutes.length * productionViewports.length,
        dynamicRoutes: dynamicProductionRoutes.length,
        dynamicRouteViewports: dynamicProductionRoutes.length * productionViewports.length,
        navigationChecks: productionViewports.length,
        total: (productionRoutes.length + dynamicProductionRoutes.length + 1) * productionViewports.length,
      },
      tests: this.records,
      note: "Nenhuma URL completa, cookie, header, corpo ou mensagem de erro foi incluída. IDs dinâmicos não são registrados.",
    };
    await writeFile(path.join(folder, "report.json"), `${JSON.stringify(report, null, 2)}\n`, "utf8");
    await writeFile(path.join(folder, "production-visual-matrix.md"), this.renderMatrix(report, finishedAt, counts), "utf8");
  }

  private renderMatrix(report: { playwright: string; browser: string; browserVersion: string; baseOrigin: string; status: string }, at: Date, counts: { planned: number; executed: number; evaluated: number; approved: number; rejected: number; blocked: number; notExecuted: number }): string {
    const devices = ["desktop", "tablet", "mobile", "mobile-compact"];
    const routes = Object.keys(routeLabels);
    const row = (route: string) => {
      const cells = devices.map(device => {
        const item = this.records.find(record => record.route === route && record.viewport === device);
        if (!item) return "NÃO EXECUTADO";
        const link = item.screenshot ? ` [captura](../../frontend/${item.screenshot})` : "";
        return `${item.status}${link}`;
      });
      return `| ${routeLabels[route]} | ${cells.join(" | ")} |`;
    };
    const nav = this.records.filter(record => record.kind === "navigation");
    return [
      "# Matriz de homologação visual — produção e custos",
      "",
      `- Gerado em: ${at.toISOString()}`,
      `- Playwright: ${report.playwright}; navegador: ${report.browser} ${report.browserVersion}`,
      `- Origem: ${report.baseOrigin}`,
      "- Autenticação: arquivo storageState local validado pelo global setup; senha/MFA/cookies não foram lidos ou publicados.",
      `- Resultado geral: ${report.status}`,
      `- Casos: planejados ${counts.planned}; executados ${counts.executed}; avaliados integralmente ${counts.evaluated}; aprovados ${counts.approved}; reprovados ${counts.rejected}; bloqueados ${counts.blocked}; não executados ${counts.notExecuted}.`,
      `- Combinações previstas: ${productionRoutes.length} rotas estáticas × ${productionViewports.length}, ${dynamicProductionRoutes.length} rotas dinâmicas × ${productionViewports.length} e ${productionViewports.length} verificações de navegação (${(productionRoutes.length + dynamicProductionRoutes.length + 1) * productionViewports.length} testes).`,
      "",
      "## Inventário de rotas",
      "",
      ...productionRoutes.map(route => `- Estática: \`${route.path}\` — ${route.label}.`),
      ...dynamicProductionRoutes.map(route => `- Dinâmica: \`${route.pathPrefix}[id]\` — ${route.label}; origem: \`${route.sourcePath}\`, usando link real existente.`),
      "",
      "| Página | Desktop 1440×1200 | Tablet 820×1180 | Mobile 390×844 | Compacto 320×720 |",
      "|---|---|---|---|---|",
      ...routes.map(row),
      "",
      "## Navegação",
      "",
      ...devices.map(device => {
        const item = nav.find(record => record.viewport === device);
        return `- ${viewportLabels[device]}: ${item?.status ?? "NÃO EXECUTADO"}`;
      }),
      "",
      "## Problemas e limitações",
      "",
      ...this.records.filter(record => record.status !== "APROVADO").map(record => `- ${record.status}: ${record.route} / ${viewportLabels[record.viewport] ?? record.viewport}; erros=${record.errorCount}; ${record.blockedReason ?? "ver HTML report; mensagens e URLs sensíveis foram omitidas"}.`),
      ...(this.records.every(record => record.status === "APROVADO") ? ["- Nenhuma falha automatizada registrada. A inspeção humana das capturas continua necessária; screenshot isolada não comprova funcionalidade completa."] : []),
      "",
      `Capturas, relatório JSON e matriz desta execução ficam em \`frontend/test-results/visual-homologation/runs/${process.env.PW_RUN_ID}/\`; o relatório HTML e traces ficam em pastas exclusivas desta execução. Os artefatos anteriores não são sobrescritos.`,
      "",
    ].join("\n");
  }
}
