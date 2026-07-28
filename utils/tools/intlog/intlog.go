package intlog

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"time"

	"go.opentelemetry.io/otel/trace"
)

const (
	stackFilterKey = "/gofly/intlog"
)

// Print prints `v` with newline using fmt.Println.
// The parameter `v` can be multiple variables.
func Print(ctx context.Context, v ...interface{}) {
	doPrint(ctx, fmt.Sprint(v...))
}

// Printf prints `v` with format `format` using fmt.Printf.
// The parameter `v` can be multiple variables.
func Printf(ctx context.Context, format string, v ...interface{}) {
	doPrint(ctx, fmt.Sprintf(format, v...))
}

// Error prints `v` with newline using fmt.Println.
// The parameter `v` can be multiple variables.
func Error(ctx context.Context, v ...interface{}) {
	doPrint(ctx, fmt.Sprint(v...))
}

// Errorf prints `v` with format `format` using fmt.Printf.
func Errorf(ctx context.Context, format string, v ...interface{}) {
	doPrint(ctx, fmt.Sprintf(format, v...))
}

// PrintFunc prints the output from function `f`.
// It only calls function `f` if debug mode is enabled.
func PrintFunc(ctx context.Context, f func() string) {
	s := f()
	if s == "" {
		return
	}
	doPrint(ctx, s)
}

// ErrorFunc prints the output from function `f`.
// It only calls function `f` if debug mode is enabled.
func ErrorFunc(ctx context.Context, f func() string) {
	s := f()
	if s == "" {
		return
	}
	doPrint(ctx, s)
}

func doPrint(ctx context.Context, content string) {
	buffer := bytes.NewBuffer(nil)
	buffer.WriteString(time.Now().Format("2006-01-02 15:04:05.000"))
	buffer.WriteString(" [INTE] ")
	buffer.WriteString(file())
	buffer.WriteString(" ")
	if s := traceIdStr(ctx); s != "" {
		buffer.WriteString(s + " ")
	}
	buffer.WriteString(content)
	buffer.WriteString("\n")
	fmt.Print(buffer.String())
}

// traceIdStr retrieves and returns the trace id string for logging output.
func traceIdStr(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	spanCtx := trace.SpanContextFromContext(ctx)
	if traceId := spanCtx.TraceID(); traceId.IsValid() {
		return "{" + traceId.String() + "}"
	}
	return ""
}

// 返回调用者的文件名及其所在的行号
// file returns caller file name along with its line number.
func file() string {
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		return "unknown:0"
	}
	return fmt.Sprintf(`%s:%d`, filepath.Base(file), line)
}
