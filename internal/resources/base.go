package resources

import (
	"math"

	"github.com/wannanbigpig/gin-layout/internal/global"
)

type Paginate struct {
	Total       int64 `json:"total"`
	PerPage     int   `json:"per_page"`
	CurrentPage int   `json:"current_page"`
	LastPage    int   `json:"last_page"`
}

func (p *Paginate) calculateLastPage() {
	if p.CurrentPage < 1 {
		p.CurrentPage = 1
	}
	if p.PerPage < 1 {
		p.PerPage = global.PerPage
	}

	p.LastPage = int(math.Ceil(float64(p.Total) / float64(p.PerPage)))
}

type Collection struct {
	Paginate
	Data []any `json:"data"`
}

func newResponseCollection(paginate Paginate, data []any) *Collection {
	paginate.calculateLastPage()
	return &Collection{
		Paginate: paginate,
		Data:     data,
	}
}
