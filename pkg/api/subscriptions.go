package api

import (
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/payment"
	"github.com/mayswind/ezbookkeeping/pkg/services"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
)

// SubscriptionsApi represents subscription api
type SubscriptionsApi struct {
	ApiUsingConfig
	users         *services.UserService
	subscriptions *services.SubscriptionService
}

// Initialize a subscription api singleton instance
var (
	Subscriptions = &SubscriptionsApi{
		ApiUsingConfig: ApiUsingConfig{
			container: settings.Container,
		},
		users:         services.Users,
		subscriptions: services.Subscriptions,
	}
)

// SubscriptionCheckoutHandler creates a Stripe Checkout session
func (a *SubscriptionsApi) SubscriptionCheckoutHandler(c *core.WebContext) (any, *errs.Error) {
	config := a.CurrentConfig()

	if !config.EnableStripe {
		return nil, errs.ErrStripeNotEnabled
	}

	var req models.SubscriptionCheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnf(c, "[subscriptions.SubscriptionCheckoutHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	if !payment.IsValidPriceID(config, req.PriceID) {
		return nil, errs.ErrInvalidPriceId
	}

	uid := c.GetCurrentUid()

	user, err := a.users.GetUserById(c, uid)
	if err != nil {
		log.Errorf(c, "[subscriptions.SubscriptionCheckoutHandler] failed to get user, because %s", err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	sub, err := a.subscriptions.GetOrCreateSubscription(c, uid)
	if err != nil {
		log.Errorf(c, "[subscriptions.SubscriptionCheckoutHandler] failed to get subscription, because %s", err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	customerID, err := payment.EnsureStripeCustomer(sub, user.Email)
	if err != nil {
		log.Errorf(c, "[subscriptions.SubscriptionCheckoutHandler] failed to ensure stripe customer, because %s", err.Error())
		return nil, errs.ErrStripeCheckoutFailed
	}

	if sub.StripeCustomerId == "" {
		sub.StripeCustomerId = customerID
		if updateErr := a.subscriptions.UpdateSubscriptionFromWebhook(c, sub); updateErr != nil {
			log.Warnf(c, "[subscriptions.SubscriptionCheckoutHandler] failed to save customer id, because %s", updateErr.Error())
		}
	}

	session, err := payment.CreateCheckoutSession(config, customerID, req.PriceID)
	if err != nil {
		log.Errorf(c, "[subscriptions.SubscriptionCheckoutHandler] failed to create checkout session, because %s", err.Error())
		return nil, errs.ErrStripeCheckoutFailed
	}

	return &models.SubscriptionCheckoutResponse{
		CheckoutURL: session.URL,
	}, nil
}

// SubscriptionStatusHandler returns the current subscription status
func (a *SubscriptionsApi) SubscriptionStatusHandler(c *core.WebContext) (any, *errs.Error) {
	uid := c.GetCurrentUid()

	sub, err := a.subscriptions.GetOrCreateSubscription(c, uid)
	if err != nil {
		log.Errorf(c, "[subscriptions.SubscriptionStatusHandler] failed to get subscription, because %s", err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return &models.SubscriptionStatusResponse{
		Plan:             sub.SubscriptionPlan,
		Status:           sub.SubscriptionStatus,
		BillingCycle:     sub.BillingCycle,
		CurrentPeriodEnd: sub.CurrentPeriodEnd,
	}, nil
}

// SubscriptionPortalHandler creates a Stripe Customer Portal session
func (a *SubscriptionsApi) SubscriptionPortalHandler(c *core.WebContext) (any, *errs.Error) {
	config := a.CurrentConfig()

	if !config.EnableStripe {
		return nil, errs.ErrStripeNotEnabled
	}

	uid := c.GetCurrentUid()

	sub, err := a.subscriptions.GetSubscriptionByUid(c, uid)
	if err != nil {
		log.Errorf(c, "[subscriptions.SubscriptionPortalHandler] failed to get subscription, because %s", err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	if sub.StripeCustomerId == "" {
		return nil, errs.ErrSubscriptionNotFound
	}

	portalSession, err := payment.CreatePortalSession(config, sub.StripeCustomerId)
	if err != nil {
		log.Errorf(c, "[subscriptions.SubscriptionPortalHandler] failed to create portal session, because %s", err.Error())
		return nil, errs.ErrStripePortalFailed
	}

	return &models.SubscriptionPortalResponse{
		PortalURL: portalSession.URL,
	}, nil
}

// SubscriptionWebhookHandler handles Stripe webhook events
func (a *SubscriptionsApi) SubscriptionWebhookHandler(c *core.WebContext) (any, *errs.Error) {
	config := a.CurrentConfig()

	if !config.EnableStripe {
		return nil, errs.ErrStripeNotEnabled
	}

	signature := c.GetHeader("Stripe-Signature")
	if signature == "" {
		return nil, errs.ErrStripeWebhookVerifyFailed
	}

	webhookErr := payment.HandleWebhook(c, config, c.Request.Body, signature)
	if webhookErr != nil {
		return nil, webhookErr
	}

	return map[string]bool{"received": true}, nil
}
