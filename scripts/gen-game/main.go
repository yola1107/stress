// gen-game：新增 internal/biz/game/g<ID>/（stub + game.go）并改写 registry.go。
// 用法: GAME_ID=<id> GAME_NAME=<名称> go run ./scripts/gen-game
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const toolVersion = "gen-game (stress scaffold)"

func main() {
	spec, err := loadSpecFromEnv()
	if err != nil {
		die(2, "%s\n", err)
	}
	root, err := findGoModuleRoot()
	if err != nil {
		die(1, "定位模块根失败: %v\n", err)
	}
	if err := writeGameScaffold(root, spec); err != nil {
		die(1, "%v\n", err)
	}
	if err := appendGameToRegistry(root, spec); err != nil {
		die(1, "%v\n", err)
	}
	dir := filepath.Join(root, "internal", "biz", "game", spec.Pkg)
	printSuccess(spec, filepath.ToSlash(dir))
}

func printSuccess(spec *gameSpec, dirSlash string) {
	buildCmd := fmt.Sprintf("go build ./internal/biz/game/%s/...", spec.Pkg)
	fprintf(os.Stdout,
		"注入成功 %s（ID=%d）「%s」\n"+
			"  · 已创建目录: %s\n"+
			"  · 已更新:    internal/biz/game/registry.go\n"+
			"  · 工具: %s\n\n"+
			"建议执行：%s\n",
		spec.Pkg, spec.ID, spec.Name,
		dirSlash,
		toolVersion,
		buildCmd,
	)
}

func die(code int, format string, args ...any) {
	fprintf(os.Stderr, format, args...)
	os.Exit(code)
}

func fprintf(w io.Writer, format string, a ...any) {
	_, _ = fmt.Fprintf(w, format, a...)
}
