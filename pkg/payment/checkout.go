package payment

import (
	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/customer"

	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

// EnsureStripeCustomer finds or creates a Stripe customer for the user
func EnsureStripeCustomer(sub *models.UserSubscription, email string) (string, error) {
	if sub.StripeCustomerId != "" {
		return sub.StripeCustomerId, nil
	}

	params := &stripe.CustomerParams{
		Email: stripe.String(email),
		Metadata: map[string]string{
			"uid": utils.Int64ToString(sub.Uid),
		},
	}

	c, err := customer.New(params)
	if err != nil {
		return "", err
	}

	return c.ID, nil
}

// CreateCheckoutSession creates a Stripe Checkout Session for the given price
func CreateCheckoutSession(config *settings.Config, customerID string, priceID string) (*stripe.CheckoutSession, error) {
	params := &stripe.CheckoutSessionParams{
		Customer: stripe.String(customerID),
		Mode:     stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(priceID),
				Quantity: stripe.Int64(1),
			},
		},
		SuccessURL:          stripe.String(config.StripeCheckoutSuccessURL),
		CancelURL:           stripe.String(config.StripeCheckoutCancelURL),
		AllowPromotionCodes: stripe.Bool(true),
	}

	return session.New(params)
}
