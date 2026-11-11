"use client";

import Image from "next/image";
import { ArrowDown, ArrowRight } from "lucide-react";
import { motion } from "motion/react";
import { Button } from "@/components/ui/button";

export function Hero() {
  return <section className="hero editorial-hero">
    <Image src="/images/hero-studio.png" alt="Vaso escultural Lumé em ambiente de tons naturais" fill priority sizes="100vw" className="hero-image" />
    <div className="hero-overlay" />
    <motion.div className="container hero-content" initial="hidden" animate="visible" variants={{ hidden: {}, visible: { transition: { staggerChildren: .14 } } }}>
      <motion.span className="eyebrow" variants={{ hidden: { opacity: 0, y: 14 }, visible: { opacity: 1, y: 0 } }}>Lumé Studio · Objetos autorais</motion.span>
      <motion.h1 variants={{ hidden: { opacity: 0, y: 22 }, visible: { opacity: 1, y: 0 } }}>O belo<br />ganha <em>forma.</em></motion.h1>
      <motion.p variants={{ hidden: { opacity: 0, y: 18 }, visible: { opacity: 1, y: 0 } }}>Objetos autorais que transformam espaços em expressões de personalidade.</motion.p>
      <motion.div className="button-row" variants={{ hidden: { opacity: 0 }, visible: { opacity: 1 } }}><Button href="/produtos">Explorar coleções <ArrowRight size={16} /></Button><Button href="/sobre" variant="secondary">Conheça a Lumé</Button></motion.div>
    </motion.div>
    <a href="#colecoes" className="scroll-cue" aria-label="Ir para as coleções"><span>Descobrir</span><ArrowDown size={15} /></a>
  </section>;
}
