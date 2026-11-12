# Homologação visual — Luxury Smart Cart

Data: 12 de novembro de 2026.

Ambiente:

- Google Chrome 157.0.8103.39 em modo headless real.
- Frontend Next.js em `http://localhost:3000`.
- API Go em `http://localhost:8080`.
- Produtos, variantes, fotografias e preços carregados da API local.

Resultados:

- Carrinho vazio, um item, dois itens e cinco itens: aprovado.
- Abertura automática após adicionar: aprovado.
- Incremento, decremento e remoção: aprovado.
- Drawer desktop 1440 px: 460 px de largura, aprovado.
- Tablet 820 px: 460 px de largura, aprovado.
- Mobile 390 px e 320 px: largura integral, aprovado.
- Rolagem interna com cinco itens e rodapé sempre acessível: aprovado.
- Foco inicial, focus trap, Escape, overlay, restauração de foco e bloqueio da página: aprovado.
- Animação normal e `prefers-reduced-motion`: aprovado.
- Revalidação de preço e variante indisponível: aprovado.
- Checkout habilitado somente após validação e navegação para `/checkout`: aprovado.
- Overflow horizontal: não detectado.

Capturas:

- `empty-1440.png`
- `one-item-1440.png`
- `multiple-items-820.png`
- `multiple-items-390.png`
- `multiple-items-320.png`
- `price-changed-1440.png`
- `unavailable-1440.png`

O botão circular com a letra `N` visível no canto inferior de algumas capturas pertence ao indicador do Next.js em modo de desenvolvimento. Ele não está presente no build de produção.
