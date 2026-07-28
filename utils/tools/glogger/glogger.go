// ======================================
// Zap 日志处理组件，可以日志切割、归档、压缩、自定义格式
// 调用示例：
// Info("服务启动成功") 或 Info("服务启动成功",  zap.String("version", "v1.0.0"), zap.Int("uid", 10086), zap.String("ip", "127.0.0.1"))
// Debug("调试信息", zap.Bool("is_debug", true))
// Warn("资源占用过高", zap.Int("cpu", 90))
// Error("请求失败", zap.String("url", "/api/test"), zap.String("err", "timeout"))
// ======================================
package glogger

import (
	"context"
	"gofly/utils/tools/gcfg"
	"gofly/utils/tools/gconv"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// 配置
var (
	logConf, _  = gcfg.Instance("app").Get(context.Background(), "logger")
	logConf_arr = gconv.Map(logConf)
	logDir      = gconv.String(logConf_arr["path"])
)

// getLogFile 按日期生成日志路径
func getLogFile(logdir string) string {
	_ = os.MkdirAll(logdir, 0755)
	return filepath.Join(logdir, time.Now().Format("2006-01-02")+".log")
}

// 自定义：正常人类时间格式 2006-01-02 15:04:05
func customTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("2006-01-02 15:04:05"))
}

// 【定时】每天指定时间执行一次（零性能开销）
func StartCleanCron() {
	go func() {
		// 固定 24 小时周期定时器
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		// 第一次执行：等待到今晚指定时间点
		now := time.Now()
		firstRun := time.Date(now.Year(), now.Month(), now.Day(), gconv.Int(logConf_arr["cleanhour"]), 0, 0, 0, now.Location())
		if now.After(firstRun) {
			firstRun = firstRun.Add(24 * time.Hour)
		}
		time.Sleep(firstRun.Sub(now))
		cleanExpiredLogs()

		// 之后每天 logConf_arr["cleanhour"] 点执行
		for range ticker.C {
			cleanExpiredLogs()
		}
	}()
}

// 自动清理过期日志
func cleanExpiredLogs() {
	KeepDays := gconv.Int(logConf_arr["keepdays"])
	dir, err := os.Open(logDir)
	if err != nil {
		return
	}
	defer dir.Close()
	files, err := dir.Readdir(-1)
	if err != nil {
		return
	}

	expiredTime := time.Now().AddDate(0, 0, -KeepDays)

	for _, f := range files {
		if f.IsDir() {
			continue
		}

		fileName := f.Name()
		if filepath.Ext(fileName) != ".log" {
			continue
		}
		dateStr := strings.TrimSuffix(fileName, ".log")

		logDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue // 不是日期命名的日志，跳过不删
		}

		// 根据文件名日期判断是否删除
		if logDate.Before(expiredTime) {
			// 删除过期日志
			_ = os.Remove(filepath.Join(logDir, fileName))
		}
	}
}

// writeLog 直接写入文件，写完立即关闭（真正释放句柄）
func writeLog(level zapcore.Level, msg string, fields ...zap.Field) {
	if gconv.Bool(logConf_arr["open"]) == false {
		return
	}
	// 打开文件
	logFile := getLogFile(logDir)
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return
	}

	// 写完立即关闭，释放文件句柄
	defer func() {
		_ = file.Sync()
		_ = file.Close()
	}()

	// zap 编码器配置
	encConfig := zap.NewProductionEncoderConfig()
	encConfig.EncodeTime = customTimeEncoder            //正常时间
	encConfig.EncodeLevel = zapcore.CapitalLevelEncoder //级别改为大写
	encConfig.EncodeCaller = zapcore.FullCallerEncoder  //调用者显示全路径
	encConfig.FunctionKey = "func"                      //增加函数名

	//输出日志格式
	var encoder zapcore.Encoder
	if gconv.String(logConf_arr["type"]) == "JSON" {
		encoder = zapcore.NewJSONEncoder(encConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encConfig)
	}

	core := zapcore.NewCore(
		encoder,
		zapcore.AddSync(file),
		getZapLevel(gconv.String(logConf_arr["level"])),
	)

	logger := zap.New(core, zap.AddCallerSkip(2))

	switch level {
	case zap.DebugLevel:
		logger.Debug(msg, fields...)
	case zap.InfoLevel:
		logger.Info(msg, fields...)
	case zap.WarnLevel:
		logger.Warn(msg, fields...)
	case zap.ErrorLevel:
		logger.Error(msg, fields...)
	}

	_ = logger.Sync()
}

// 获取日志级别
func getZapLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

// 纯文本日志（无 JSON 格式，直接写入原始文字）
func writeTextLog(msg string, path ...string) {
	pathDir := logDir
	if len(path) > 0 {
		pathDir = path[0]
	}
	// 打开文件
	logFile := getLogFile(pathDir)
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return
	}
	// 写完立即关闭，释放文件句柄
	defer func() {
		_ = file.Sync()
		_ = file.Close()
	}()

	encConfig := zapcore.EncoderConfig{
		TimeKey:        "", // 关闭时间
		LevelKey:       "", // 关闭级别
		NameKey:        "",
		CallerKey:      "",    // 关闭文件路径
		FunctionKey:    "",    // 关闭函数名
		MessageKey:     "msg", // 只保留日志文字
		StacktraceKey:  "",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    nil,
		EncodeTime:     nil,
		EncodeDuration: nil,
		EncodeCaller:   nil,
	}
	encoder := zapcore.NewConsoleEncoder(encConfig)

	core := zapcore.NewCore(
		encoder,
		zapcore.AddSync(file),
		zapcore.InfoLevel,
	)

	logger := zap.New(core, zap.AddCallerSkip(2))
	logger.Info(msg)
	logger.Sync()
}

// ===============对外调用===============

// Text 纯文本日志：msg写入的内容,path可置日志路径(可选)
func Text(msg string, path ...string) {
	writeTextLog(msg, path...)
}

// Debug 调试日志
func Debug(msg string, fields ...zap.Field) {
	writeLog(zap.DebugLevel, msg, fields...)
}

// Info 普通日志
func Info(msg string, fields ...zap.Field) {
	writeLog(zap.InfoLevel, msg, fields...)
}

// Warn 警告日志
func Warn(msg string, fields ...zap.Field) {
	writeLog(zap.WarnLevel, msg, fields...)
}

// Error 错误日志
func Error(msg string, fields ...zap.Field) {
	writeLog(zap.ErrorLevel, msg, fields...)
}
