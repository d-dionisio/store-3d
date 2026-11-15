import type{JobRecommendation,OptimizationStrategy}from"@/services/production-api";
export const strategyLabels:Record<OptimizationStrategy,string>={fifo:"FIFO",shortest_processing_time:"Menor tempo",earliest_due_date:"Menor prazo",material_color_grouping:"Material e cor",combined:"Combinada"};
export const durationLabel=(value:number|null)=>value===null?"Dados insuficientes":`${Math.floor(value/60)}h ${value%60}min`;
export const recommendationLabel=(job:JobRecommendation)=>job.recommended?`${job.recommended.printer_name} · ${job.recommended.score} pts`:"Nenhuma impressora recomendável.";
