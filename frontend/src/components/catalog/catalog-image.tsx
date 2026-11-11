"use client";

import Image from "next/image";
import { useState } from "react";

export function CatalogImage({ src, alt, priority = false, sizes = "100vw" }: { src?: string; alt: string; priority?: boolean; sizes?: string }) {
  const [failed, setFailed] = useState(false);
  const safeSource = src?.startsWith("/") ? src : undefined;
  return safeSource && !failed ? (
    <Image src={safeSource} alt={alt} fill priority={priority} sizes={sizes} onError={() => setFailed(true)} />
  ) : (
    <div className="catalog-image-placeholder" role="img" aria-label={`Imagem indisponível: ${alt}`}><span>LUMÉ</span></div>
  );
}
