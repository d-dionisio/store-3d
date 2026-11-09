"use client";
import Image from "next/image";
import { useState } from "react";
import type { Product, ProductImage as ProductImageType } from "@/types/product";

export function ProductImage({ product, selectedImage, className = "" }: { product: Product; selectedImage?: ProductImageType; className?: string }) {
  const [failed, setFailed] = useState(false);
  const source = selectedImage ?? product.images.find((item) => item.isPrimary) ?? product.images[0];
  const safeSource = source?.url.startsWith("/") ? source.url : undefined;
  return <div className={`product-image ${className}`}>{safeSource && !failed ? <Image src={safeSource} alt={source.altText || product.name} fill sizes="(max-width: 900px) 100vw, 40vw" onError={() => setFailed(true)} /> : <div className="image-placeholder" role="img" aria-label={`Imagem indisponível de ${product.name}`}><span>3D</span></div>}</div>;
}

