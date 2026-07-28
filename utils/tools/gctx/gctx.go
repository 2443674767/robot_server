// ===================
// 彻底杜绝 context key 冲突
// ===================
package gctx

import (
	"context"

	"github.com/google/uuid"
)

type (
	Ctx     = context.Context
	Context = context.Context
)

type ctxKey string

const ctxKeyTraceID ctxKey = "gofly_trace_id"

// New 创建带 TraceID 的上下文
func New() Context {
	return WithTraceID(context.Background(), uuid.NewString())
}

// WithTraceID 设置 TraceID
func WithTraceID(ctx Context, traceID string) Context {
	return context.WithValue(ctx, ctxKeyTraceID, traceID)
}

// GetTraceID 获取 TraceID
func GetTraceID(ctx Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := ctx.Value(ctxKeyTraceID).(string)
	return id
}
