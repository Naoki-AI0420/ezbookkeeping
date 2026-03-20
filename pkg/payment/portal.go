package payment

import (
	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/billingportal/session"

	"github.com/mayswind/ezbookkeeping/pkg/settings"
)

// CreatePortalSession creates a Stripe Customer Portal session
func CreatePortalSession(config *settings.Config, customerID string) (*stripe.BillingPortalSession, error) {
	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(customerID),
		ReturnURL: stripe.String(config.RootUrl),
	}

	return session.New(params)
}
