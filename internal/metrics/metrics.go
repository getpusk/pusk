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
	OrgsTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "pusk_orgs_total",
			Help: "Total registered organizations",
		},
	)
	UsersTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "pusk_users_total",
			Help: "Total registered users across all orgs",
		},
	)
	// Delivery-outcome metrics: without these the platform's primary function
	// (did the alert actually reach a recipient?) is unobservable.
	PushTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pusk_push_total",
			Help: "Web Push notifications by result",
		},
		[]string{"result"}, // sent | failed | stale
	)
	WebhookForward = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pusk_webhook_forward_total",
			Help: "Outbound webhook deliveries by result",
		},
		[]string{"result"}, // success | failure
	)
	UpdateQueueDropped = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "pusk_update_queue_dropped_total",
			Help: "Bot update-queue messages dropped because the queue was full",
		},
	)
	RelayBotsConnected = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "pusk_relay_bots_connected",
			Help: "Bots currently connected via WebSocket relay",
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
		OrgsTotal,
		UsersTotal,
		PushTotal,
		WebhookForward,
		UpdateQueueDropped,
		RelayBotsConnected,
	)
}
