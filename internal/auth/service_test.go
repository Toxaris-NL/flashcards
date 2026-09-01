package auth

import (
	"testing"
	"time"
)

func TestSignupRequiresUniqueUsernameAndSixDigitPIN(t *testing.T) {
	store := MustNewStore(t.TempDir() + "/kids.json")
	service := NewService(store, nil)

	if _, err := service.Signup("sander", "12345"); err == nil {
		t.Fatal("expected invalid PIN rejection")
	}

	kid, err := service.Signup("sander", "123456")
	if err != nil {
		t.Fatalf("signup failed: %v", err)
	}
	if kid.Status != StatusPending {
		t.Fatalf("expected pending status, got %q", kid.Status)
	}

	if _, err := service.Signup("SANDER", "654321"); err == nil {
		t.Fatal("expected duplicate username rejection")
	}
}

func TestLoginRejectsPendingOrDisabledAccounts(t *testing.T) {
	store := MustNewStore(t.TempDir() + "/kids.json")
	service := NewService(store, nil)

	kid, err := service.Signup("mia", "112233")
	if err != nil {
		t.Fatalf("signup failed: %v", err)
	}

	if _, err := service.Login("mia", "112233"); err == nil {
		t.Fatal("expected pending account to be rejected")
	}

	if err := service.Approve(kid.ID); err != nil {
		t.Fatalf("approve failed: %v", err)
	}

	if _, err := service.Login("mia", "112233"); err != nil {
		t.Fatalf("approved login failed: %v", err)
	}

	if err := service.Disable(kid.ID); err != nil {
		t.Fatalf("disable failed: %v", err)
	}

	if _, err := service.Login("mia", "112233"); err == nil {
		t.Fatal("expected disabled account to be rejected")
	}
}

func TestLoginRateLimitAfterRepeatedFailures(t *testing.T) {
	store := MustNewStore(t.TempDir() + "/kids.json")
	service := NewService(store, nil)

	if _, err := service.Signup("noa", "987654"); err != nil {
		t.Fatalf("signup failed: %v", err)
	}

	if err := service.ApproveByUsername("noa"); err != nil {
		t.Fatalf("approve failed: %v", err)
	}

	for i := 0; i < 5; i++ {
		if _, err := service.Login("noa", "000000"); err == nil {
			t.Fatalf("expected login failure at attempt %d", i+1)
		}
	}

	if _, err := service.Login("noa", "987654"); err == nil {
		t.Fatal("expected rate limit to block further attempts")
	}

	if time.Until(service.lockoutUntil("noa")) <= 0 {
		t.Fatal("expected a non-expired lockout window")
	}
}

func TestAdminCanCreateApprovedKidAndApprovePending(t *testing.T) {
	store := MustNewStore(t.TempDir() + "/kids.json")
	service := NewService(store, nil)

	kid, err := service.CreateApprovedKid("adminkid", "222222")
	if err != nil {
		t.Fatalf("create approved kid failed: %v", err)
	}
	if kid.Status != StatusApproved {
		t.Fatalf("expected approved status, got %q", kid.Status)
	}

	pendingKid, err := service.Signup("pendingkid", "333333")
	if err != nil {
		t.Fatalf("signup failed: %v", err)
	}
	if pendingKid.Status != StatusPending {
		t.Fatalf("expected pending status, got %q", pendingKid.Status)
	}

	if err := service.Approve(pendingKid.ID); err != nil {
		t.Fatalf("approve pending kid failed: %v", err)
	}
}

func TestAdminCanRejectPendingAndReenableDisabledKids(t *testing.T) {
	store := MustNewStore(t.TempDir() + "/kids.json")
	service := NewService(store, nil)
	pending, err := service.Signup("pending", "111222")
	if err != nil {
		t.Fatalf("signup pending kid: %v", err)
	}
	if err := service.Reject(pending.ID); err != nil {
		t.Fatalf("reject pending kid: %v", err)
	}
	if _, ok := store.Get(pending.ID); ok {
		t.Fatal("rejected pending kid must be deleted")
	}

	kid, err := service.CreateApprovedKid("approved", "333444")
	if err != nil {
		t.Fatalf("create approved kid: %v", err)
	}
	if err := service.Disable(kid.ID); err != nil {
		t.Fatalf("disable kid: %v", err)
	}
	if err := service.Enable(kid.ID); err != nil {
		t.Fatalf("enable kid: %v", err)
	}
	enabled, ok := store.Get(kid.ID)
	if !ok || enabled.Status != StatusApproved {
		t.Fatalf("enabled kid = %#v, exists = %t", enabled, ok)
	}
}

func TestAdminPINResetRequiresStudentToChooseNewPIN(t *testing.T) {
	store := MustNewStore(t.TempDir() + "/kids.json")
	service := NewService(store, nil)
	kid, err := service.CreateApprovedKid("mia", "112233")
	if err != nil {
		t.Fatalf("create approved kid: %v", err)
	}
	if err := service.ResetPIN(kid.ID, "445566"); err != nil {
		t.Fatalf("reset PIN: %v", err)
	}
	if !service.MustChangePIN(kid.ID) {
		t.Fatal("student must be required to change the temporary PIN")
	}
	if err := service.ChangePIN(kid.ID, "445566", "778899"); err != nil {
		t.Fatalf("change PIN: %v", err)
	}
	if service.MustChangePIN(kid.ID) {
		t.Fatal("student must no longer be required to change the new PIN")
	}
	if _, err := service.Login("mia", "778899"); err != nil {
		t.Fatalf("login with new PIN: %v", err)
	}
}

func MustNewStore(path string) *Store {
	store, err := NewStore(path)
	if err != nil {
		panic(err)
	}
	return store
}
