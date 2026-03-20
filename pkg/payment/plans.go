package payment

import (
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
)

// PriceIDToPlan maps a Stripe price ID to a subscription plan and billing cycle
func PriceIDToPlan(config *settings.Config, priceID string) (models.SubscriptionPlanType, models.BillingCycleType, bool) {
	switch priceID {
	case config.StripeProMonthlyPriceID:
		return models.PLAN_PRO, models.BILLING_CYCLE_MONTHLY, true
	case config.StripeProYearlyPriceID:
		return models.PLAN_PRO, models.BILLING_CYCLE_YEARLY, true
	case config.StripeBizMonthlyPriceID:
		return models.PLAN_BUSINESS, models.BILLING_CYCLE_MONTHLY, true
	case config.StripeBizYearlyPriceID:
		return models.PLAN_BUSINESS, models.BILLING_CYCLE_YEARLY, true
	default:
		return "", "", false
	}
}

// IsValidPriceID checks if a price ID is configured
func IsValidPriceID(config *settings.Config, priceID string) bool {
	_, _, ok := PriceIDToPlan(config, priceID)
	return ok
}
