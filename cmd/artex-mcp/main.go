// Command artex-mcp is a zero-dependency MCP stdio server that exposes a
// deployed ARTEX instance's REST API as MCP tools, so an agent can create,
// inspect and control ARTEX exploration tasks from a normal conversation.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"artex-mcp/internal/artex"
	"artex-mcp/internal/config"
	"artex-mcp/internal/mcp"
	"artex-mcp/internal/tools"
)

// version is injected at build time:
//
//	go build -ldflags "-X main.version=$(git describe --tags --always)"
var version = "dev"

func main() {
	var (
		configPath = flag.String("config", "", "凭据文件（默认 ~/.config/artex-mcp/config.json）")
		baseURL    = flag.String("base", "", "ARTEX 地址（默认 http://127.0.0.1:8787）")
		check      = flag.Bool("check", false, "检查连通性与登录后退出")
		listTools  = flag.Bool("list-tools", false, "列出全部工具后退出")
		showVer    = flag.Bool("version", false, "打印版本后退出")
	)
	flag.Parse()

	log.SetOutput(os.Stderr)
	log.SetPrefix("artex-mcp: ")

	if *showVer {
		fmt.Printf("artex-mcp %s (%d tools, %d routes)\n", version, len(tools.All()), tools.RouteCount())
		return
	}
	if *listTools {
		defs := tools.Defs()
		b, _ := json.MarshalIndent(defs, "", "  ")
		fmt.Println(string(b))
		return
	}

	path := *configPath
	if path == "" {
		path = config.DefaultPath()
	}
	cfg, err := config.Load(path)
	if err != nil {
		log.Fatalf("读取配置失败: %v", err)
	}
	if *baseURL != "" {
		cfg.BaseURL = *baseURL
	}

	api := artex.New(cfg.BaseURL, cfg.Username, cfg.Password, cfg.Token)
	if cfg.Password == "" && cfg.Token == "" && !*check {
		log.Printf("警告: 未配置 ARTEX 密码或 token（配置文件 %s），需要认证的工具会失败", path)
	}

	if *check {
		if err := mcp.Check(context.Background(), api); err != nil {
			fmt.Fprintln(os.Stderr, "check failed:", err)
			os.Exit(1)
		}
		return
	}

	srv := &mcp.Server{API: api, Version: version}
	if err := srv.Serve(os.Stdin, os.Stdout); err != nil && !errors.Is(err, io.EOF) {
		log.Fatalf("stdio 服务退出: %v", err)
	}
}
