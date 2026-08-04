package control

import (
	"strings"

	internalcontrol "gofly/app/robotdog/internal/control"
	"gofly/dao"
	"gofly/utils/gf"
)

type Dog struct{ NoNeedAuths []string }

func init() {
	gf.Register(&Dog{})
}

func (api *Dog) Move(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	dogID := gf.Int64(param["dog_id"])
	if dogID == 0 {
		gf.Failed().SetMsg("机械狗ID不能为空").Regin(ctx)
		return
	}
	direction := normalizeDirection(gf.String(param["direction"]))
	if direction == "" {
		gf.Failed().SetMsg("方向参数不能为空").Regin(ctx)
		return
	}
	dogDB := dao.Query().RobotdogDog
	dog, err := dogDB.WithContext(ctx).Where(dogDB.ID.Eq(dogID), dogDB.TenantID.Eq(tenantID(ctx, param))).First()
	if err != nil {
		gf.Failed().SetMsg("机械狗不存在").SetData(err).Regin(ctx)
		return
	}
	driver, driverName, err := internalcontrol.DogDriver(dog.Model)
	if err != nil {
		gf.Failed().SetMsg("获取机械狗驱动失败").SetData(err).Regin(ctx)
		return
	}
	target := internalcontrol.DogTarget{
		ID:      dog.ID,
		Name:    dog.Name,
		Model:   dog.Model,
		UDPHost: dog.UdpHost,
		UDPPort: dog.UdpPort,
	}
	opts := internalcontrol.MoveOptions{
		Direction: direction,
		Speed:     gf.Float64(param["speed"]),
		Duration:  gf.Int(param["duration"]),
	}
	var result *internalcontrol.CommandResult
	switch direction {
	case "left":
		result, err = driver.MoveLeft(ctx, target, opts)
	case "right":
		result, err = driver.MoveRight(ctx, target, opts)
	case "forward":
		result, err = driver.MoveForward(ctx, target, opts)
	case "backward":
		result, err = driver.MoveBackward(ctx, target, opts)
	case "stop":
		result, err = driver.Stop(ctx, target)
	default:
		gf.Failed().SetMsg("不支持的机械狗方向: " + direction).Regin(ctx)
		return
	}
	if err != nil {
		gf.Failed().SetMsg("发送机械狗UDP指令失败").SetData(err).Regin(ctx)
		return
	}
	result.Driver = driverName
	gf.Success().SetMsg("机械狗UDP指令已发送").SetData(result).Regin(ctx)
}

func (api *Dog) GetRealtime(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	dogID := gf.Int64(param["dog_id"])
	if dogID == 0 {
		gf.Failed().SetMsg("机械狗ID不能为空").Regin(ctx)
		return
	}
	dogDB := dao.Query().RobotdogDog
	dog, err := dogDB.WithContext(ctx).Where(dogDB.ID.Eq(dogID), dogDB.TenantID.Eq(tenantID(ctx, param))).First()
	if err != nil {
		gf.Failed().SetMsg("机械狗不存在").SetData(err).Regin(ctx)
		return
	}
	driver, driverName, err := internalcontrol.DogDriver(dog.Model)
	if err != nil {
		gf.Failed().SetMsg("获取机械狗驱动失败").SetData(err).Regin(ctx)
		return
	}
	data, err := driver.Realtime(ctx, internalcontrol.DogTarget{
		ID:      dog.ID,
		Name:    dog.Name,
		Model:   dog.Model,
		UDPHost: dog.UdpHost,
		UDPPort: dog.UdpPort,
	})
	if err != nil {
		gf.Failed().SetMsg("获取机械狗实时数据失败").SetData(err).Regin(ctx)
		return
	}
	data.Driver = driverName
	gf.Success().SetMsg("获取机械狗实时数据").SetData(data).Regin(ctx)
}

func tenantID(ctx *gf.GinCtx, param map[string]interface{}) int32 {
	id := ctx.GetInt32("tenant_id")
	if id == 0 {
		id = gf.Int32(param["tenant_id"])
	}
	if id == 0 {
		id = 1
	}
	return id
}

func normalizeDirection(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	switch v {
	case "left", "l", "move_left":
		return "left"
	case "right", "r", "move_right":
		return "right"
	case "forward", "front", "up", "w", "move_forward":
		return "forward"
	case "backward", "back", "down", "s", "move_backward":
		return "backward"
	case "stop", "halt":
		return "stop"
	default:
		return v
	}
}
