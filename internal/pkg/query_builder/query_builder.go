package query_builder

import "strings"

type QueryBuilder struct {
	conditions []string
	args       []any
}

func New() *QueryBuilder {
	return &QueryBuilder{
		conditions: make([]string, 0),
		args:       make([]any, 0),
	}
}

func (qb *QueryBuilder) AddCondition(cond string, args ...any) *QueryBuilder {
	if cond != "" {
		qb.conditions = append(qb.conditions, cond)
		qb.args = append(qb.args, args...)
	}
	return qb
}

func (qb *QueryBuilder) AddLike(field, value string) *QueryBuilder {
	if value != "" {
		qb.conditions = append(qb.conditions, field+" like ?")
		qb.args = append(qb.args, "%"+value+"%")
	}
	return qb
}

func (qb *QueryBuilder) AddEq(field string, value any) *QueryBuilder {
	if hasValue(value) {
		qb.conditions = append(qb.conditions, field+" = ?")
		qb.args = append(qb.args, value)
	}
	return qb
}

func (qb *QueryBuilder) AddIn(field string, values []uint) *QueryBuilder {
	if len(values) > 0 {
		qb.conditions = append(qb.conditions, field+" IN (?)")
		qb.args = append(qb.args, values)
	}
	return qb
}

func (qb *QueryBuilder) AddExists(subQuery string) *QueryBuilder {
	if subQuery != "" {
		qb.conditions = append(qb.conditions, "EXISTS ("+subQuery+")")
	}
	return qb
}

func (qb *QueryBuilder) AddConditionf(cond string, args ...any) *QueryBuilder {
	return qb.AddCondition(cond, args...)
}

func (qb *QueryBuilder) AddKeywordLike(keyword string, fields ...string) *QueryBuilder {
	if keyword == "" || len(fields) == 0 {
		return qb
	}

	clauses := make([]string, 0, len(fields))
	for range fields {
		qb.args = append(qb.args, "%"+keyword+"%")
	}
	for _, field := range fields {
		clauses = append(clauses, field+" like ?")
	}
	qb.conditions = append(qb.conditions, "("+strings.Join(clauses, " OR ")+")")
	return qb
}

func (qb *QueryBuilder) Build() (string, []any) {
	if len(qb.conditions) == 0 {
		return "", nil
	}
	cond := qb.conditions[0]
	for i := 1; i < len(qb.conditions); i++ {
		cond += " AND " + qb.conditions[i]
	}
	return cond, qb.args
}

func hasValue(value any) bool {
	if value == nil {
		return false
	}

	switch typed := value.(type) {
	case string:
		return typed != ""
	case *string:
		return typed != nil && *typed != ""
	case *int8:
		return typed != nil
	case *uint8:
		return typed != nil
	case *uint:
		return typed != nil
	case *int:
		return typed != nil
	default:
		return true
	}
}
