package main

import "testing"

func TestConfigUsesLoopbackPortEnvironment(t *testing.T) {
	cfg, err := parseConfig(nil, func(key string) string {
		if key == "PORT" {
			return "19123"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Address != "127.0.0.1:19123" {
		t.Fatalf("PORT 地址错误: %s", cfg.Address)
	}
	if _, err := parseConfig([]string{"-addr=0.0.0.0:19081"}, func(string) string { return "" }); err == nil {
		t.Fatal("公共网卡监听应被拒绝")
	}
}
