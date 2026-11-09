export type ProductCategory = "Decoração" | "Natal" | "Religiosos" | "Acessórios" | "Personalizados";

export type Product = {
  id: string;
  slug: string;
  name: string;
  description: string;
  price: number;
  category: ProductCategory;
  colors: string[];
  dimensions: string;
  material: string;
  leadTime: string;
  imagePosition: "top-left" | "top-right" | "bottom-left" | "bottom-right";
  featured?: boolean;
};

export type CartItem = {
  productId: string;
  color: string;
  quantity: number;
};

