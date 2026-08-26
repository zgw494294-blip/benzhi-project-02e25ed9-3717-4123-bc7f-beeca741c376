package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"oral-history-release-desk/internal/policy"
	"oral-history-release-desk/internal/release"
	"oral-history-release-desk/internal/store"
	webtransport "oral-history-release-desk/internal/web"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Printf("服务失败: %v", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := parseConfig(args, os.Getenv)
	if err != nil {
		return err
	}
	dataDirectory := cfg.DataDirectory
	if cfg.SelfCheck {
		temporary, err := os.MkdirTemp("", "oral-history-selfcheck-*")
		if err != nil {
			return fmt.Errorf("创建自检数据目录: %w", err)
		}
		defer os.RemoveAll(temporary)
		dataDirectory = temporary
	}
	repository, err := store.Open(dataDirectory)
	if err != nil {
		return fmt.Errorf("打开本地账本: %w", err)
	}
	defer repository.Close()
	service := release.NewService(repository, policy.New())
	transport := webtransport.New(service)
	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return fmt.Errorf("监听 %s: %w", cfg.Address, err)
	}
	server := &http.Server{
		Handler:           transport.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    32 << 10,
	}
	serveErrors := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if !errors.Is(err, http.ErrServerClosed) {
			serveErrors <- err
		}
		close(serveErrors)
	}()
	log.Printf("口述史开放治理台监听于 http://%s", listener.Addr().String())
	if cfg.SelfCheck {
		checkErr := runSelfCheck(listener.Addr().String())
		shutdownErr := shutdown(server, cfg.ShutdownTimeout)
		if checkErr != nil {
			return fmt.Errorf("自检失败: %w", checkErr)
		}
		if shutdownErr != nil {
			return shutdownErr
		}
		if err := <-serveErrors; err != nil {
			return fmt.Errorf("HTTP 服务异常退出: %w", err)
		}
		log.Printf("自检成功：建案、校核、处置、复核、批准与摘要验证均已完成")
		return nil
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	select {
	case signal := <-signals:
		log.Printf("收到 %s，准备关闭", signal)
	case err := <-serveErrors:
		if err != nil {
			return fmt.Errorf("HTTP 服务异常退出: %w", err)
		}
		return nil
	}
	return shutdown(server, cfg.ShutdownTimeout)
}

func shutdown(server *http.Server, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("优雅关闭 HTTP 服务: %w", err)
	}
	return nil
}
