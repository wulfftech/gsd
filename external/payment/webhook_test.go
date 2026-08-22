package payment

import (
	"encoding/json"
	"testing"

	"donetick.com/core/config"
	pDB "donetick.com/core/external/payment/repo"
)

func newTestWebhook(t *testing.T, whitelistedIPs []string) *Webhook {
	t.Helper()
	return NewWebhook(
		pDB.StripeDB{},
		pDB.RevenueCatDB{},
		pDB.SubscriptionDB{},
		nil,
		nil,
		&config.Config{
			StripeConfig: config.StripeConfig{WhitelistedIPs: whitelistedIPs},
		},
	)
}

func TestWebhook_IsIPWhitelisted(t *testing.T) {
	w := newTestWebhook(t, []string{"1.2.3.4", "10.0.0.1"})

	tests := []struct {
		name string
		ip   string
		want bool
	}{
		{"exact match is whitelisted", "1.2.3.4", true},
		{"second configured ip is whitelisted", "10.0.0.1", true},
		{"unlisted ip is rejected", "8.8.8.8", false},
		{"empty ip is rejected", "", false},
		{"partial/prefix match is rejected (no CIDR support)", "1.2.3", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := w.isIPWhitelisted(tt.ip); got != tt.want {
				t.Errorf("isIPWhitelisted(%q) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

func TestWebhook_IsIPWhitelisted_EmptyConfigRejectsEverything(t *testing.T) {
	w := newTestWebhook(t, nil)

	if w.isIPWhitelisted("1.2.3.4") {
		t.Error("isIPWhitelisted() returned true with no configured whitelist")
	}
}

func TestWebhook_ExtractUserIDFromEvent(t *testing.T) {
	w := newTestWebhook(t, nil)

	tests := []struct {
		name  string
		event RevenueCatEvent
		want  int
	}{
		{
			name:  "numeric app_user_id is used directly",
			event: RevenueCatEvent{AppUserID: "42"},
			want:  42,
		},
		{
			name: "falls back to subscriber $userId when app_user_id is not numeric",
			event: RevenueCatEvent{
				AppUserID: "anonymous-install-id",
				Subscriber: RevenueCatSubscriberAttributes{
					UserID: struct {
						Value     string `json:"value"`
						UpdatedAt int64  `json:"updated_at_ms"`
					}{Value: "7"},
				},
			},
			want: 7,
		},
		{
			name:  "non-numeric app_user_id with no subscriber fallback yields zero",
			event: RevenueCatEvent{AppUserID: "anonymous-install-id"},
			want:  0,
		},
		{
			name:  "zero app_user_id is rejected, not treated as a valid user",
			event: RevenueCatEvent{AppUserID: "0"},
			want:  0,
		},
		{
			name:  "negative app_user_id is rejected",
			event: RevenueCatEvent{AppUserID: "-5"},
			want:  0,
		},
		{
			name:  "empty app_user_id with no subscriber data yields zero",
			event: RevenueCatEvent{AppUserID: ""},
			want:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := w.extractUserIDFromEvent(tt.event); got != tt.want {
				t.Errorf("extractUserIDFromEvent() = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestRevenueCatWebhookEvent_JSONParsing exercises decoding of a realistic
// RevenueCat webhook payload, including the nested $userId subscriber
// attribute and optional pointer fields, since this is the only
// non-network-bound "request parsing" surface in the RevenueCat webhook path.
func TestRevenueCatWebhookEvent_JSONParsing(t *testing.T) {
	raw := `{
		"api_version": "1.0",
		"event": {
			"id": "evt_123",
			"type": "INITIAL_PURCHASE",
			"event_timestamp_ms": 1700000000000,
			"app_user_id": "99",
			"original_app_user_id": "99",
			"product_id": "gsd_plus_monthly",
			"entitlement_ids": ["plus"],
			"store": "app_store",
			"purchased_at_ms": 1700000000000,
			"expiration_at_ms": 1702592000000,
			"price": 4.99,
			"currency": "USD",
			"environment": "PRODUCTION",
			"app_id": "app_1",
			"subscriber_attributes": {
				"$userId": {"value": "99", "updated_at_ms": 1699999999000}
			},
			"transactions": [
				{
					"id": "txn_1",
					"original_transaction_id": "orig_txn_1",
					"product_id": "gsd_plus_monthly",
					"purchase_date_ms": 1700000000000,
					"expires_date_ms": 1702592000000,
					"is_trial_period": false,
					"auto_renew_status": true,
					"period_type": "normal",
					"store": "app_store",
					"environment": "PRODUCTION"
				}
			]
		}
	}`

	var got RevenueCatWebhookEvent
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got.Event.ID != "evt_123" {
		t.Errorf("Event.ID = %q, want %q", got.Event.ID, "evt_123")
	}
	if got.Event.Type != "INITIAL_PURCHASE" {
		t.Errorf("Event.Type = %q, want %q", got.Event.Type, "INITIAL_PURCHASE")
	}
	if got.Event.Subscriber.UserID.Value != "99" {
		t.Errorf("Event.Subscriber.UserID.Value = %q, want %q", got.Event.Subscriber.UserID.Value, "99")
	}
	if got.Event.ExpirationAtMs == nil || *got.Event.ExpirationAtMs != 1702592000000 {
		t.Errorf("Event.ExpirationAtMs = %v, want pointer to 1702592000000", got.Event.ExpirationAtMs)
	}
	if got.Event.Price == nil || *got.Event.Price != 4.99 {
		t.Errorf("Event.Price = %v, want pointer to 4.99", got.Event.Price)
	}
	if len(got.Event.Transactions) != 1 {
		t.Fatalf("len(Event.Transactions) = %d, want 1", len(got.Event.Transactions))
	}
	txn := got.Event.Transactions[0]
	if txn.OriginalTransactionID != "orig_txn_1" {
		t.Errorf("Transactions[0].OriginalTransactionID = %q, want %q", txn.OriginalTransactionID, "orig_txn_1")
	}
	if txn.AutoRenewStatus == nil || !*txn.AutoRenewStatus {
		t.Errorf("Transactions[0].AutoRenewStatus = %v, want pointer to true", txn.AutoRenewStatus)
	}
}

func TestRevenueCatWebhookEvent_JSONParsing_OmitsOptionalPointerFields(t *testing.T) {
	raw := `{
		"api_version": "1.0",
		"event": {
			"id": "evt_456",
			"type": "NON_RENEWING_PURCHASE",
			"app_user_id": "5",
			"product_id": "gsd_plus_lifetime"
		}
	}`

	var got RevenueCatWebhookEvent
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got.Event.ExpirationAtMs != nil {
		t.Errorf("Event.ExpirationAtMs = %v, want nil when absent from payload", got.Event.ExpirationAtMs)
	}
	if got.Event.Price != nil {
		t.Errorf("Event.Price = %v, want nil when absent from payload", got.Event.Price)
	}
	if got.Event.Transactions != nil {
		t.Errorf("Event.Transactions = %v, want nil when absent from payload", got.Event.Transactions)
	}
}
