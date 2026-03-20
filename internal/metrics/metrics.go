// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	HTTPRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pusk_http_requests_total",
			Help: "Total HTTP requests",
		},
		[]string{"method", "path", "status"},
	)
	HTTPDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "pusk_http_request_duration_seconds",
			Help:    "Request duration",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
	WSConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "pusk_ws_connections",
			Help: "Active WebSocket connections",
		},
	)
	MessagesSent = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pusk_messages_total",
			Help: "Messages sent",
		},
		[]string{"type"},
	)
	WebhooksReceived = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pusk_webhooks_total",
			Help: "Webhooks received",
		},
		[]string{"format"},
	)
	WebhooksDedupedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "pusk_webhooks_deduped_total",
			Help: "Webhooks deduplicated by debounce",
		},
	)
)

func init() {
	prometheus.MustRegister(
		HTTPRequests,
		HTTPDuration,
		WSConnections,
		MessagesSent,
		WebhooksReceived,
		WebhooksDedupedTotal,
	)
}
