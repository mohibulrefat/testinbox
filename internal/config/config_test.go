package config

import "testing"

func TestLoad(t *testing.T) {
	empty := func(string) (string, bool) { return "", false }
	c, err := load(empty)
	if err != nil || c.HTTPAddr != "127.0.0.1:8025" || c.SMTPAddr != "127.0.0.1:1025" || c.DBPath != "./data/testinbox.db" {
		t.Fatalf("defaults: %+v %v", c, err)
	}
	for _, tc := range []struct{ name, value string }{
		{"TESTINBOX_HTTP_ADDR", ""}, {"TESTINBOX_HTTP_ADDR", ":8025"}, {"TESTINBOX_HTTP_ADDR", "localhost:8025"}, {"TESTINBOX_HTTP_ADDR", "127.0.0.1:0"}, {"TESTINBOX_HTTP_ADDR", "127.0.0.1:65536"}, {"TESTINBOX_HTTP_ADDR", "127.0.0.1:http"}, {"TESTINBOX_SMTP_ADDR", "garbage"}, {"TESTINBOX_DB_PATH", " "}, {"TESTINBOX_DB_PATH", "bad\x00path"},
	} {
		t.Run(tc.name+tc.value, func(t *testing.T) {
			_, err := load(func(name string) (string, bool) { return tc.value, name == tc.name })
			if err == nil {
				t.Fatal("expected invalid configuration")
			}
		})
	}
	values := map[string]string{"TESTINBOX_HTTP_ADDR": "[::1]:8026", "TESTINBOX_SMTP_ADDR": "0.0.0.0:1026", "TESTINBOX_DB_PATH": "/tmp/inbox.db"}
	c, err = load(func(name string) (string, bool) { v, ok := values[name]; return v, ok })
	if err != nil || c.HTTPAddr != values["TESTINBOX_HTTP_ADDR"] || c.SMTPAddr != values["TESTINBOX_SMTP_ADDR"] || c.DBPath != values["TESTINBOX_DB_PATH"] {
		t.Fatalf("overrides: %+v %v", c, err)
	}
}
