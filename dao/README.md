# GORM Gen说明
 Gen是由字节跳动无恒实验室与GORM作者联合研发的一个基于GORM的安全ORM框架，其主要通过代码生成方式实现GORM代码封装。使用Gen框架能够自动生成Model结构体和类型安全的CRUD代码，极大地减少样板代码的编写，提升开发效率。

 Gen框架在GORM框架的基础上提供了以下能力：
- 基于原始SQL语句生成可重用的CRUD API
- 生成不使用interface{}的100%安全的DAO API
- 依据数据库生成遵循GORM约定的结构体Model
- 支持GORM的所有特性

 简单来说，使用Gen框架后我们无需手动定义结构体Model，同时Gen框架也能帮我们生成类型安全的CRUD代码。
# ORM目录结构及使用说明
  本框架把数据库相关代码统一存放在dao目录中，dao目录是存放GORM Gen数据模型模型代码生成、数据库链接等相关操作，其中cmd目录是gen生成代码程序；model目录和query目录是用cmd中程序生成的；drivers目录是存放数据库驱动程序配置操作（如：MySQL驱动程序gorm_mysql.go、postgres动程序gorm_postgres.go等）；db.go文件是数据库连接实例统一调用入口。

  注意：mssql是SQL Server数据库（sqlserver）

# 生成数据库模型
 在项目dao\cmd目录下，通过以下命令运行生成器：
```
go run gen.go
```
运行后，GORM Gen 会根据数据库结构，在指定的OutPath目录下（即：dao）生成相应的模型代码（model）和查询代码（query）。注意：每次更改数据库（新增、删除数据表和添加、删除、修改数据表字段）都要重新运行go run gen.go命令进行更新数据库模型代码和查询代码，确保与数据库一致。

# ORM使用说明
## 数据库配置
数据库配置统一放在resource\config\config.yaml文件中，数据配置如下：
``` yaml
database: #数据库配置
  default:
    #地址
    hostname: 127.0.0.1
    #端口           
    hostport: 3306
    #账号                 
    username: root
    #密码              
    password: root
    #数据库名称           
    dbname: gofly_gen_v1
    #表名前缀
    prefix: gf_
    type:          "mysql"                   #数据库类型(如：mariadb/tidb/mysql/pgsql/mssql/sqlite/oracle/clickhouse/dm) 
    sqlmode:       "NO_ENGINE_SUBSTITUTION"  #设置数据库sql_mode，当数据库类型mariadb/mysql可设置，不设置留空
    extra:         "&parseTime=True"         #不同数据库的额外特性配置，由底层数据库driver定义,mysql填：&parseTime=True,如果pgsql配置：不填
    role:          "master"                  #数据库主从角色(master/slave)，不使用应用层的主从机制请均设置为master
    debug:         true                      #开启调试模式，且runEnv为 开发debug和测试模式test生效
    dryrun:        false                     #生成 SQL 但不执行，可以用于准备或测试生成的 SQL
    weight:        100                       #负载均衡权重，用于负载均衡控制（预留给多个数据库扩展）
    charset:       "utf8mb4"                 #数据库编码(如: utf8/utf8mb4/gbk/gb2312)，一般设置为utf8mb4,低版本数据库设置utf8
    timezone:      "Local"                   #时区配置，例如:Local",如果pgsql配置：Asia/Shanghai
    maxIdle:       10                        #连接池最大闲置的连接数
    maxOpen:       100                       #连接池最大打开的连接数
    maxLifetime:   1                         #连接对象可重复使用的时间长度（单位：小时）
    maxIdletime:   30                        #设置连接最大空闲时间（单位：秒）
    
```
其中 default是数据库配置默认分组，如果还有其他数据库，则复选default下配置进行修改，并把default名称改成新数据库相关命名。
## 调用ORM模型操作数据库
这里说明一下，框架为开发者封装好数据库调用对象，统一封装在dao\db.go中，然后在dao\drivers数据连接不同数据的驱动配置，框架值默认Mysql数据，如果使用其他数据库参考Mysql配置好dsn和打开dao\drivers\drivers.go中对应switch类型。
### Gen数据库操作使用
#### Gen DAO操作
框架为开发分组数据库连接和调用方法，使用Gen Query对象则使用方法如下:
- 引入dao和model

``` golang
import (
	"gofly/dao"
    "gofly/dao/model"
}

```
- 通过：dao.Query().数据表模型+对应操作，使用数据表model，直接：model.数据表模型
完整代码如下：
``` golang
package test

import (
	"fmt"
	"gofly/dao"
	"gofly/dao/model"
	"gofly/utils/gf"
)

// 这是一个GORM Gen数据库操作示例，开发业务可以参考
type Orm struct {
	NoNeedLogin []string //忽略登录接口配置-忽略全部传[*]
	NoNeedAuths []string //忽略RBAC权限认证接口配置-忽略全部传[*]
}

func init() {
	gf.Register(&Orm{NoNeedLogin: []string{"*"}, NoNeedAuths: []string{"*"}})
}
// 添加数据
func (api *Orm) Add(ctx *gf.GinCtx) {
	var json bookData
	if err := ctx.ShouldBind(&json); err != nil {
		gf.Failed().SetMsg(errerr.Error()).Regin(ctx)
		return
	}
	// 创建
	b1 := model.Book{
		Title:       json.Title,
		Author:      json.Author,
		Price:       json.Price,
	}
	err := dao.Query().Book.WithContext(ctx).Create(&b1)
	if err != nil {
		gf.Failed().SetMsg(fmt.Sprintf("创建数据失败:%v", err)).Regin(ctx)
		return
	}
	gf.Success().SetMsg("创建数据操作操作").SetData(b1).Regin(ctx)
}

```

> 更多使用方法可以参考：app\admin\demo\orm.go示例代码