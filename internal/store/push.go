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
	// Replace all subscriptions for this user — one active browser at a time.
	// Stale endpoints accumulate and waste push delivery attempts.
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	tx.Exec("DELETE FROM push_subscriptions WHERE user_id=?", userID)
	_, err = tx.Exec("INSERT INTO push_subscriptions (user_id, endpoint, p256dh, auth) VALUES (?, ?, ?, ?)",
		userID, endpoint, p256dh, auth)
	if err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
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
