package audit

import "github.com/wannanbigpig/gin-layout/internal/pkg/query_builder"

type logListQuery struct {
	*query_builder.QueryBuilder
}

func newLogListQuery() *logListQuery {
	return &logListQuery{QueryBuilder: query_builder.New()}
}

func (q *logListQuery) addEq(field string, value any) *logListQuery {
	q.QueryBuilder.AddEq(field, value)
	return q
}

func (q *logListQuery) addLike(field, value string) *logListQuery {
	q.QueryBuilder.AddLike(field, value)
	return q
}

func (q *logListQuery) addCondition(condition string, args ...any) *logListQuery {
	q.QueryBuilder.AddCondition(condition, args...)
	return q
}

func (q *logListQuery) addCreatedAtRange(startTime, endTime string) *logListQuery {
	if startTime != "" {
		q.QueryBuilder.AddCondition("created_at >= ?", startTime)
	}
	if endTime != "" {
		q.QueryBuilder.AddCondition("created_at <= ?", endTime)
	}
	return q
}

func uintFilterValue(value uint) any {
	if value == 0 {
		return nil
	}
	return value
}
