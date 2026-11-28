package router

import "testing"

func TestManagedOwnerHelpers(t *testing.T) {
	options := map[string]string{}
	SetManagedOwner(options, "token-a")
	if !IsManaged(options) || ManagedOwner(options) != "token-a" {
		t.Fatalf("owner helpers did not record a token: %#v", options)
	}
	SetManagedOwner(options, "token-b")
	if ManagedOwner(options) != "token-b" {
		t.Fatalf("owner token was not replaced: %#v", options)
	}
	options[ManagedOption] = "0"
	delete(options, OwnerTokenOption)
	if IsManaged(options) || ManagedOwner(options) != "" {
		t.Fatalf("released section still reports an owner: %#v", options)
	}
}
