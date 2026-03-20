package services

import (
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// SubscriptionService represents subscription service
type SubscriptionService struct {
	ServiceUsingDB
}

// Initialize a subscription service singleton instance
var (
	Subscriptions = &SubscriptionService{
		ServiceUsingDB: ServiceUsingDB{
			container: datastore.Container,
		},
	}
)

// GetSubscriptionByUid returns subscription for a user
func (s *SubscriptionService) GetSubscriptionByUid(c core.Context, uid int64) (*models.UserSubscription, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	sub := &models.UserSubscription{}
	has, err := s.UserDB().NewSession(c).Where("uid=?", uid).Get(sub)

	if err != nil {
		return nil, err
	}

	if !has {
		return nil, errs.ErrSubscriptionNotFound
	}

	return sub, nil
}

// GetSubscriptionByStripeCustomerId returns subscription by Stripe customer ID
func (s *SubscriptionService) GetSubscriptionByStripeCustomerId(c core.Context, customerId string) (*models.UserSubscription, error) {
	sub := &models.UserSubscription{}
	has, err := s.UserDB().NewSession(c).Where("stripe_customer_id=?", customerId).Get(sub)

	if err != nil {
		return nil, err
	}

	if !has {
		return nil, errs.ErrSubscriptionNotFound
	}

	return sub, nil
}

// GetSubscriptionByStripeSubscriptionId returns subscription by Stripe subscription ID
func (s *SubscriptionService) GetSubscriptionByStripeSubscriptionId(c core.Context, subscriptionId string) (*models.UserSubscription, error) {
	sub := &models.UserSubscription{}
	has, err := s.UserDB().NewSession(c).Where("stripe_subscription_id=?", subscriptionId).Get(sub)

	if err != nil {
		return nil, err
	}

	if !has {
		return nil, errs.ErrSubscriptionNotFound
	}

	return sub, nil
}

// CreateSubscription creates a new subscription record for a user
func (s *SubscriptionService) CreateSubscription(c core.Context, uid int64, stripeCustomerId string) (*models.UserSubscription, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	now := time.Now().Unix()
	sub := &models.UserSubscription{
		Uid:              uid,
		StripeCustomerId: stripeCustomerId,
		SubscriptionPlan: models.PLAN_FREE,
		SubscriptionStatus: models.SUBSCRIPTION_STATUS_ACTIVE,
		CreatedUnixTime: now,
		UpdatedUnixTime: now,
	}

	_, err := s.UserDB().NewSession(c).Insert(sub)

	if err != nil {
		return nil, err
	}

	return sub, nil
}

// UpdateSubscriptionFromWebhook updates subscription data from a Stripe webhook event
func (s *SubscriptionService) UpdateSubscriptionFromWebhook(c core.Context, sub *models.UserSubscription) error {
	sub.UpdatedUnixTime = time.Now().Unix()

	_, err := s.UserDB().NewSession(c).Where("uid=?", sub.Uid).Cols(
		"subscription_plan",
		"subscription_status",
		"stripe_price_id",
		"stripe_subscription_id",
		"billing_cycle",
		"current_period_end",
		"date_paid",
		"updated_unix_time",
	).Update(sub)

	return err
}

// GetOrCreateSubscription returns existing subscription or creates free one
func (s *SubscriptionService) GetOrCreateSubscription(c core.Context, uid int64) (*models.UserSubscription, error) {
	sub, err := s.GetSubscriptionByUid(c, uid)

	if err == errs.ErrSubscriptionNotFound {
		return s.CreateSubscription(c, uid, "")
	}

	return sub, err
}
