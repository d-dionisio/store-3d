# API pública de produtos

Base local: `http://localhost:8080/api/v1`.

## Listar produtos

```http
GET /products?category=decoracao&search=vaso&color=marfim&min_price=1000&max_price=30000&sort=price_asc&page=1&limit=20
```

Preços dos filtros são informados em centavos. `sort` aceita `name_asc`, `name_desc`, `price_asc` e `price_desc`. `limit` aceita valores entre 1 e 100.

```json
{
  "data": [
    {
      "id": "60000000-0000-0000-0000-000000000001",
      "slug": "vaso-orbita",
      "name": "Vaso Órbita",
      "short_description": "Curvas contínuas para composições contemporâneas.",
      "description": "Vaso decorativo fictício produzido sob demanda em impressão 3D.",
      "is_featured": true,
      "production_type": "made_to_order",
      "min_price_cents": 18900,
      "category": { "id": "...", "slug": "decoracao", "name": "Decoração", "description": "..." },
      "images": [],
      "materials": [{ "id": "...", "code": "PLA", "name": "PLA Premium" }],
      "colors": [{ "id": "...", "slug": "marfim", "name": "Marfim", "hex_code": "#F7F5F0" }],
      "variants": [
        {
          "id": "...",
          "sku": "VASO-ORB-PLA-MAR",
          "name": "Marfim",
          "price_cents": 18900,
          "available_quantity": 0,
          "track_inventory": false,
          "weight_grams": 420,
          "dimensions": { "width_mm": 180, "height_mm": 240, "depth_mm": 180 },
          "production_lead_time": { "min_days": 5, "max_days": 8 },
          "material": { "id": "...", "code": "PLA", "name": "PLA Premium" },
          "color": { "id": "...", "slug": "marfim", "name": "Marfim", "hex_code": "#F7F5F0" }
        }
      ]
    }
  ],
  "meta": { "page": 1, "limit": 20, "total": 1, "total_pages": 1 }
}
```

## Consultar produto

```http
GET /products/{slug}
```

Retorna `{"data": product}` com o mesmo DTO da listagem. Produto inexistente, inativo ou sem variante comercializável retorna HTTP 404.

## Listar categorias

```http
GET /categories
```

Somente categorias ativas com produto e variante comercializável são retornadas.

## Erros

```json
{
  "error": {
    "code": "invalid_request",
    "message": "limit deve estar entre 1 e 100"
  }
}
```

- HTTP 400: filtros inválidos.
- HTTP 404: produto não encontrado.
- HTTP 500: falha interna, sem SQL ou detalhes da infraestrutura.

## Diferenças para os tipos atuais do Next.js

Os mocks existentes permanecem no frontend. A adaptação futura deverá considerar:

| Mock atual | DTO da API |
|---|---|
| `id` textual curto | UUID textual |
| `price` em reais | `min_price_cents` e `variant.price_cents` em centavos |
| `category` como string | `category` como objeto |
| `colors` como nomes | `colors` como objetos e cores nas variantes |
| `material` como string | `materials` e `variant.material` |
| `dimensions` formatada | milímetros por variante |
| `leadTime` formatado | `production_lead_time.min_days/max_days` |
| `imagePosition` local | `images[]` com URL, alt text e variante opcional |

O frontend deverá formatar moeda, dimensões e prazo na apresentação, sem transformar valores monetários em ponto flutuante.

