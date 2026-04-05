// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package store

// PushSubscription represents a Web Push subscription.
type PushSubscription struct {
	ID       int64  `json:"id"`
	UserID   int64  `json:"user_id"`
	Endpoint string `json:"endpoint"`
	P256dh   string `json:"p256dh"`
	Auth     string `json:"auth"`
}

func (s *Store) SavePushSubscription(userID int64, endpoint, p256dh, auth string) error {
	// Upsert by endpoint — each device/browser has a unique endpoint.
	// Max 5 subscriptions per user; remove oldest if exceeded.
	_, err := s.db.Exec(`INSERT INTO push_subscriptions (user_id, endpoint, p256dh, auth) VALUES (?, ?, ?, ?)
		ON CONFLICT(endpoint) DO UPDATE SET user_id=?, p256dh=?, auth=?`,
		userID, endpoint, p256dh, auth, userID, p256dh, auth)
	if err != nil {
		return err
	}
	// Trim to max 5 per user
	_, _ = s.db.Exec(`DELETE FROM push_subscriptions WHERE user_id=? AND id NOT IN
		(SELECT id FROM push_subscriptions WHERE user_id=? ORDER BY id DESC LIMIT 5)`, userID, userID)
	return nil
}

func (s *Store) UserPushSubscriptions(userID int64) ([]PushSubscription, error) {
	rows, err := s.db.Query("SELECT id, user_id, endpoint, p256dh, auth FROM push_subscriptions WHERE user_id=?", userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
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

func (s *Store) DeleteAllPushSubscriptions(userID int64) error {
	_, err := s.db.Exec("DELETE FROM push_subscriptions WHERE user_id=?", userID)
	return err
}
