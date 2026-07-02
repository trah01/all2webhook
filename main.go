package main

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

//go:embed templates/* static/*
var templatesFS embed.FS

// ===================== 主函数 =====================

func main() {
	// 初始化
	initDB()
	loadConfig()

	addLog("系统启动", "info")

	// 启动后台任务
	startBackgroundTasks()

	// 启动 Web 服务
	adminRouter := gin.Default()

	// 设置模板
	tmpl := template.Must(template.New("").ParseFS(templatesFS, "templates/*.html"))
	adminRouter.SetHTMLTemplate(tmpl)

	// 设置 API
	setupAPI(adminRouter)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	adminSrv := &http.Server{
		Addr:    ":" + port,
		Handler: adminRouter,
	}

	publicRouter := gin.Default()
	setupPublicAPI(publicRouter)

	publicPort := os.Getenv("PUBLIC_PORT")
	if publicPort == "" {
		publicPort = "8081"
	}
	publicSrv := &http.Server{
		Addr:    ":" + publicPort,
		Handler: publicRouter,
	}

	go func() {
		addLog(fmt.Sprintf("管理服务启动在端口: %s", port), "info")
		if err := adminSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	go func() {
		addLog(fmt.Sprintf("公开接收服务启动在端口: %s", publicPort), "info")
		if err := publicSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// 等待中断信号来优雅地关闭服务器
	quit := make(chan os.Signal, 1)
	// kill 默认是信号 SIGTERM (比如 docker-compose stop)
	// 用户控制台的 CTRL+C 会发送 SIGINT
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	addLog("接收到关闭信号，正在进行优雅停机...", "warning")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := adminSrv.Shutdown(ctx); err != nil {
		log.Fatal("Admin server shutdown:", err)
	}
	if err := publicSrv.Shutdown(ctx); err != nil {
		log.Fatal("Public server shutdown:", err)
	}

	addLog("安全断开数据库连接...", "info")
	if db != nil {
		db.Close()
	}

	time.Sleep(1 * time.Second)
	log.Println("Server exiting")
}
