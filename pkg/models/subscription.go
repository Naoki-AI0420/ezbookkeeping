package models

// SubscriptionPlanType represents subscription plan type
type SubscriptionPlanType string

// Subscription plan types
const (
	PLAN_FREE     SubscriptionPlanType = "free"
	PLAN_PRO      SubscriptionPlanType = "pro"
	PLAN_BUSINESS SubscriptionPlanType = "business"
)

// SubscriptionStatusType represents subscription status type
type SubscriptionStatusType string

// Subscription status types
const (
	SUBSCRIPTION_STATUS_ACTIVE    SubscriptionStatusType = "active"
	SUBSCRIPTION_STATUS_PAST_DUE  SubscriptionStatusType = "past_due"
	SUBSCRIPTION_STATUS_CANCELED  SubscriptionStatusType = "canceled"
	SUBSCRIPTION_STATUS_DELETED   SubscriptionStatusType = "deleted"
)

// BillingCycleType represents billing cycle type
type BillingCycleType string

// Billing cycle types
const (
	BILLING_CYCLE_MONTHLY BillingCycleType = "monthly"
	BILLING_CYCLE_YEARLY  BillingCycleType = "yearly"
)

// UserSubscription represents user subscription model
type UserSubscription struct {
	Uid                  int64                  `xorm:"PK" json:"uid"`
	StripeCustomerId     string                 `xorm:"VARCHAR(255) INDEX" json:"stripeCustomerId"`
	SubscriptionPlan     SubscriptionPlanType   `xorm:"VARCHAR(50) NOT NULL DEFAULT 'free'" json:"subscriptionPlan"`
	SubscriptionStatus   SubscriptionStatusType `xorm:"VARCHAR(50) NOT NULL DEFAULT 'active'" json:"subscriptionStatus"`
	StripePriceId        string                 `xorm:"VARCHAR(255)" json:"stripePriceId"`
	StripeSubscriptionId string                 `xorm:"VARCHAR(255) INDEX" json:"stripeSubscriptionId"`
	BillingCycle         BillingCycleType       `xorm:"VARCHAR(20)" json:"billingCycle"`
	CurrentPeriodEnd     int64                  `xorm:"BIGINT" json:"currentPeriodEnd"`
	DatePaid             int64                  `xorm:"BIGINT" json:"datePaid"`
	CreatedUnixTime      int64                  `xorm:"BIGINT NOT NULL" json:"createdUnixTime"`
	UpdatedUnixTime      int64                  `xorm:"BIGINT NOT NULL" json:"updatedUnixTime"`
}

// SubscriptionCheckoutRequest represents checkout session creation request
type SubscriptionCheckoutRequest struct {
	PriceID string `json:"priceId" binding:"required"`
}

// SubscriptionCheckoutResponse represents checkout session response
type SubscriptionCheckoutResponse struct {
	CheckoutURL string `json:"checkoutUrl"`
}

// SubscriptionStatusResponse represents subscription status response
type SubscriptionStatusResponse struct {
	Plan             SubscriptionPlanType   `json:"plan"`
	Status           SubscriptionStatusType `json:"status"`
	BillingCycle     BillingCycleType       `json:"billingCycle"`
	CurrentPeriodEnd int64                  `json:"currentPeriodEnd"`
}

// SubscriptionPortalResponse represents customer portal URL response
type SubscriptionPortalResponse struct {
	PortalURL string `json:"portalUrl"`
}
