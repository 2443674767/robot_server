package dbtool

import "os"

// 导出数据库数据sql文件
func ExecSqlFile(tables []string, pathname string) {
	f, _ := os.Create(pathname)
	_ = DBDump(
		WithDropTable(),    // Option: Delete table before create (Default: Not delete table)
		WithData(),         // Option: Dump Data (Default: Only dump table schema)
		WithTables(tables), // Option: Dump Tables (Default: All tables)
		WithWriter(f),      // Option: Writer (Default: os.Stdout)
	)
	f.Close()
}
