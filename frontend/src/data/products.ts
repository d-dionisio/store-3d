import type { Product, ProductCategory } from "@/types/product";

export const categories: { name: ProductCategory; description: string }[] = [
  { name: "Decoração", description: "Formas autorais para espaços com personalidade." },
  { name: "Natal", description: "Objetos afetivos para celebrar e presentear." },
  { name: "Religiosos", description: "Símbolos produzidos com delicadeza e respeito." },
  { name: "Acessórios", description: "Design funcional para acompanhar sua rotina." },
  { name: "Personalizados", description: "Ideias únicas, modeladas para você." },
];

export const products: Product[] = [
  {
    id: "p1", slug: "vaso-orbita", name: "Vaso Órbita", description: "Um vaso escultural de curvas contínuas, criado para composições contemporâneas.", price: 189, category: "Decoração", colors: ["Marfim", "Areia", "Grafite"], dimensions: "18 × 18 × 24 cm", material: "PLA premium de origem renovável", leadTime: "5 a 8 dias úteis", imagePosition: "top-left", featured: true,
  },
  {
    id: "p2", slug: "presepio-luz", name: "Presépio Luz", description: "Silhueta minimalista para celebrar o Natal com leveza e significado.", price: 149, category: "Religiosos", colors: ["Marfim", "Areia"], dimensions: "22 × 6 × 20 cm", material: "PLA premium fosco", leadTime: "4 a 7 dias úteis", imagePosition: "top-right", featured: true,
  },
  {
    id: "p3", slug: "porta-incenso-faceta", name: "Porta-incenso Faceta", description: "Geometria baixa com desenho preciso para um ritual cotidiano mais sereno.", price: 79, category: "Acessórios", colors: ["Marfim", "Violeta", "Grafite"], dimensions: "15 × 15 × 4 cm", material: "PLA premium fosco", leadTime: "3 a 5 dias úteis", imagePosition: "bottom-left", featured: true,
  },
  {
    id: "p4", slug: "organizador-nexo", name: "Organizador Nexo", description: "Módulos integrados para manter o essencial por perto, sem ruído visual.", price: 129, category: "Acessórios", colors: ["Grafite", "Violeta"], dimensions: "21 × 13 × 12 cm", material: "PETG de alta resistência", leadTime: "4 a 6 dias úteis", imagePosition: "bottom-right", featured: true,
  },
  {
    id: "p5", slug: "vaso-onda", name: "Vaso Onda", description: "Ritmo e textura em uma peça leve, produzida sob demanda.", price: 159, category: "Decoração", colors: ["Areia", "Marfim", "Lilás"], dimensions: "16 × 16 × 21 cm", material: "PLA premium", leadTime: "5 a 8 dias úteis", imagePosition: "top-left",
  },
  {
    id: "p6", slug: "estrela-aurora", name: "Estrela Aurora", description: "Ornamento geométrico para árvore, mesa ou embalagem de presente.", price: 39, category: "Natal", colors: ["Marfim", "Areia", "Violeta"], dimensions: "10 × 2 × 10 cm", material: "PLA premium", leadTime: "3 a 5 dias úteis", imagePosition: "top-right",
  },
  {
    id: "p7", slug: "bandeja-prisma", name: "Bandeja Prisma", description: "Pequeno apoio para joias, chaves e objetos especiais.", price: 69, category: "Decoração", colors: ["Marfim", "Grafite"], dimensions: "16 × 14 × 3 cm", material: "PLA premium", leadTime: "3 a 5 dias úteis", imagePosition: "bottom-left",
  },
  {
    id: "p8", slug: "nexo-personalizado", name: "Nexo Personalizado", description: "Base demonstrativa para um organizador adaptado às suas medidas.", price: 219, category: "Personalizados", colors: ["Grafite", "Violeta", "Marfim"], dimensions: "Sob consulta", material: "PLA ou PETG", leadTime: "Prazo sob consulta", imagePosition: "bottom-right",
  },
];

export const getProduct = (slug: string) => products.find((product) => product.slug === slug);

