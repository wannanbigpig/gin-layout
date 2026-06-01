package taskcenter

import (
	"github.com/wannanbigpig/gin-layout/internal/pkg/query_builder"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

type listQuery struct {
	*query_builder.QueryBuilder
}

func newListQuery() *listQuery {
	return &listQuery{QueryBuilder: query_builder.New()}
}

func (q *listQuery) addEq(field string, value any) *listQuery {
	q.QueryBuilder.AddEq(field, value)
	return q
}

func (q *listQuery) addLike(field, value string) *listQuery {
	q.QueryBuilder.AddLike(field, value)
	return q
}

func (q *listQuery) addCreatedAtRange(startTime, endTime string) *listQuery {
	if t, ok := utils.ParseTimeBoundary(startTime); ok {
		q.QueryBuilder.AddCondition("created_at >= ?", t)
	}
	if t, ok := utils.ParseTimeBoundary(endTime); ok {
		q.QueryBuilder.AddCondition("created_at <= ?", t)
	}
	return q
}
