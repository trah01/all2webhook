package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

func postJSON(webhookURL string, jsonBody []byte) error {
	return postJSONWithTarget(WebhookTarget{}, webhookURL, jsonBody)
}

func postJSONWithTarget(target WebhookTarget, webhookURL string, jsonBody []byte) error {
	client, err := httpClientForTarget(target)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, webhookURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range parseWebhookHeaders(target.Headers) {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}
	return nil
}

func httpClientForTarget(target WebhookTarget) (*http.Client, error) {
	if target.TLSCACert == "" && target.TLSClientCert == "" && target.TLSClientKey == "" && !target.TLSSkipVerify {
		return httpClient, nil
	}

	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: target.TLSSkipVerify,
	}
	if target.TLSCACert != "" {
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		if ok := pool.AppendCertsFromPEM([]byte(target.TLSCACert)); !ok {
			return nil, fmt.Errorf("自定义 CA 证书不是有效 PEM")
		}
		tlsConfig.RootCAs = pool
	}
	if target.TLSClientCert != "" || target.TLSClientKey != "" {
		cert, err := tls.X509KeyPair([]byte(target.TLSClientCert), []byte(target.TLSClientKey))
		if err != nil {
			return nil, fmt.Errorf("客户端证书或私钥无效: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           (&net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			TLSClientConfig:       tlsConfig,
		},
	}, nil
}

func parseWebhookHeaders(raw string) map[string]string {
	headers := make(map[string]string)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return headers
	}
	var parsed map[string]string
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return headers
	}
	for key, value := range parsed {
		key = strings.TrimSpace(key)
		if key != "" {
			headers[key] = strings.TrimSpace(value)
		}
	}
	return headers
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			items = append(items, part)
		}
	}
	return items
}
