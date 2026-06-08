package models

import "time"

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Phone     string    `json:"phone,omitempty"`
	FullName  string    `json:"fullName"`
	CreatedAt time.Time `json:"createdAt"`
}

type Account struct {
	ID           string `json:"id"`
	Currency     string `json:"currency"`
	BalanceCents int64  `json:"balanceCents"`
}

// Transaction is the API view of a money movement, rendered relative to the
// account that is querying it (Direction = "in" | "out").
type Transaction struct {
	ID          string    `json:"id"`
	Direction   string    `json:"direction"`
	Counterpart string    `json:"counterpart"`
	AmountCents int64     `json:"amountCents"`
	Currency    string    `json:"currency"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
}
