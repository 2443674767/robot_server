package admin

/**
* 引入控制器-文件夹名称的路径
* 请把您使用包用 _ "gofly/app/admin/XX"导入您编写的包 自动生成路由
* 不是使用则注释掉
* 即：http://xx.com/admin/article/cate/getList
* 如果文件夹内没有对应package的go文件请把控制器类删除
 */
import (
	_ "gofly/app/admin/common"
	_ "gofly/app/admin/dashboard"
	_ "gofly/app/admin/datacenter"
	_ "gofly/app/admin/developer"
	_ "gofly/app/admin/system"
	_ "gofly/app/admin/user"
	"gofly/utils/gf"
)

// RouterHandler方法是管理后台需要角色权限验证时添加
// 路由中间件/路由钩子，noAuths无需路由验证接口，可以从ctx获取请求各种参数
func RouterHandler(ctx *gf.GinCtx, modelname string) {
	if gf.IsModelPath(ctx.FullPath(), modelname) { //在这里面处理拦截操作，如果需要拦截终止执行则：ctx.Abort()
		// 判断请求接口是否需要验证权限(RBAC的权限)
		if gf.NeedAuthMatch(ctx) {
			haseauth := gf.CheckAuth(ctx, modelname)
			if haseauth {
				ctx.Next()
			} else {
				gf.Failed().SetMsg(gf.LocaleMsg().SetLanguage(ctx.Request.Header.Get("locale")).Message("sys_auth_permission")).SetData(haseauth).Regin(ctx)
				ctx.Abort()
			}
		} else {
			ctx.Next()
		}
	}
}
