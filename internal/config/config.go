package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	HTTPAddr string
	SMTPAddr string
	DBPath   string
}

func Load() (Config, error) { return load(os.LookupEnv) }

func load(lookup func(string) (string, bool)) (Config, error) {
	c := Config{HTTPAddr: "127.0.0.1:8025", SMTPAddr: "127.0.0.1:1025", DBPath: "./data/testinbox.db"}
	for _, setting := range []struct {
		name   string
		target *string
	}{
		{"TESTINBOX_HTTP_ADDR", &c.HTTPAddr}, {"TESTINBOX_SMTP_ADDR", &c.SMTPAddr}, {"TESTINBOX_DB_PATH", &c.DBPath},
	} {
		if value, ok := lookup(setting.name); ok {
			*setting.target = value
		}
		if strings.TrimSpace(*setting.target) == "" || strings.TrimSpace(*setting.target) != *setting.target || strings.ContainsAny(*setting.target, "\x00\r\n") {
			return Config{}, fmt.Errorf("%s must be non-empty without surrounding whitespace or control characters", setting.name)
		}
		if setting.name == "TESTINBOX_DB_PATH" {
			continue
		}
		host, port, err := net.SplitHostPort(*setting.target)
		n, portErr := strconv.Atoi(port)
		if err != nil || net.ParseIP(host) == nil || portErr != nil || n < 1 || n > 65535 {
			return Config{}, fmt.Errorf("%s must be an IP address and numeric port from 1 to 65535 (IPv6 uses brackets)", setting.name)
		}
	}
	return c, nil
}
