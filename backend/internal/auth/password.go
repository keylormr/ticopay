package auth

import "golang.org/x/crypto/bcrypt"

func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	return string(b), err
}

func CheckPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// DummyHash is a valid bcrypt hash (DefaultCost, matches no real password). Used
// to equalize login timing: comparing against it on the "user not found" path
// pays the same bcrypt cost as a real check, closing the user-enumeration
// timing side channel.
var DummyHash = func() string {
	h, _ := bcrypt.GenerateFromPassword([]byte("ticopay-timing-equalizer"), bcrypt.DefaultCost)
	return string(h)
}()
