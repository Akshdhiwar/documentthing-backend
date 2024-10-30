package models

type WebhookEvent struct {
	ID           string   `json:"id"`
	CreateTime   string   `json:"create_time"`
	ResourceType string   `json:"resource_type"`
	EventType    string   `json:"event_type"`
	Summary      string   `json:"summary"`
	Resource     Resource `json:"resource"`
	Links        []Link   `json:"links"`
	EventVersion string   `json:"event_version"`
}

type Resource struct {
	AgreementDetails AgreementDetails `json:"agreement_details"`
	Description      string           `json:"description"`
	Links            []Link           `json:"links"`
	ID               string           `json:"id"`
	ShippingAddress  ShippingAddress  `json:"shipping_address"`
	State            string           `json:"state"`
	Plan             Plan             `json:"plan"`
	Payer            Payer            `json:"payer"`
	StartDate        string           `json:"start_date"`
}

type AgreementDetails struct {
	OutstandingBalance  Amount `json:"outstanding_balance"`
	NumCyclesRemaining  string `json:"num_cycles_remaining"`
	NumCyclesCompleted  string `json:"num_cycles_completed"`
	FinalPaymentDueDate string `json:"final_payment_due_date"`
	FailedPaymentCount  string `json:"failed_payment_count"`
}

type ShippingAddress struct {
	RecipientName string `json:"recipient_name"`
	Line1         string `json:"line1"`
	Line2         string `json:"line2"`
	City          string `json:"city"`
	State         string `json:"state"`
	PostalCode    string `json:"postal_code"`
	CountryCode   string `json:"country_code"`
}

type Plan struct {
	CurrencyCode        string              `json:"curr_code"`
	Links               []Link              `json:"links"`
	PaymentDefinitions  []PaymentDefinition `json:"payment_definitions"`
	MerchantPreferences MerchantPreferences `json:"merchant_preferences"`
}

type PaymentDefinition struct {
	Type              string        `json:"type"`
	Frequency         string        `json:"frequency"`
	FrequencyInterval string        `json:"frequency_interval"`
	Amount            Amount        `json:"amount"`
	Cycles            string        `json:"cycles"`
	ChargeModels      []ChargeModel `json:"charge_models"`
}

type ChargeModel struct {
	Type   string `json:"type"`
	Amount Amount `json:"amount"`
}

type MerchantPreferences struct {
	SetupFee        Amount `json:"setup_fee"`
	AutoBillAmount  string `json:"auto_bill_amount"`
	MaxFailAttempts string `json:"max_fail_attempts"`
}

type Payer struct {
	PaymentMethod string    `json:"payment_method"`
	Status        string    `json:"status"`
	PayerInfo     PayerInfo `json:"payer_info"`
}

type PayerInfo struct {
	Email           string          `json:"email"`
	FirstName       string          `json:"first_name"`
	LastName        string          `json:"last_name"`
	PayerID         string          `json:"payer_id"`
	ShippingAddress ShippingAddress `json:"shipping_address"`
}

type Amount struct {
	Value string `json:"value"`
}

type Link struct {
	Href    string `json:"href"`
	Rel     string `json:"rel"`
	Method  string `json:"method"`
	EncType string `json:"encType,omitempty"`
}
