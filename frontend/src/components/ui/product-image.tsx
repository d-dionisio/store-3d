import type { Product } from "@/types/product";

const positions = {
  "top-left": "0% 0%",
  "top-right": "100% 0%",
  "bottom-left": "0% 100%",
  "bottom-right": "100% 100%",
};

export function ProductImage({ product, className = "" }: { product: Product; className?: string }) {
  return (
    <div
      role="img"
      aria-label={`Imagem demonstrativa de ${product.name}`}
      className={`product-image ${className}`}
      style={{ backgroundPosition: positions[product.imagePosition] }}
    />
  );
}

