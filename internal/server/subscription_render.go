package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/hyfleet/hyfleet/internal/store"
	"go.yaml.in/yaml/v3"
)

type renderedSubscription struct {
	ContentType string
	Body        []byte
}

type clashSubscription struct {
	Proxies     []clashHysteria2Proxy `yaml:"proxies"`
	ProxyGroups []clashProxyGroup     `yaml:"proxy-groups"`
	Rules       []string              `yaml:"rules"`
}

type clashProxyGroup struct {
	Name    string   `yaml:"name"`
	Type    string   `yaml:"type"`
	Proxies []string `yaml:"proxies"`
}

type clashHysteria2Proxy struct {
	Name           string `yaml:"name"`
	Type           string `yaml:"type"`
	Server         string `yaml:"server"`
	Port           int    `yaml:"port"`
	Password       string `yaml:"password"`
	SNI            string `yaml:"sni,omitempty"`
	SkipCertVerify bool   `yaml:"skip-cert-verify"`
	Fingerprint    string `yaml:"fingerprint,omitempty"`
}

type singBoxSubscription struct {
	Outbounds []singBoxHysteria2Outbound `json:"outbounds"`
}

type singBoxHysteria2Outbound struct {
	Type       string         `json:"type"`
	Tag        string         `json:"tag"`
	Server     string         `json:"server"`
	ServerPort int            `json:"server_port"`
	Password   string         `json:"password"`
	TLS        singBoxTLSData `json:"tls"`
}

type singBoxTLSData struct {
	Enabled                    bool     `json:"enabled"`
	ServerName                 string   `json:"server_name,omitempty"`
	Insecure                   bool     `json:"insecure"`
	CertificatePublicKeySHA256 []string `json:"certificate_public_key_sha256,omitempty"`
}

func defaultClashRules() []string {
	return []string{
		"DOMAIN-SUFFIX,cn,DIRECT",
		"DOMAIN-KEYWORD,-cn,DIRECT",
		"DOMAIN-SUFFIX,qq.com,DIRECT",
		"DOMAIN-SUFFIX,weixin.qq.com,DIRECT",
		"DOMAIN-SUFFIX,tenpay.com,DIRECT",
		"DOMAIN-SUFFIX,baidu.com,DIRECT",
		"DOMAIN-SUFFIX,bilibili.com,DIRECT",
		"DOMAIN-SUFFIX,bilivideo.com,DIRECT",
		"DOMAIN-SUFFIX,douyin.com,DIRECT",
		"DOMAIN-SUFFIX,iesdouyin.com,DIRECT",
		"DOMAIN-SUFFIX,amemv.com,DIRECT",
		"DOMAIN-SUFFIX,snssdk.com,DIRECT",
		"DOMAIN-SUFFIX,toutiao.com,DIRECT",
		"DOMAIN-SUFFIX,bytedance.com,DIRECT",
		"DOMAIN-SUFFIX,pstatp.com,DIRECT",
		"DOMAIN-SUFFIX,ixigua.com,DIRECT",
		"DOMAIN-SUFFIX,zhihu.com,DIRECT",
		"DOMAIN-SUFFIX,aliyun.com,DIRECT",
		"DOMAIN-SUFFIX,taobao.com,DIRECT",
		"DOMAIN-SUFFIX,tmall.com,DIRECT",
		"DOMAIN-SUFFIX,jd.com,DIRECT",
		"DOMAIN-SUFFIX,360buyimg.com,DIRECT",
		"DOMAIN-SUFFIX,163.com,DIRECT",
		"DOMAIN-SUFFIX,126.com,DIRECT",
		"DOMAIN-SUFFIX,netease.com,DIRECT",
		"DOMAIN-SUFFIX,mi.com,DIRECT",
		"DOMAIN-SUFFIX,xiaomi.com,DIRECT",
		"DOMAIN-SUFFIX,xiaohongshu.com,DIRECT",
		"DOMAIN-SUFFIX,iqiyi.com,DIRECT",
		"DOMAIN-SUFFIX,youku.com,DIRECT",
		"IP-CIDR,10.0.0.0/8,DIRECT,no-resolve",
		"IP-CIDR,172.16.0.0/12,DIRECT,no-resolve",
		"IP-CIDR,192.168.0.0/16,DIRECT,no-resolve",
		"IP-CIDR,127.0.0.0/8,DIRECT,no-resolve",
		"GEOIP,CN,DIRECT",
		"MATCH,HyFleet",
	}
}

func renderSubscription(format string, subscription store.Subscription) (renderedSubscription, error) {
	switch format {
	case "uri":
		return renderedSubscription{
			ContentType: "text/plain; charset=utf-8",
			Body:        []byte(strings.Join(subscriptionURIs(subscription), "\n")),
		}, nil
	case "base64":
		plain := strings.Join(subscriptionURIs(subscription), "\n")
		return renderedSubscription{
			ContentType: "text/plain; charset=utf-8",
			Body:        []byte(base64.StdEncoding.EncodeToString([]byte(plain))),
		}, nil
	case "clash":
		proxies := make([]clashHysteria2Proxy, 0, len(subscription.Endpoints))
		proxyNames := make([]string, 0, len(subscription.Endpoints))
		for _, endpoint := range subscription.Endpoints {
			proxies = append(proxies, clashHysteria2Proxy{
				Name: endpoint.NodeName, Type: "hysteria2", Server: endpoint.PublicHost,
				Port: endpoint.PublicPort, Password: endpoint.Credential, SNI: endpoint.SNI,
				SkipCertVerify: endpoint.TLSInsecure, Fingerprint: endpoint.TLSCertFingerprint,
			})
			proxyNames = append(proxyNames, endpoint.NodeName)
		}
		if len(proxyNames) == 0 {
			proxyNames = append(proxyNames, "DIRECT")
		}
		body, err := yaml.Marshal(clashSubscription{
			Proxies: proxies,
			ProxyGroups: []clashProxyGroup{{
				Name: "HyFleet", Type: "select", Proxies: proxyNames,
			}},
			Rules: defaultClashRules(),
		})
		if err != nil {
			return renderedSubscription{}, fmt.Errorf("encode Clash subscription: %w", err)
		}
		return renderedSubscription{ContentType: "application/yaml; charset=utf-8", Body: body}, nil
	case "sing-box":
		outbounds := make([]singBoxHysteria2Outbound, 0, len(subscription.Endpoints))
		for _, endpoint := range subscription.Endpoints {
			outbounds = append(outbounds, singBoxHysteria2Outbound{
				Type: "hysteria2", Tag: endpoint.NodeName, Server: endpoint.PublicHost,
				ServerPort: endpoint.PublicPort, Password: endpoint.Credential,
				TLS: singBoxTLSData{
					Enabled: true, ServerName: endpoint.SNI, Insecure: endpoint.TLSInsecure,
					CertificatePublicKeySHA256: optionalStringSlice(endpoint.TLSPublicKeySHA256),
				},
			})
		}
		body, err := json.MarshalIndent(singBoxSubscription{Outbounds: outbounds}, "", "  ")
		if err != nil {
			return renderedSubscription{}, fmt.Errorf("encode sing-box subscription: %w", err)
		}
		body = append(body, '\n')
		return renderedSubscription{ContentType: "application/json; charset=utf-8", Body: body}, nil
	default:
		return renderedSubscription{}, store.ErrUnsupported
	}
}

func subscriptionURIs(subscription store.Subscription) []string {
	result := make([]string, 0, len(subscription.Endpoints))
	for _, endpoint := range subscription.Endpoints {
		result = append(result, hysteria2URI(endpoint))
	}
	return result
}

func hysteria2URI(endpoint store.SubscriptionEndpoint) string {
	address := net.JoinHostPort(endpoint.PublicHost, strconv.Itoa(endpoint.PublicPort))
	value := &url.URL{
		Scheme:   "hysteria2",
		User:     url.User(endpoint.Credential),
		Host:     address,
		Path:     "/",
		Fragment: endpoint.NodeName,
	}
	query := url.Values{}
	if endpoint.SNI != "" {
		query.Set("sni", endpoint.SNI)
	}
	if endpoint.TLSInsecure {
		query.Set("insecure", "1")
	}
	if endpoint.TLSCertFingerprint != "" {
		query.Set("pinSHA256", endpoint.TLSCertFingerprint)
	}
	value.RawQuery = query.Encode()
	return value.String()
}

func optionalStringSlice(value string) []string {
	if value == "" {
		return nil
	}
	return []string{value}
}
