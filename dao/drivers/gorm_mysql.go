// =====================================
// MySQL database driver operation
// Other databases are configured based on the GORM files and this MySQL database configuration.
// GORM Doc: https://gorm.io/zh_CN/docs/connecting_to_the_database.html
// ======================================
package drivers

import (
	"fmt"
	"gofly/utils/tools/gconv"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// DSN data source name
func GormMysqlDSN(dbConf_arr map[string]any) string {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%v)/%s?charset=%s&loc=%s",
		dbConf_arr["username"],
		dbConf_arr["password"],
		dbConf_arr["hostname"],
		dbConf_arr["hostport"],
		dbConf_arr["dbname"],
		dbConf_arr["charset"],
		dbConf_arr["timezone"],
	)
	//The link has successfully set the SQL_MODE mode for the database./链接成功设置数据库sql_mode模式
	if gconv.String(dbConf_arr["sqlmode"]) != "" {
		dsn = fmt.Sprintf("%s&sql_mode=%s", dsn, dbConf_arr["sqlmode"])
	}
	if gconv.String(dbConf_arr["extra"]) != "" {
		dsn = fmt.Sprintf("%s&%s", dsn, dbConf_arr["extra"])
	}
	return dsn
}

// GormMysql is used to initialize the MySQL driver adapter.
func GormMysql(dbConf_arr map[string]any) gorm.Dialector {
	return mysql.Open(GormMysqlDSN(dbConf_arr))
}
