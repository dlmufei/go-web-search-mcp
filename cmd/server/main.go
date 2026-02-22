package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/cliffyan/go-web-search-mcp/internal/config"
	"github.com/cliffyan/go-web-search-mcp/internal/engine"
	"github.com/cliffyan/go-web-search-mcp/internal/server"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("🔍 Starting go-web-search MCP Server...")

	// 加载配置
	cfg := config.Load()

	// 初始化搜索引擎管理器
	engineManager := engine.NewManager(cfg)

	// 创建并启动服务器
	srv := server.New(cfg, engineManager)

	// 优雅关闭
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("🛑 Shutting down server...")
		os.Exit(0)
	}()

	// 启动服务器
	if err := srv.Start(); err != nil {
		log.Fatalf("❌ Server failed: %v", err)
	}
}
