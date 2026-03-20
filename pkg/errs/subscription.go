package errs

import "net/http"

// NormalSubcategorySubscription is the subcategory for subscription errors
const NormalSubcategorySubscription = 20

// Subscription error codes
var (
	ErrSubscriptionNotFound          = NewNormalError(NormalSubcategorySubscription, 0, http.StatusBadRequest, "subscription not found")
	ErrStripeNotEnabled              = NewNormalError(NormalSubcategorySubscription, 1, http.StatusBadRequest, "stripe payment is not enabled")
	ErrInvalidPriceId                = NewNormalError(NormalSubcategorySubscription, 2, http.StatusBadRequest, "invalid price id")
	ErrStripeCheckoutFailed          = NewNormalError(NormalSubcategorySubscription, 3, http.StatusInternalServerError, "failed to create checkout session")
	ErrStripeWebhookVerifyFailed     = NewNormalError(NormalSubcategorySubscription, 4, http.StatusBadRequest, "webhook signature verification failed")
	ErrStripePortalFailed            = NewNormalError(NormalSubcategorySubscription, 5, http.StatusInternalServerError, "failed to create customer portal session")
	ErrSubscriptionLimitReached      = NewNormalError(NormalSubcategorySubscription, 6, http.StatusPaymentRequired, "subscription plan limit reached")
)
