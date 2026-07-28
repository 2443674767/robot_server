# 机械狗框架
# 一、安装
如果是开发环境电脑是Mac系统，fresh热编译可能不兼容，你可以选择air实现文件实时热重载
## 1.1 fresh 热更新-热加载-边开发边编译
```
go install github.com/pilu/fresh@latest
```
根目录下的 runner.conf 是fresh热编译配置文件

fresh的runner.conf配置说明：
```
 # 监听文件后缀
 valid_ext .go
 # 忽略监听的文件
 no_rebuild_ext .tpl, .tmpl, .html, .js, .ts, .vue, .env
 # 忽略监听的目录
 ignored views, tmp, utils, resource, devsource, runtime
```
## 1.2 使用air实现文件实时热重载
``` 
go install github.com/cosmtrek/air@latest

```
根目录下的 .air.conf 是air的配置文件
## 2 初始化mod
```
go mod tidy
```
如果安装时报错：
> https://mirrors.aliyun.com/goproxy/sumdb/sum.golang.org/lookup/gopkg.in/natefinch/lumberjack.v2@v2.2.1+incompatible: 404 Not Found
则运行如下命令(Windows / Mac / Linux 通用命令)：
```
go env -w GOPROXY=https://goproxy.cn,direct
go env -w GOPRIVATE=
go clean -modcache
```
## 3 热编译运行
```
 fresh 
```
热加载使用fresh或者air都可以两者区别不大，air配置比较直观，根据自己喜好选择。如果你是mac系统开发 则fresh会有问题，所以mac可选择air或者其他热编译工具。

注意：如果热加载太慢，请把views内的前端代码移到go项目目录外，这样就可快速热加载了，加快开发速度，移除后记得把resource\config.yaml下的vueobjroot修改成新目录路径。
# 二、打包
## 开发电脑系统为：windows
### 1 windows系统下打包在windows系统程序
```
go build main.go
```
### 2 windows系统下打包在Linux上运行（推荐）
（此时会打包成Linux上可运行的二进制文件，不带后缀名的文件）
```
SET GOOS=linux
SET GOARCH=amd64
go build

# 如果修改编译生成程序名称(推荐)执行如下
SET GOOS=linux
SET GOARCH=amd64
go build -o goflygen

```
如果想缩小体积使用下面命令，这行标准压缩编译命令，几乎不影响性能：
上面命令加上：-ldflags "-s -w"
```
SET GOOS=linux
SET GOARCH=amd64
go build -ldflags "-s -w"
```
### 3 切换回widows系统程序
```
// 配置环境变量
SET CGO_ENABLED=1
SET GOOS=windows
SET GOARCH=amd64

// 编译命令
go build main.go

go build -o goflygen.exe main.go
```
### 7 编译成Linux环境可执行文件

```
// 配置参数
SET CGO_ENABLED=0 
SET GOOS=linux 
SET GOARCH=amd64 
// 编译命令
go build main.go

```
## 开发电脑系统为：mac
```
// 配置参数
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build
// 或者
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build main.go

```