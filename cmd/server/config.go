package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultAddress = "127.0.0.1:19081"

type config struct {
	Address         string
	DataDirectory   string
	SelfCheck       bool
	ShutdownTimeout time.Duration
}

func parseConfig(args []string, getenv func(string) string) (config, error) {
	addressDefault := defaultAddress
	if rawPort := strings.TrimSpace(getenv("PORT")); rawPort != "" {
		port, err := strconv.Atoi(rawPort)
		if err != nil || port < 1 || port > 65535 {
			return config{}, fmt.Errorf("PORT 必须是 1 到 65535 的端口号")
		}
		addressDefault = net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	}
	set := flag.NewFlagSet("oral-history-release-desk", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	address := set.String("addr", addressDefault, "回环监听地址，例如 127.0.0.1:19081")
	dataDirectory := set.String("data", "./data", "本地账本数据目录")
	selfCheck := set.Bool("selfcheck", false, "运行真实 HTTP 完整流程后退出")
	shutdownTimeout := set.Duration("shutdown-timeout", 5*time.Second, "优雅关闭超时")
	if err := set.Parse(args); err != nil {
		return config{}, err
	}
	if set.NArg() != 0 {
		return config{}, fmt.Errorf("不支持位置参数: %s", strings.Join(set.Args(), " "))
	}
	if err := validateAddress(*address); err != nil {
		return config{}, err
	}
	if strings.TrimSpace(*dataDirectory) == "" {
		return config{}, fmt.Errorf("数据目录不能为空")
	}
	if *shutdownTimeout <= 0 || *shutdownTimeout > time.Minute {
		return config{}, fmt.Errorf("shutdown-timeout 必须在 0 到 1 分钟之间")
	}
	return config{Address: *address, DataDirectory: *dataDirectory, SelfCheck: *selfCheck, ShutdownTimeout: *shutdownTimeout}, nil
}

func validateAddress(address string) error {
	host, rawPort, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("addr 必须为 host:port: %w", err)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("addr 必须使用明确的回环 IP，不能绑定公共网卡")
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("addr 端口必须在 1 到 65535 之间")
	}
	return nil
}
