// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package notify

import (
	"encoding/json"
	"log/slog"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/pusk-platform/pusk/internal/store"
)

// PushService sends Web Push notifications.
// It no longer holds a store reference; callers pass the org store per-request.
type PushService struct {
	vapidPub   string
	vapidPriv  string
	vapidEmail string
}

func NewPushService(vapidPub, vapidPriv, email string) *PushService {
	return &PushService{
		vapidPub: vapidPub, vapidPriv: vapidPriv, vapidEmail: email,
	}
}

type PushPayload struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Icon  string `json:"icon,omitempty"`
	Tag   string `json:"tag,omitempty"`
	URL   string `json:"url,omitempty"`
}

// SendToUser sends push notifications using the provided org store
// to look up subscriptions (not a hardcoded default store).
func (p *PushService) SendToUser(s *store.Store, userID int64, payload PushPayload) {
	if p.vapidPub == "" {
		return // push not configured
	}

	subs, err := s.UserPushSubscriptions(userID)
	if err != nil || len(subs) == 0 {
		return
	}

	data, _ := json.Marshal(payload)

	for _, sub := range subs {
		wps := &webpush.Subscription{
			Endpoint: sub.Endpoint,
			Keys: webpush.Keys{
				P256dh: sub.P256dh,
				Auth:   sub.Auth,
			},
		}

		resp, err := webpush.SendNotification(data, wps, &webpush.Options{
			Subscriber:      p.vapidEmail,
			VAPIDPublicKey:  p.vapidPub,
			VAPIDPrivateKey: p.vapidPriv,
			TTL:             3600,
		})
		if err != nil {
			slog.Error("push send failed", "user_id", userID, "error", err)
			if resp != nil && (resp.StatusCode == 410 || resp.StatusCode == 404) {
				s.DeletePushSubscription(sub.Endpoint)
				slog.Info("push stale subscription removed", "user_id", userID)
			}
			continue
		}
		resp.Body.Close()
	}
}
