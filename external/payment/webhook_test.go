package payment

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"donetick.com/core/config"
	pModel "donetick.com/core/external/payment/model"
	pDB "donetick.com/core/external/payment/repo"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/webhook"
	"gorm.io/gorm"
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

// --- StripeWebhook signature verification ---
//
// These tests exercise the full HTTP handler (not just isIPWhitelisted), so
// the request must originate from a whitelisted IP to reach the signature
// check, and the fake event payload's api_version must match stripe.APIVersion
// or webhook.ConstructEvent rejects it as a version mismatch before signature
// verification even matters.

const (
	testWhitelistedIP   = "1.2.3.4"
	testStripeSecret    = "whsec_test_secret"
	testStripeSignature = "Stripe-Signature"
)

// newTestWebhookWithSecret builds a Webhook backed by an in-memory sqlite DB
// (so handler paths that touch the DB - e.g. invoice.payment_succeeded - work
// end to end) configured with the given whitelisted IPs and webhook secret.
func newTestWebhookWithSecret(t *testing.T, whitelistedIPs []string, secret string) (*Webhook, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&pModel.StripeCustomer{},
		&pModel.StripeSession{},
		&pModel.StripeSubscription{},
		&pModel.StripeInvoice{},
		&pModel.Subscription{},
	); err != nil {
		t.Fatalf("failed to migrate schema: %v", err)
	}

	w := NewWebhook(
		pDB.NewStripeDB(db),
		pDB.RevenueCatDB{},
		pDB.NewSubscriptionDB(db),
		nil,
		nil,
		&config.Config{
			StripeConfig: config.StripeConfig{
				WhitelistedIPs: whitelistedIPs,
				WebhookSecret:  secret,
			},
		},
	)
	return w, db
}

// stripeEventJSON builds a minimal Stripe event envelope carrying dataObject
// as event.Data.Raw, with api_version set to stripe.APIVersion so
// webhook.ConstructEvent doesn't reject it as an API version mismatch.
func stripeEventJSON(t *testing.T, id, eventType string, dataObject interface{}) []byte {
	t.Helper()

	rawData, err := json.Marshal(dataObject)
	if err != nil {
		t.Fatalf("failed to marshal event data object: %v", err)
	}

	envelope := map[string]interface{}{
		"id":          id,
		"object":      "event",
		"api_version": stripe.APIVersion,
		"type":        eventType,
		"data": map[string]interface{}{
			"object": json.RawMessage(rawData),
		},
	}
	b, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("failed to marshal event envelope: %v", err)
	}
	return b
}

// signedStripeRequest builds a *gin.Context/httptest.ResponseRecorder pair
// for a POST to the Stripe webhook endpoint, coming from remoteIP, with the
// given body and Stripe-Signature header value (skipped entirely if empty).
func signedStripeRequest(payload []byte, sigHeader, remoteIP string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/stripe", bytes.NewReader(payload))
	req.RemoteAddr = remoteIP + ":12345"
	if sigHeader != "" {
		req.Header.Set(testStripeSignature, sigHeader)
	}
	c.Request = req

	return c, rec
}

func TestStripeWebhook_ValidSignature_Accepted(t *testing.T) {
	w, _ := newTestWebhookWithSecret(t, []string{testWhitelistedIP}, testStripeSecret)

	// An event type the handler doesn't otherwise act on, so this test is
	// purely about the signature-verification gate, not downstream processing.
	payload := stripeEventJSON(t, "evt_valid_1", "customer.created", map[string]interface{}{"id": "cus_test"})
	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload: payload,
		Secret:  testStripeSecret,
	})

	c, rec := signedStripeRequest(payload, signed.Header, testWhitelistedIP)
	w.StripeWebhook(c)

	if rec.Code != http.StatusOK {
		t.Errorf("StripeWebhook() with a validly signed payload = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestStripeWebhook_TamperedSignature_Rejected(t *testing.T) {
	w, _ := newTestWebhookWithSecret(t, []string{testWhitelistedIP}, testStripeSecret)

	original := stripeEventJSON(t, "evt_tampered_1", "customer.created", map[string]interface{}{"id": "cus_test"})
	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload: original,
		Secret:  testStripeSecret,
	})

	// Send a different body than the one that was signed - the signature no
	// longer matches the payload it's paired with.
	tampered := stripeEventJSON(t, "evt_tampered_1_evil", "customer.created", map[string]interface{}{"id": "cus_attacker"})

	c, rec := signedStripeRequest(tampered, signed.Header, testWhitelistedIP)
	w.StripeWebhook(c)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("StripeWebhook() with a tampered payload = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestStripeWebhook_MissingSignatureHeader_Rejected(t *testing.T) {
	w, _ := newTestWebhookWithSecret(t, []string{testWhitelistedIP}, testStripeSecret)

	payload := stripeEventJSON(t, "evt_no_header_1", "customer.created", map[string]interface{}{"id": "cus_test"})

	c, rec := signedStripeRequest(payload, "", testWhitelistedIP)
	w.StripeWebhook(c)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("StripeWebhook() with no Stripe-Signature header = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// TestStripeWebhook_EmptySecret_FailsClosed documents the deliberate choice
// to fail closed when webhook_secret is unconfigured: the handler rejects
// the request before even looking at the signature, rather than silently
// skipping verification. This matters because the endpoint isn't wired up
// today (see Webhooks() in webhook.go) - whoever registers the routes later
// must configure the secret, or the endpoint refuses to serve at all instead
// of quietly accepting unverified payment events.
func TestStripeWebhook_EmptySecret_FailsClosed(t *testing.T) {
	w, _ := newTestWebhookWithSecret(t, []string{testWhitelistedIP}, "")

	// Deliberately do not sign this payload at all - an unconfigured secret
	// must reject the request regardless of what (if anything) is in the
	// Stripe-Signature header.
	payload := stripeEventJSON(t, "evt_no_secret_1", "customer.created", map[string]interface{}{"id": "cus_test"})

	c, rec := signedStripeRequest(payload, "", testWhitelistedIP)
	w.StripeWebhook(c)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("StripeWebhook() with an unconfigured webhook secret = %d, want %d (fail closed)", rec.Code, http.StatusInternalServerError)
	}
}

func TestStripeWebhook_InvoicePaymentSucceeded_PopulatesAmount(t *testing.T) {
	w, db := newTestWebhookWithSecret(t, []string{testWhitelistedIP}, testStripeSecret)

	const (
		invoiceID      = "in_test_123"
		customerID     = "cus_test_123"
		subscriptionID = "sub_test_123"
		amountPaid     = 2599 // $25.99, in cents - Stripe amounts are already in the smallest currency unit
	)
	periodStart := time.Now().UTC().Add(-30 * 24 * time.Hour).Unix()
	periodEnd := time.Now().UTC().Unix()

	invoiceData := map[string]interface{}{
		"id":           invoiceID,
		"customer":     map[string]interface{}{"id": customerID},
		"subscription": map[string]interface{}{"id": subscriptionID},
		"amount_paid":  amountPaid,
		"period_start": periodStart,
		"period_end":   periodEnd,
	}
	payload := stripeEventJSON(t, "evt_invoice_1", "invoice.payment_succeeded", invoiceData)
	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload: payload,
		Secret:  testStripeSecret,
	})

	c, rec := signedStripeRequest(payload, signed.Header, testWhitelistedIP)
	w.StripeWebhook(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("StripeWebhook() for invoice.payment_succeeded = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var saved pModel.StripeInvoice
	if err := db.Where("invoice_id = ?", invoiceID).First(&saved).Error; err != nil {
		t.Fatalf("failed to load saved invoice: %v", err)
	}
	if saved.Amount != amountPaid {
		t.Errorf("saved invoice Amount = %d, want %d", saved.Amount, amountPaid)
	}
}
