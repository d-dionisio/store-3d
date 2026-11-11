"use client";

import Script from "next/script";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { createCardPayment, CustomerApiError, paymentStatus, type CardPayment } from "@/services/customer-api";

type BrickFormData = { token?: string; payment_method_id?: string; installments?: number; payer?: { identification?: { type?: string; number?: string } } };
type BrickController = { unmount: () => void };
declare global { interface Window { MercadoPago?: new (key: string, options: { locale: string }) => { bricks: () => { create: (name: string, container: string, settings: unknown) => Promise<BrickController> } } } }

export function CardPaymentPanel({ reference, total }: { reference: string; total: number }) {
  const router = useRouter();
  const controller = useRef<BrickController | null>(null);
  const [loaded, setLoaded] = useState(false);
  const [result, setResult] = useState<CardPayment | null>(null);
  const [error, setError] = useState("");
  const publicKey = process.env.NEXT_PUBLIC_MERCADO_PAGO_PUBLIC_KEY || "";

  useEffect(() => {
    if (!loaded || !publicKey || !window.MercadoPago || controller.current) return;
    let active = true;
    const mp = new window.MercadoPago(publicKey, { locale: "pt-BR" });
    mp.bricks().create("cardPayment", "card-payment-brick", {
      initialization: { amount: total / 100 },
      customization: { paymentMethods: { types: { excluded: ["debit_card", "prepaid_card"] } } },
      callbacks: {
        onReady: () => undefined,
        onError: () => setError("Não foi possível carregar o formulário seguro."),
        onSubmit: async (data: BrickFormData) => {
          setError("");
          if (!data.token || !data.payment_method_id || !data.installments) throw new Error("Tokenização incompleta");
          try {
            const card = await createCardPayment(reference, {
              token: data.token,
              payment_method_id: data.payment_method_id,
              installments: data.installments,
              identification_type: data.payer?.identification?.type,
              identification_number: data.payer?.identification?.number,
            }, crypto.randomUUID());
            setResult(card);
            return card;
          } catch (cause) {
            setError(cause instanceof CustomerApiError ? cause.message : "Não foi possível processar o cartão.");
            throw cause;
          }
        },
      },
    }).then((created) => { if (active) controller.current = created; else created.unmount(); })
      .catch(() => setError("Não foi possível iniciar o pagamento com cartão."));
    return () => { active = false; controller.current?.unmount(); controller.current = null; };
  }, [loaded, publicKey, reference, total]);

  useEffect(() => {
    if (!result || !["pending", "processing"].includes(result.Status)) return;
    const timer = setInterval(async () => {
      try {
        const state = await paymentStatus(reference);
        if (state.Status === "paid") { clearInterval(timer); router.refresh(); }
      } catch { /* indisponibilidade momentânea não altera o estado financeiro */ }
    }, 5000);
    return () => clearInterval(timer);
  }, [reference, result, router]);

  if (!publicKey) return <p role="alert" className="rounded-xl bg-amber-50 p-4 text-sm text-amber-900">Pagamento com cartão indisponível: configure a chave pública do Mercado Pago.</p>;
  return <section>
    <Script src="https://sdk.mercadopago.com/js/v2" strategy="afterInteractive" onLoad={() => setLoaded(true)} />
    {result?.RequiresAction && result.AuthenticationURL ? <div className="mb-5 rounded-2xl border border-violet-200 p-4">
      <h3 className="font-black">Confirme a compra com seu banco</h3>
      <p className="mt-1 text-sm text-stone-600">A cobrança permanece pendente até o desafio 3DS ser concluído.</p>
      <iframe title="Autenticação segura 3DS" src={result.AuthenticationURL} className="mt-4 h-[520px] w-full rounded-xl border" sandbox="allow-forms allow-scripts allow-same-origin" />
    </div> : null}
    <div id="card-payment-brick" />
    {result && !result.RequiresAction ? <p aria-live="polite" className="mt-4 rounded-xl bg-violet-50 p-3 text-sm">Status do cartão: <strong>{result.Status}</strong>. A confirmação final vem do Mercado Pago.</p> : null}
    {error ? <p role="alert" className="mt-3 text-sm text-red-700">{error}</p> : null}
  </section>;
}
