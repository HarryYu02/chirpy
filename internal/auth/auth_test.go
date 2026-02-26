package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestJWT(t *testing.T) {
	id := uuid.New()
	secret := "secret"
	expireDuration := time.Hour * 1
	jwt, err := MakeJWT(id, secret, expireDuration)
	if err != nil {
		t.Errorf("MakeJWT error: %v\n", err)
		return
	}
	validatedID, err := ValidateJWT(jwt, secret)
	if validatedID != id || err != nil {
		t.Errorf("ID cannot be validated: %v and %v\n", id, validatedID)
		return
	}
}
