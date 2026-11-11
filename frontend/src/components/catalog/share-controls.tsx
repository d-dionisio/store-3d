"use client";

import { Check, Copy, Share2 } from "lucide-react";
import { useState } from "react";

export function ShareControls({ title }: { title: string }) {
  const [feedback, setFeedback] = useState("");
  const currentURL = () => window.location.href;
  const copied = async () => {
    try {
      await navigator.clipboard.writeText(currentURL());
      setFeedback("Link copiado");
    } catch {
      setFeedback("Não foi possível copiar");
    }
    window.setTimeout(() => setFeedback(""), 2200);
  };
  const share = async () => {
    if (!navigator.share) return copied();
    try { await navigator.share({ title, text: `Conheça a coleção ${title} da Lumé Studio.`, url: currentURL() }); }
    catch (error) { if (error instanceof DOMException && error.name !== "AbortError") setFeedback("Não foi possível compartilhar"); }
  };
  const whatsapp = `https://wa.me/?text=${encodeURIComponent(`Conheça a coleção ${title} da Lumé Studio: `)}`;

  return <div className="catalog-share" aria-label="Compartilhar catálogo">
    <button type="button" onClick={share}><Share2 size={16} /> Compartilhar</button>
    <button type="button" onClick={copied}>{feedback === "Link copiado" ? <Check size={16} /> : <Copy size={16} />} Copiar link</button>
    <a href={whatsapp} onClick={(event) => { event.currentTarget.href = whatsapp + encodeURIComponent(currentURL()); }} target="_blank" rel="noreferrer">WhatsApp</a>
    <span className="sr-only" aria-live="polite">{feedback}</span>
  </div>;
}
