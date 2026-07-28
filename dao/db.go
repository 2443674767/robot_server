package dao

import (
	"context"
	"fmt"
	"gofly/dao/drivers"
	"gofly/dao/query"
	"gofly/utils/tools/gcfg"
	"gofly/utils/tools/gconv"
	"gofly/utils/tools/instance"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gen"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

var (
	runEnv, _ = gcfg.Instance("app").Get(context.Background(), "app.runEnv")
)

type (
	// Condition query condition
	// field.Expr and subquery are expect value
	Condition = gen.Condition
	Querys    = query.Query
)

// Create a writer for logging entries/创建写日志的writer
type logWriter struct {
	stdout bool
}

// Default database configuration/默认数据库配置
const (
	DefaultGroupName      = "default"            // Default group name.
	ComponentNameDatabase = "component.database" // Database of Component Names
)

// Return the query object. The parameter 'name' is the name of the database configuration, with the default value being 'default'.
// 返回query对象，参数name是数据库配置名称，默认default。
func Query(name ...string) *query.Query {
	return query.Use(DB(name...))
}

// Instantiate the data object/实例化数据对象
func DB(name ...string) *gorm.DB {
	return database(name...)
}

// Obtain the current table prefix/获取当前表前缀
func GetTablePrefix(name ...string) string {
	db := Query(name...).Admin.WithContext(context.Background()).UnderlyingDB()
	if ns, ok := db.Config.NamingStrategy.(schema.NamingStrategy); ok {
		return ns.TablePrefix
	}
	return ""
}

// RenameTableWithPrefix Drop the table with prefix, then rename original table with prefix
// tableName: original table name
// prefix: backup prefix, e.g. gf_
func RenameTableWithPrefix(ctx context.Context, db *gorm.DB, tableName, prefix string) error {
	backupTable := prefix + tableName
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		oldTbl := gorm.Expr("`" + tableName + "`")
		newTbl := gorm.Expr("`" + backupTable + "`")
		// 1. Drop backup table if exists
		if err := tx.Exec("DROP TABLE IF EXISTS ?", newTbl).Error; err != nil {
			return fmt.Errorf("failed to drop old backup table: %w", err)
		}
		// 2. Rename original table to table with backup prefix
		if err := tx.Exec("ALTER TABLE ? RENAME TO ?", oldTbl, newTbl).Error; err != nil {
			return fmt.Errorf("failed to rename table: %w", err)
		}
		return nil
	})
}

// Database returns an instance of database ORM object with specified configuration group name.
// Note that it panics if any error occurs duration instance creating.
func database(name ...string) *gorm.DB {
	var group = DefaultGroupName
	if len(name) > 0 && name[0] != "" {
		group = name[0]
	}
	instanceKey := fmt.Sprintf("%s.%s", ComponentNameDatabase, group)
	db := instance.GetOrSetFuncLock(instanceKey, func() interface{} {
		// Create a new ORM object with given configurations.
		// fmt.Println("1.执行数据对象方法", group)
		if db, err := NewConnectDB(group); err == nil {
			return db
		} else {
			// If panics, often because it does not find its configuration for given group.
			panic(err)
		}
	})

	if db != nil {
		return db.(*gorm.DB)
	}
	return nil
}

// 清除DB实例对象
func Clear(name ...string) {
	var group = DefaultGroupName
	if len(name) > 0 {
		group = name[0]
	}
	instance.ClearOne(fmt.Sprintf("%s.%s", ComponentNameDatabase, group))
}

// NewConnectDB creates and returns an ORM object with global configurations.
func NewConnectDB(groupName string) (*gorm.DB, error) {
	// Retrieve the database configuration and determine whether the database configuration exists.
	dbConf, err := gcfg.Instance().Get(context.Background(), "database."+groupName)
	if err != nil {
		return nil, err
	}
	if dbConf == nil {
		return nil, fmt.Errorf("The configuration of the data %s does not exist.", groupName)
	}
	dbConf_arr := gconv.Map(dbConf)

	// 1.仅在终端输出
	newLogger := logger.Discard //日志级别：Silent、Error、Warn、Info
	//2.在debug模式下才判处理文件日志（同时输出到文件和控制台）
	if runEnv.String() != "release" && gconv.Bool(dbConf_arr["debug"]) {
		newLogger = logger.New(
			logWriter{stdout: gconv.Bool(dbConf_arr["stdout"])}, //Set log type
			logger.Config{
				SlowThreshold:             time.Second,   // Slow SQL threshold
				LogLevel:                  logger.Silent, // Log level,eg: Silent、Error、Warn、Info
				IgnoreRecordNotFoundError: false,         // Ignore ErrRecordNotFound error for logger
				ParameterizedQueries:      false,         // Don't include params in the SQL log
				Colorful:                  true,          // Disable color
			},
		)
	}
	db, err := gorm.Open(drivers.DbOpen(dbConf_arr), &gorm.Config{
		Logger: newLogger,
		DryRun: gconv.Bool(dbConf_arr["dryrun"]),
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   gconv.String(dbConf_arr["prefix"]), // table name prefix, table for `User` would be `t_users`
			SingularTable: true,                               // use singular table name, table for `User` would be `user` with this option enabled
			NoLowerCase:   true,                               // skip the snake_casing of names
			NameReplacer:  strings.NewReplacer("CID", "Cid"),  // use name replacer to change struct/field name before convert it to db name
		},
	})
	if err != nil {
		return nil, fmt.Errorf("connect db fail: %w", err)
	}

	//Set up the connection pool/设置连接池
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("Set up the connection pool fail: %w", err)
	}

	// SetMaxIdleConns 设置空闲连接池中连接的最大数量。
	sqlDB.SetMaxIdleConns(gconv.Int(dbConf_arr["maxIdle"]))
	// SetMaxOpenConns 设置打开数据库连接的最大数量。
	sqlDB.SetMaxOpenConns(gconv.Int(dbConf_arr["maxOpen"]))
	// SetConnMaxLifetime 设置了可以重新使用连接的最大时间。
	sqlDB.SetConnMaxLifetime(time.Duration(gconv.Int(dbConf_arr["maxLifetime"])) * time.Hour)
	// SetConnMaxIdleTime 设置连接最大空闲时间（空闲30秒就关闭）
	sqlDB.SetConnMaxIdleTime(time.Duration(gconv.Int(dbConf_arr["maxIdletime"])) * time.Second)

	// Enable Debug mode/开启Debug模式
	if runEnv.String() != "release" && gconv.Bool(dbConf_arr["debug"]) {
		return db.Debug(), nil
	}
	return db, nil
}

// Customized SQL log output (to file)/自定义的sql日志输出（到文件）
func (w logWriter) Printf(format string, args ...interface{}) {
	msg := "\n" + fmt.Sprintf(format, args...)
	if runEnv.String() == "debug" && w.stdout {
		fmt.Println(msg) // 打印到控制台
	}
	//处理文件名:
	var filePath string = ""
	path, err := os.Getwd() //获取当前路径
	if err != nil {
		filePath = "./runtime/log/gorm/"
	} else {
		filePath = filepath.Join(path, "/runtime/log/gorm/")
	}
	os.MkdirAll(filePath, 0755)
	filePathName := filepath.Join(filePath, time.Now().Format("2006-01-02")+".log")
	// 替换掉彩色打印符号
	msg = strings.ReplaceAll(msg, logger.Reset, "")
	msg = strings.ReplaceAll(msg, logger.Red, "")
	msg = strings.ReplaceAll(msg, logger.Green, "")
	msg = strings.ReplaceAll(msg, logger.Yellow, "")
	msg = strings.ReplaceAll(msg, logger.Blue, "")
	msg = strings.ReplaceAll(msg, logger.Magenta, "")
	msg = strings.ReplaceAll(msg, logger.Cyan, "")
	msg = strings.ReplaceAll(msg, logger.White, "")
	msg = strings.ReplaceAll(msg, logger.BlueBold, "")
	msg = strings.ReplaceAll(msg, logger.MagentaBold, "")
	msg = strings.ReplaceAll(msg, logger.RedBold, "")
	msg = strings.ReplaceAll(msg, logger.YellowBold, "")

	//写入文件
	file, err := os.OpenFile(filePathName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("日志文件的打开错误 :", err)
	}
	defer file.Close()
	if _, err := file.WriteString(msg); err != nil {
		fmt.Println("写入日志文件错误 :", err)
	}
}

// Offset 处理分页偏移量，页数 (pageNo) 转 偏移量 (offset)
func Offset(PageNo, PageSize int) int {
	if PageNo <= 0 {
		PageNo = 1
	}
	if PageSize <= 0 {
		PageSize = 10 // 默认每页10条
	}
	return (PageNo - 1) * PageSize
}
