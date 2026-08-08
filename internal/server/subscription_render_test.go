package server

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/hyfleet/hyfleet/internal/store"
	"go.yaml.in/yaml/v3"
)

func TestSubscriptionRenderersEscapeStructuredValues(t *testing.T) {
	subscription := store.Subscription{Endpoints: []store.SubscriptionEndpoint{
		{
			NodeID: "node-one", NodeName: "IPv6 / Tokyo #1", PublicHost: "2001:db8::1",
			PublicPort: 8443, SNI: "edge.example.com", TLSInsecure: true,
			Credential: "user:p@ss/?# value",
		},
	}}

	uriDocument, err := renderSubscription("uri", subscription)
	if err != nil {
		t.Fatalf("renderSubscription(uri) error = %v", err)
	}
	parsed, err := url.Parse(string(uriDocument.Body))
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	if parsed.Scheme != "hysteria2" || parsed.Hostname() != "2001:db8::1" || parsed.Port() != "8443" ||
		parsed.User.Username() != "user:p@ss/?# value" || parsed.Fragment != "IPv6 / Tokyo #1" ||
		parsed.Query().Get("sni") != "edge.example.com" || parsed.Query().Get("insecure") != "1" {
		t.Fatalf("unexpected Hysteria2 URI: %s", uriDocument.Body)
	}

	encoded, err := renderSubscription("base64", subscription)
	if err != nil {
		t.Fatalf("renderSubscription(base64) error = %v", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(string(encoded.Body))
	if err != nil || string(decoded) != string(uriDocument.Body) {
		t.Fatalf("base64 decoded = %q, error = %v", decoded, err)
	}

	clash, err := renderSubscription("clash", subscription)
	if err != nil {
		t.Fatalf("renderSubscription(clash) error = %v", err)
	}
	var clashValue struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	if err := yaml.Unmarshal(clash.Body, &clashValue); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v; body = %s", err, clash.Body)
	}
	if len(clashValue.Proxies) != 1 || clashValue.Proxies[0]["password"] != "user:p@ss/?# value" ||
		clashValue.Proxies[0]["server"] != "2001:db8::1" || clashValue.Proxies[0]["skip-cert-verify"] != true {
		t.Fatalf("unexpected Clash subscription: %#v", clashValue)
	}

	singBox, err := renderSubscription("sing-box", subscription)
	if err != nil {
		t.Fatalf("renderSubscription(sing-box) error = %v", err)
	}
	var singBoxValue struct {
		Outbounds []struct {
			Tag      string `json:"tag"`
			Password string `json:"password"`
			TLS      struct {
				Enabled    bool   `json:"enabled"`
				ServerName string `json:"server_name"`
				Insecure   bool   `json:"insecure"`
			} `json:"tls"`
		} `json:"outbounds"`
	}
	if err := json.Unmarshal(singBox.Body, &singBoxValue); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; body = %s", err, singBox.Body)
	}
	if len(singBoxValue.Outbounds) != 1 || singBoxValue.Outbounds[0].Tag != "IPv6 / Tokyo #1" ||
		singBoxValue.Outbounds[0].Password != "user:p@ss/?# value" ||
		!singBoxValue.Outbounds[0].TLS.Enabled || !singBoxValue.Outbounds[0].TLS.Insecure ||
		singBoxValue.Outbounds[0].TLS.ServerName != "edge.example.com" {
		t.Fatalf("unexpected sing-box subscription: %#v", singBoxValue)
	}
}

func TestSubscriptionRenderersProduceValidEmptyDocuments(t *testing.T) {
	empty := store.Subscription{Endpoints: []store.SubscriptionEndpoint{}}
	for _, format := range []string{"uri", "base64"} {
		rendered, err := renderSubscription(format, empty)
		if err != nil || len(rendered.Body) != 0 {
			t.Fatalf("renderSubscription(%s) = %q, error = %v", format, rendered.Body, err)
		}
	}
	clash, err := renderSubscription("clash", empty)
	if err != nil || !strings.Contains(string(clash.Body), "proxies: []") {
		t.Fatalf("empty Clash = %q, error = %v", clash.Body, err)
	}
	singBox, err := renderSubscription("sing-box", empty)
	if err != nil || !strings.Contains(string(singBox.Body), `"outbounds": []`) {
		t.Fatalf("empty sing-box = %q, error = %v", singBox.Body, err)
	}
}
