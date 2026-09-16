package global

const (
	// PerPage is the default per page size
	PerPage = 10
	// MaxPerPage 是每页条数上限，防止超大 LIMIT 拖库
	MaxPerPage = 200
	// Yes is the value of yes
	Yes uint8 = 1
	// No is the value of no
	No uint8 = 0
	// ChinaCountryCode is the value of China country code
	ChinaCountryCode = "86"
	// SUCCESS is the value of success
	SUCCESS = "SUCCESS"
	// ERROR is the value of error
	ERROR = "ERROR"
)

// Version 是默认版本号；Release 构建可通过 -ldflags 注入 Git Tag。
var Version = "0.9.2"
