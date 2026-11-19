export type ProductionRoute = {
  id: string;
  label: string;
  path: string;
  heading: string;
};

export const productionRoutes: readonly ProductionRoute[] = [
  { id: "production-dashboard", label: "Produção", path: "/admin/producao", heading: "Produção 3D" },
  { id: "production-planning", label: "Planejamento", path: "/admin/producao/planejamento", heading: "Planejamento da produção" },
  { id: "production-optimization", label: "Otimização", path: "/admin/producao/otimizacao", heading: "Otimização da produção" },
  { id: "production-printers", label: "Impressoras", path: "/admin/producao/impressoras", heading: "Impressoras" },
  { id: "production-materials", label: "Materiais", path: "/admin/producao/materiais", heading: "Materiais" },
  { id: "production-compatibility", label: "Compatibilidade", path: "/admin/producao/compatibilidade", heading: "Compatibilidade técnica" },
  { id: "production-history", label: "Histórico", path: "/admin/producao/historico", heading: "Histórico de impressões" },
  { id: "production-performance", label: "Desempenho", path: "/admin/producao/desempenho", heading: "Desempenho da produção" },
  { id: "costs-overview", label: "Custos", path: "/admin/producao/custos", heading: "Custos de fabricação" },
  { id: "costs-printer-profiles", label: "Perfis de custos", path: "/admin/producao/custos/impressoras", heading: "Custos por impressora" },
  { id: "costs-executions", label: "Execuções de custos", path: "/admin/producao/custos/execucoes", heading: "Custos por execução" },
  { id: "costs-profitability", label: "Rentabilidade", path: "/admin/producao/custos/rentabilidade", heading: "Rentabilidade" },
];

export const dynamicProductionRoutes = [
  {
    id: "production-job-detail",
    label: "Detalhe de job (ID real via fila)",
    sourcePath: "/admin/producao",
    sourceHeading: "Produção 3D",
    linkName: "Detalhes",
    pathPrefix: "/admin/producao/jobs/",
    headingLevel: 1,
  },
  {
    id: "production-run-detail",
    label: "Detalhe de execução (ID real via histórico)",
    sourcePath: "/admin/producao/historico",
    sourceHeading: "Histórico de impressões",
    linkName: "Detalhes",
    pathPrefix: "/admin/producao/historico/",
    headingLevel: 1,
  },
] as const;

export const productionViewports = [
  { id: "desktop", label: "Desktop", width: 1440, height: 1200 },
  { id: "tablet", label: "Tablet", width: 820, height: 1180 },
  { id: "mobile", label: "Mobile", width: 390, height: 844 },
  { id: "mobile-compact", label: "Mobile compacto", width: 320, height: 720 },
] as const;
