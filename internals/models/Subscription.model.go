package models

import "time"

type Subscription struct {
	ID               string             `json:"id"`
	PlanID           string             `json:"plan_id"`
	StartTime        time.Time          `json:"start_time"`
	Quantity         string             `json:"quantity"`
	ShippingAmount   CurrencyAmount     `json:"shipping_amount"`
	Subscriber       Subscriber         `json:"subscriber"`
	BillingInfo      BillingInfo        `json:"billing_info"`
	CreateTime       time.Time          `json:"create_time"`
	UpdateTime       time.Time          `json:"update_time"`
	Links            []SubscriptionLink `json:"links"`
	Status           string             `json:"status"`
	StatusUpdateTime time.Time          `json:"status_update_time"`
}

type CurrencyAmount struct {
	CurrencyCode string `json:"currency_code"`
	Value        string `json:"value"`
}

type Subscriber struct {
	ShippingAddress SubscriptionShippingAddress `json:"shipping_address"`
	Name            Name                        `json:"name"`
	EmailAddress    string                      `json:"email_address"`
	PayerID         string                      `json:"payer_id"`
}

type SubscriptionShippingAddress struct {
	Name    FullName `json:"name"`
	Address Address  `json:"address"`
}

type FullName struct {
	FullName string `json:"full_name"`
}

type Address struct {
	AddressLine1 string `json:"address_line_1"`
	AddressLine2 string `json:"address_line_2"`
	AdminArea2   string `json:"admin_area_2"`
	AdminArea1   string `json:"admin_area_1"`
	PostalCode   string `json:"postal_code"`
	CountryCode  string `json:"country_code"`
}

type Name struct {
	GivenName string `json:"given_name"`
	Surname   string `json:"surname"`
}

type BillingInfo struct {
	OutstandingBalance  CurrencyAmount   `json:"outstanding_balance"`
	CycleExecutions     []CycleExecution `json:"cycle_executions"`
	LastPayment         PaymentDetail    `json:"last_payment"`
	NextBillingTime     time.Time        `json:"next_billing_time"`
	FailedPaymentsCount int              `json:"failed_payments_count"`
}

type CycleExecution struct {
	TenureType      string `json:"tenure_type"`
	Sequence        int    `json:"sequence"`
	CyclesCompleted int    `json:"cycles_completed"`
	CyclesRemaining int    `json:"cycles_remaining"`
	TotalCycles     int    `json:"total_cycles"`
}

type PaymentDetail struct {
	Amount CurrencyAmount `json:"amount"`
	Time   time.Time      `json:"time"`
}

type SubscriptionLink struct {
	Href   string `json:"href"`
	Rel    string `json:"rel"`
	Method string `json:"method"`
}
