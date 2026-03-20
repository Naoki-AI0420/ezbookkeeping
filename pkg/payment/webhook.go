package payment

import (
	"encoding/json"
	"io"
	"time"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/webhook"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/services"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
)

// HandleWebhook verifies and processes a Stripe webhook event
func HandleWebhook(c core.Context, config *settings.Config, body io.Reader, signature string) *errs.Error {
	payload, err := io.ReadAll(body)
	if err != nil {
		log.Errorf(c, "[payment.HandleWebhook] failed to read body, because %s", err.Error())
		return errs.ErrStripeWebhookVerifyFailed
	}

	event, err := webhook.ConstructEvent(payload, signature, config.StripeWebhookSecret)
	if err != nil {
		log.Errorf(c, "[payment.HandleWebhook] signature verification failed, because %s", err.Error())
		return errs.ErrStripeWebhookVerifyFailed
	}

	switch event.Type {
	case "invoice.paid":
		return handleInvoicePaid(c, config, &event)
	case "customer.subscription.updated":
		return handleSubscriptionUpdated(c, config, &event)
	case "customer.subscription.deleted":
		return handleSubscriptionDeleted(c, &event)
	default:
		log.Infof(c, "[payment.HandleWebhook] unhandled event type: %s", event.Type)
	}

	return nil
}

func handleInvoicePaid(c core.Context, config *settings.Config, event *stripe.Event) *errs.Error {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		log.Errorf(c, "[payment.handleInvoicePaid] failed to parse invoice, because %s", err.Error())
		return errs.ErrOperationFailed
	}

	customerID := invoice.Customer.ID
	sub, err := services.Subscriptions.GetSubscriptionByStripeCustomerId(c, customerID)
	if err != nil {
		log.Warnf(c, "[payment.handleInvoicePaid] subscription not found for customer %s", customerID)
		return nil
	}

	if invoice.Subscription != nil {
		sub.StripeSubscriptionId = invoice.Subscription.ID
	}

	if len(invoice.Lines.Data) > 0 && invoice.Lines.Data[0].Price != nil {
		priceID := invoice.Lines.Data[0].Price.ID
		sub.StripePriceId = priceID

		plan, cycle, ok := PriceIDToPlan(config, priceID)
		if ok {
			sub.SubscriptionPlan = plan
			sub.BillingCycle = cycle
		}
	}

	sub.SubscriptionStatus = models.SUBSCRIPTION_STATUS_ACTIVE
	sub.DatePaid = time.Now().Unix()

	if err := services.Subscriptions.UpdateSubscriptionFromWebhook(c, sub); err != nil {
		log.Errorf(c, "[payment.handleInvoicePaid] failed to update subscription, because %s", err.Error())
		return errs.ErrOperationFailed
	}

	return nil
}

func handleSubscriptionUpdated(c core.Context, config *settings.Config, event *stripe.Event) *errs.Error {
	var stripeSub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &stripeSub); err != nil {
		log.Errorf(c, "[payment.handleSubscriptionUpdated] failed to parse subscription, because %s", err.Error())
		return errs.ErrOperationFailed
	}

	sub, err := services.Subscriptions.GetSubscriptionByStripeCustomerId(c, stripeSub.Customer.ID)
	if err != nil {
		log.Warnf(c, "[payment.handleSubscriptionUpdated] subscription not found for customer %s", stripeSub.Customer.ID)
		return nil
	}

	sub.StripeSubscriptionId = stripeSub.ID
	sub.CurrentPeriodEnd = stripeSub.CurrentPeriodEnd

	switch stripeSub.Status {
	case stripe.SubscriptionStatusActive:
		sub.SubscriptionStatus = models.SUBSCRIPTION_STATUS_ACTIVE
	case stripe.SubscriptionStatusPastDue:
		sub.SubscriptionStatus = models.SUBSCRIPTION_STATUS_PAST_DUE
	case stripe.SubscriptionStatusCanceled:
		sub.SubscriptionStatus = models.SUBSCRIPTION_STATUS_CANCELED
	}

	if len(stripeSub.Items.Data) > 0 && stripeSub.Items.Data[0].Price != nil {
		priceID := stripeSub.Items.Data[0].Price.ID
		sub.StripePriceId = priceID

		plan, cycle, ok := PriceIDToPlan(config, priceID)
		if ok {
			sub.SubscriptionPlan = plan
			sub.BillingCycle = cycle
		}
	}

	if err := services.Subscriptions.UpdateSubscriptionFromWebhook(c, sub); err != nil {
		log.Errorf(c, "[payment.handleSubscriptionUpdated] failed to update subscription, because %s", err.Error())
		return errs.ErrOperationFailed
	}

	return nil
}

func handleSubscriptionDeleted(c core.Context, event *stripe.Event) *errs.Error {
	var stripeSub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &stripeSub); err != nil {
		log.Errorf(c, "[payment.handleSubscriptionDeleted] failed to parse subscription, because %s", err.Error())
		return errs.ErrOperationFailed
	}

	sub, err := services.Subscriptions.GetSubscriptionByStripeCustomerId(c, stripeSub.Customer.ID)
	if err != nil {
		log.Warnf(c, "[payment.handleSubscriptionDeleted] subscription not found for customer %s", stripeSub.Customer.ID)
		return nil
	}

	sub.SubscriptionPlan = models.PLAN_FREE
	sub.SubscriptionStatus = models.SUBSCRIPTION_STATUS_DELETED
	sub.StripePriceId = ""
	sub.StripeSubscriptionId = ""
	sub.BillingCycle = ""
	sub.CurrentPeriodEnd = 0

	if err := services.Subscriptions.UpdateSubscriptionFromWebhook(c, sub); err != nil {
		log.Errorf(c, "[payment.handleSubscriptionDeleted] failed to update subscription, because %s", err.Error())
		return errs.ErrOperationFailed
	}

	return nil
}
