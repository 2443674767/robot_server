// ============================
// 遍历目录及文件
// 在插件打包中使用
// ============================
package gf

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Option 遍历选项
type DirOption struct {
	RootPath   []string `yaml:"rootPath"`   // 目标根目录
	SubFlag    bool     `yaml:"subFlag"`    // 遍历子目录标志 true: 遍历 false: 不遍历
	IgnorePath []string `yaml:"ignorePath"` // 忽略目录
	IgnoreFile []string `yaml:"ignoreFile"` // 忽略文件
}

// Node 树节点
type Node struct {
	Name     string  `json:"name"`     // 目录（或文件）名
	Path     string  `json:"path"`     // 目录（或文件）完整路径
	Children []*Node `json:"children"` // 目录下的文件或子目录
	IsDir    bool    `json:"isDir"`    // 是否为目录 true: 是目录 false: 不是目录
}

// TraverVueDir-遍历指定路径多个目录，获取目录和文件数据
//
//	DirOption : 遍历选项，pathroot：指定的路径
//	tree : 遍历结果
func TraverPathDir(option DirOption, pathroot string) (Node, error) {
	// 根节点
	var root Node
	if _, err := os.Stat(pathroot); err != nil && os.IsNotExist(err) {
		root.Children = make([]*Node, 0)
		return root, nil
	}
	// 多个目录搜索
	for _, p := range option.RootPath {
		// 空目录跳过
		if strings.TrimSpace(p) == "" {
			continue
		}
		var child Node
		// 目录路径
		child.Path = p
		// 递归
		explorerRecursive(&child, &option, pathroot)
		root.Children = append(root.Children, &child)
	}
	return root, nil
}

// TraverDir-遍历当前程序运行目录下多个目录，获取目录和文件数据
//
//	DirOption : 遍历选项
//	tree : 遍历结果
func TraverDir(option DirOption) (Node, error) {
	// 根节点
	var root Node
	pathroot, _ := os.Getwd()
	// 多个目录搜索
	for _, p := range option.RootPath {
		// 空目录跳过
		if strings.TrimSpace(p) == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(pathroot, p)); err != nil && os.IsNotExist(err) {
			continue
		}
		var child Node
		// 目录路径
		child.Path = p
		// 递归
		explorerRecursive(&child, &option, pathroot)
		root.Children = append(root.Children, &child)
	}
	return root, nil
}

// 递归遍历目录
//
//	node : 目录节点
//	option : 遍历选项
func explorerRecursive(node *Node, option *DirOption, pathroot string) error {
	filePath := filepath.Join(pathroot, node.Path)
	// 节点的信息
	p, err := os.Stat(filePath)
	if err != nil {
		return err
	}
	// 目录（或文件）名
	node.Name = p.Name()
	// 是否为目录
	node.IsDir = p.IsDir()

	// 非目录，返回
	if !p.IsDir() {
		return errors.New("非目录")
	}
	// 目录中的文件和子目录
	sub, err := os.ReadDir(filePath)
	if err != nil {
		return fmt.Errorf("目录不存在，或打开错误: %v", err)
	}

	for _, f := range sub {
		tmp := path.Join(node.Path, f.Name())
		var child Node
		// 完整子目录
		child.Path = tmp
		// 是否为目录
		child.IsDir = f.IsDir()
		// 目录
		if f.IsDir() {
			//查找子目录
			if option.SubFlag {
				// 不在忽略目录中的目录，进行递归查找
				if !IsInSlice(option.IgnorePath, f.Name()) {
					node.Children = append(node.Children, &child)
					explorerRecursive(&child, option, pathroot)
				}
			}
		} else { // 文件
			// 非忽略文件，添加到结果中
			if !IsInSlice(option.IgnoreFile, f.Name()) {
				child.Name = f.Name()
				node.Children = append(node.Children, &child)
			}
		}
	}
	return nil
}

// GetAllDirs 获取文文件夹全部目录
func GetAllDirs(pathname string) ([]string, error) {
	rd, err := os.ReadDir(pathname)
	var folders = make([]string, 0)
	if err != nil {
		return folders, err
	}
	for _, fi := range rd {
		if fi.IsDir() {
			folders = append(folders, fi.Name())
		}
	}
	return folders, nil
}
