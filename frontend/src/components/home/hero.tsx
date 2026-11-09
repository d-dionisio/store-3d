"use client";

import Image from "next/image";
import { ArrowRight, Sparkles } from "lucide-react";
import { motion } from "motion/react";
import { Button } from "@/components/ui/button";

export function Hero() {
  return (
    <section className="hero">
      <Image src="/images/hero-studio.png" alt="Vaso espiral e escultura abstrata impressos em 3D em um estúdio claro" fill priority sizes="100vw" className="hero-image" />
      <div className="hero-overlay" />
      <motion.div className="container hero-content" initial={{ opacity: 0, y: 24 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.7 }}>
        <span className="eyebrow"><Sparkles size={15} /> Objetos autorais, feitos camada por camada</span>
        <h1>Design que você<br />pode <em>tocar.</em></h1>
        <p>Peças com presença, produzidas sob demanda com tecnologia, cuidado e intenção.</p>
        <div className="button-row"><Button href="/produtos">Explorar coleção <ArrowRight size={17} /></Button><Button href="/personalizados" variant="secondary">Criar peça personalizada</Button></div>
      </motion.div>
    </section>
  );
}

