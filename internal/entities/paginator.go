package entities

const MaxPageSize = 150

type Paginator struct {
	Page    int  `json:"page"`
	Limit   int  `json:"limit"`
	Total   int  `json:"total"`
	HasNext bool `json:"has_next"`
	HasPrev bool `json:"has_prev"`
}

func NewPaginator(page, limit, total int) Paginator {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = total
	}
	return Paginator{
		Page:    page,
		Limit:   limit,
		Total:   total,
		HasNext: (page * limit) < total,
		HasPrev: page > 1,
	}
}
