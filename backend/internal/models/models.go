package models

import "time"

type User struct {
	ID            string    `json:"id"`
	Email         string    `json:"email"`
	Phone         string    `json:"phone,omitempty"`
	FullName      string    `json:"fullName"`
	KYCStatus     string    `json:"kycStatus"` // none | verified
	IDType        string    `json:"idType,omitempty"`
	IDNumber      string    `json:"idNumber,omitempty"`
	EmailVerified bool      `json:"emailVerified"`
	CreatedAt     time.Time `json:"createdAt"`
}

type Account struct {
	ID           string `json:"id"`
	Currency     string `json:"currency"`
	BalanceCents int64  `json:"balanceCents"`
}

// Transaction is the API view of a money movement, rendered relative to the
// account that is querying it (Direction = "in" | "out" | "self").
type Transaction struct {
	ID          string    `json:"id"`
	Direction   string    `json:"direction"`
	Counterpart string    `json:"counterpart"`
	AmountCents int64     `json:"amountCents"`
	Currency    string    `json:"currency"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Kind        string    `json:"kind"` // transfer | conversion | request | pool
	CreatedAt   time.Time `json:"createdAt"`
}

// PaymentRequest is a "cobro" — a request to be paid, shareable by link/QR.
type PaymentRequest struct {
	ID            string    `json:"id"`
	RequesterName string    `json:"requesterName"`
	AmountCents   *int64    `json:"amountCents"` // nil = payer chooses amount
	Currency      string    `json:"currency"`
	Description   string    `json:"description"`
	Status        string    `json:"status"`
	Direction     string    `json:"direction,omitempty"` // incoming | outgoing (in lists)
	CreatedAt     time.Time `json:"createdAt"`
}

type Pool struct {
	ID          string    `json:"id"`
	OwnerName   string    `json:"ownerName"`
	IsOwner     bool      `json:"isOwner"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	GoalCents   int64     `json:"goalCents"`
	RaisedCents int64     `json:"raisedCents"`
	Currency    string    `json:"currency"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
}

type PoolContribution struct {
	Name        string    `json:"name"`
	AmountCents int64     `json:"amountCents"`
	CreatedAt   time.Time `json:"createdAt"`
}
