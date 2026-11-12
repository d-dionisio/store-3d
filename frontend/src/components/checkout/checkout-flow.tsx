"use client";

import { type FormEvent, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCart } from "@/components/cart/cart-provider";
import { createAddress, createCatalogOrder, type CustomerAddress, listAddresses } from "@/services/customer-api";
import { formatCurrency } from "@/lib/format";

export function CheckoutFlow() {
  const { items, ready, clear, validatedCart, validationStatus, validationError, unavailableVariantIds } = useCart();
  const [addresses, setAddresses] = useState<CustomerAddress[]>([]);
  const [selected, setSelected] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const router = useRouter();

  useEffect(() => {
    if (!ready || !items.length) return;
    let active = true;
    listAddresses().then((values) => {
      if (!active) return;
      setAddresses(values); setSelected(values[0]?.ID || "");
    }).catch((cause: unknown) => {
      if (active) setError(cause instanceof Error ? cause.message : "Entre na sua conta para continuar.");
    });
    return () => { active = false; };
  }, [items.length, ready]);

  async function addAddress(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    try {
      const address = await createAddress({
        RecipientName: String(form.get("name")), PostalCode: String(form.get("postal")),
        Street: String(form.get("street")), Number: String(form.get("number")),
        Complement: String(form.get("complement")), District: String(form.get("district")),
        City: String(form.get("city")), StateCode: String(form.get("state")),
        CountryCode: "BR", IsDefault: addresses.length === 0,
      });
      setAddresses((current) => [address, ...current]); setSelected(address.ID);
    } catch (cause) { setError(cause instanceof Error ? cause.message : "Endereço inválido."); }
  }

  async function finish() {
    if (!validatedCart || !selected || unavailableVariantIds.length) return;
    setBusy(true);
    try {
      let key = sessionStorage.getItem("loja3d_checkout_key");
      if (!key) { key = crypto.randomUUID(); sessionStorage.setItem("loja3d_checkout_key", key); }
      const order = await createCatalogOrder(items.map((item) => ({ variant_id: item.productVariantId, quantity: item.quantity })), selected, key);
      sessionStorage.removeItem("loja3d_checkout_key"); clear();
      router.push(`/checkout/confirmacao?reference=${encodeURIComponent(order.reference)}&status=${order.status}`);
    } catch (cause) { setError(cause instanceof Error ? cause.message : "Checkout indisponível."); setBusy(false); }
  }

  if (!ready || validationStatus === "loading") return <p>Atualizando carrinho...</p>;
  if (!items.length) return <p>Seu carrinho está vazio.</p>;
  if (validationStatus === "error" || !validatedCart) return <div className="prototype-notice"><p>{validationError || "Não foi possível validar o carrinho."}</p><Link href="/carrinho">Revisar carrinho</Link></div>;
  if (unavailableVariantIds.length) return <div className="prototype-notice"><p>Existem itens indisponíveis ou acima do estoque atual.</p><Link href="/carrinho">Revisar carrinho</Link></div>;
  if (error && !addresses.length) return <div className="prototype-notice"><p>{error}</p><Link href="/minha-conta">Acessar minha conta</Link></div>;

  return <div className="checkout-layout">
    <div className="checkout-form"><section><h2>Endereço de entrega</h2>
      {addresses.map((address) => <label className="choice-card" key={address.ID}><input type="radio" checked={selected === address.ID} onChange={() => setSelected(address.ID)} /><span><strong>{address.RecipientName}</strong><small>{address.Street}, {address.Number} · {address.City}/{address.StateCode}</small></span></label>)}
      <form onSubmit={addAddress} className="form-grid">
        <label><span>Destinatário</span><input name="name" required /></label><label><span>CEP</span><input name="postal" required /></label>
        <label><span>Logradouro</span><input name="street" required /></label><label><span>Número</span><input name="number" required /></label>
        <label><span>Complemento</span><input name="complement" /></label><label><span>Bairro</span><input name="district" required /></label>
        <label><span>Cidade</span><input name="city" required /></label><label><span>UF</span><input name="state" maxLength={2} required /></label>
        <button className="button" type="submit">Salvar endereço</button>
      </form>
    </section></div>
    <aside className="order-summary checkout-summary"><h2>Resumo validado</h2>
      {validatedCart.Items.map((item) => <div key={item.VariantID}><span>{item.ProductName} × {item.Quantity}</span><strong>{formatCurrency(item.LineTotalCents)}</strong></div>)}
      <div><span>Frete</span><strong>Aguardando cotação</strong></div><div className="total"><span>Subtotal</span><strong>{formatCurrency(validatedCart.SubtotalCents)}</strong></div>
      <button disabled={busy || !selected} onClick={finish} className="button button-primary">{busy ? "Criando pedido..." : "Confirmar pedido"}</button>
      <small>Nenhum pagamento será processado. O pedido aguardará cotação de frete.</small>{error && <p role="alert">{error}</p>}
    </aside>
  </div>;
}
