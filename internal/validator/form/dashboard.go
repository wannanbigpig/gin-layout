package form

// DashboardStatisticsQuery defines an inclusive calendar-date range for
// dashboard trend statistics. Both values must be supplied together.
type DashboardStatisticsQuery struct {
	StartDate string `form:"start_date" json:"start_date" binding:"omitempty,datetime=2006-01-02" label:"开始日期"`
	EndDate   string `form:"end_date" json:"end_date" binding:"omitempty,datetime=2006-01-02" label:"结束日期"`
}

func NewDashboardStatisticsQuery() *DashboardStatisticsQuery {
	return &DashboardStatisticsQuery{}
}
