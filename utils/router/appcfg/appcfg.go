package appcfg

import (
	"context"
	"gofly/utils/tools/gcfg"
	"gofly/utils/tools/gconv"
)

var (
	appConf, _    = gcfg.Instance("app").Get(context.Background(), "app")
	AppConf_arr   = gconv.Map(appConf)
	OpenLogger, _ = gcfg.Instance("app").Get(context.Background(), "logger.open")
)
