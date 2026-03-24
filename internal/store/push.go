// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package store

import "strings"

// PushSubscription represents a Web Push subscription.
type PushSubscription struct {
	ID       int64  `json:"id"`
	UserID   int64  `json:"user_id"`
	Endpoint string `json:"endpoint"`
	P256dh   string `json:"p256dh"`
	Auth     string `json:"auth"`
}

func (s *Store) SavePushSubscription(userID int64, endpoint, p256dh, auth string) error {
	// Upsert per-provider: one sub per browser type (chrome/firefox) per user.
	// Detect provider from endpoint URL.
	provider := "other"
	if strings.Contains(endpoint, "fcm.googleapis.com") {
		provider = "fcm"
	} else if strings.Contains(endpoint, "mozilla.com") || strings.Contains(endpoint, "push.services.mozilla") {
		provider = "mozilla"
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	// Delete old subs from same provider for this user
	tx.Exec("DELETE FROM push_subscriptions WHERE user_id=? AND endpoint LIKE ?",
		userID, providerPattern(provider))
	_, err = tx.Exec("INSERT INTO push_subscriptions (user_id, endpoint, p256dh, auth) VALUES (?, ?, ?, ?)",
		userID, endpoint, p256dh, auth)
	if err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func providerPattern(provider string) string {
	switch provider {
	case "fcm":
		return "%fcm.googleapis.com%"
	case "mozilla":
		return "%push.services.mozilla%"
	default:
		return "%"
	}
}

func (s *Store) UserPushSubscriptions(userID int64) ([]PushSubscription, error) {
	rows, err := s.db.Query("SELECT id, user_id, endpoint, p256dh, auth FROM push_subscriptions WHERE user_id=?", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var subs []PushSubscription
	for rows.Next() {
		var sub PushSubscription
		if err := rows.Scan(&sub.ID, &sub.UserID, &sub.Endpoint, &sub.P256dh, &sub.Auth); err != nil {
			continue
		}
		subs = append(subs, sub)
	}
	return subs, nil
}

func (s *Store) DeletePushSubscription(endpoint string) error {
	_, err := s.db.Exec("DELETE FROM push_subscriptions WHERE endpoint=?", endpoint)
	return err
}
