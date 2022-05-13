package autoload

type MysqlConfig struct {
	Host        string `ini:"host"`
	Username    string `ini:"username"`
	Password    string `ini:"password"`
	Port        uint   `ini:"port"`
	Database    string `ini:"database"`
	Charset     string `ini:"charset"`
	TablePrefix string `ini:"table_prefix"`
}

var Mysql = &MysqlConfig{
	Host:        "127.0.0.1",
	Username:    "root",
	Password:    "root1234",
	Port:        3306,
	Database:    "test",
	Charset:     "utf8mb4",
	TablePrefix: "",
}
