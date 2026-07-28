// ---
// ipregion database v2.0 searcher.
// @Note ip地址解析属地区
//
// @Author gofly
// @Date   2024/08/19
package plugin

import (
	"gofly/utils/plugin/ipregion"
)

// 获取ip属地,返回：中国|云南省|昆明市|移动
func NewIpRegion(ip string) (string, error) {
	searcher, err := ipregion.NewWithFileOnly()
	if err != nil {
		return "", err
	}
	defer searcher.Close()
	if ip == "::1" {
		ip = "127.0.0.1"
	}
	region, err := searcher.SearchByStr(ip)
	if err != nil {
		return "", err
	}
	return region, nil
}
