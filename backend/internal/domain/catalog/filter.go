package catalog

type ProductSort string

const (
	SortNameAscending   ProductSort = "name_asc"
	SortNameDescending  ProductSort = "name_desc"
	SortPriceAscending  ProductSort = "price_asc"
	SortPriceDescending ProductSort = "price_desc"
)

type ProductFilter struct {
	Category      string
	Search        string
	Color         string
	MinPriceCents *int64
	MaxPriceCents *int64
	Sort          ProductSort
	Page          int
	Limit         int
}

func (filter ProductFilter) Offset() int {
	return (filter.Page - 1) * filter.Limit
}

type ProductPage struct {
	Products []Product
	Total    int64
	Page     int
	Limit    int
}
