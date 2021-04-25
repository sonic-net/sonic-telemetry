package dbconfig

import (
	"io"
	"os"
	"testing"
)

func setupMultiNamespace(t *testing.T) func() {
	err := os.MkdirAll("/var/run/redis0/sonic-db/", 0755)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	srcFileName := [2]string{"../testdata/database_global.json", "../testdata/database_config_asic0.json"}
	dstFileName := [2]string{SONIC_DB_GLOBAL_CONFIG_FILE, "/var/run/redis0/sonic-db/database_config_asic0.json"}
	for i := 0; i < len(srcFileName); i++ {
		sourceFileStat, err := os.Stat(srcFileName[i])
		if err != nil {
			t.Fatalf("err: %s", err)
		}

		if !sourceFileStat.Mode().IsRegular() {
			t.Fatalf("err: %s", err)
		}

		source, err := os.Open(srcFileName[i])
		if err != nil {
			t.Fatalf("err: %s", err)
		}
		defer source.Close()

		destination, err := os.Create(dstFileName[i])
		if err != nil {
			t.Fatalf("err: %s", err)
		}
		defer destination.Close()
		_, err = io.Copy(destination, source)
		if err != nil {
			t.Fatalf("err: %s", err)
		}
	}
	/* https://github.com/golang/go/issues/32111 */
	return func() {
		err := os.Remove(SONIC_DB_GLOBAL_CONFIG_FILE)
		if err != nil {
			t.Fatalf("err: %s", err)
		}
		err = os.RemoveAll("/var/run/redis0")
		if err != nil {
			t.Fatalf("err: %s", err)
		}
	}
}
func TestGetDb(t *testing.T) {
	t.Run("Id", func(t *testing.T) {
		db_id := GetDbId("CONFIG_DB", GetDbDefaultNamespace())
		if db_id != 4 {
			t.Fatalf(`Id("") = %d, want 4, error`, db_id)
		}
	})
	t.Run("Sock", func(t *testing.T) {
		sock_path := GetDbSock("CONFIG_DB", GetDbDefaultNamespace())
		if sock_path != "/var/run/redis/redis.sock" {
			t.Fatalf(`Sock("") = %q, want "", error`, sock_path)
		}
	})
	t.Run("AllNamespaces", func(t *testing.T) {
		ns_list := GetDbAllNamespaces()
		if len(ns_list) != 1 {
			t.Fatalf(`AllNamespaces("") = %q, want "1", error`, len(ns_list))
		}
		if !(ns_list[0] == GetDbDefaultNamespace()) {
			t.Fatalf(`AllNamespaces("") = %q, want default, error`, ns_list[0])
		}
	})
	t.Run("TcpAddr", func(t *testing.T) {
		tcp_addr := GetDbTcpAddr("CONFIG_DB", GetDbDefaultNamespace())
		if tcp_addr != "127.0.0.1:6379" {
			t.Fatalf(`TcpAddr("") = %q, want 127.0.0.1:6379, error`, tcp_addr)
		}
	})
}
func TestGetDbMultiNs(t *testing.T) {
	Init()
	cleanupMultiNamespace := setupMultiNamespace(t)
	/* https://www.gopherguides.com/articles/test-cleanup-in-go-1-14*/
	t.Cleanup(cleanupMultiNamespace)
	t.Run("Id", func(t *testing.T) {
		db_id := GetDbId("CONFIG_DB", "asic0")
		if db_id != 4 {
			t.Fatalf(`Id("") = %d, want 4, error`, db_id)
		}
	})
	t.Run("Sock", func(t *testing.T) {
		sock_path := GetDbSock("CONFIG_DB", "asic0")
		if sock_path != "/var/run/redis0/redis.sock" {
			t.Fatalf(`Sock("") = %q, want "", error`, sock_path)
		}
	})
	t.Run("AllNamespaces", func(t *testing.T) {
		ns_list := GetDbAllNamespaces()
		if len(ns_list) != 2 {
			t.Fatalf(`AllNamespaces("") = %q, want "2", error`, len(ns_list))
		}
		if !((ns_list[0] == GetDbDefaultNamespace() && ns_list[1] == "asic0") || (ns_list[0] == "asic0" && ns_list[1] == GetDbDefaultNamespace())) {
			t.Fatalf(`AllNamespaces("") = %q %q, want default and asic0, error`, ns_list[0], ns_list[1])
		}
	})
	t.Run("TcpAddr", func(t *testing.T) {
		tcp_addr := GetDbTcpAddr("CONFIG_DB", "asic0")
		if tcp_addr != "127.0.0.1:6379" {
			t.Fatalf(`TcpAddr("") = %q, want 127.0.0.1:6379, error`, tcp_addr)
		}
	})
}
