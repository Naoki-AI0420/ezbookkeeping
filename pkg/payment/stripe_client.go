package payment

import (
	"github.com/stripe/stripe-go/v82"

	"github.com/mayswind/ezbookkeeping/pkg/settings"
)

// InitStripe initializes the Stripe client with the API key from config
func InitStripe(config *settings.Config) {
	if config.EnableStripe && config.StripeAPIKey != "" {
		stripe.Key = config.StripeAPIKey
	}
}
