package main

import "testing"

func TestApplyNameTemplate(t *testing.T) {
	cfg := &ProxyConfig{Protocol: "vless", Remark: "MyNode"}
	cases := []struct {
		tmpl string
		want string
	}{
		{"", "MyNode @ 1.2.3.4:443"},
		{"{remark}-{ip}", "MyNode-1.2.3.4"},
		{"{proto} {ep} #{n}", "vless 1.2.3.4:443 #7"},
		{"{ip}:{port}", "1.2.3.4:443"},
	}
	for _, c := range cases {
		got := applyNameTemplate(c.tmpl, cfg, "1.2.3.4", 443, 7)
		if got != c.want {
			t.Errorf("applyNameTemplate(%q) = %q, want %q", c.tmpl, got, c.want)
		}
	}

	// Empty remark + empty template falls back to the bare endpoint.
	bare := &ProxyConfig{Protocol: "trojan"}
	if got := applyNameTemplate("", bare, "5.6.7.8", 8443, 1); got != "5.6.7.8:8443" {
		t.Errorf("empty-remark fallback = %q, want %q", got, "5.6.7.8:8443")
	}
}
