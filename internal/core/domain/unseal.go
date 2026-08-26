package domain

// UnsealSlotID is the stable position of an unseal credential in the N-of-N set.
type UnsealSlotID int

// UnsealKey is the real key material recovered during unseal.
// It must never be persisted or returned to the caller.
type UnsealKey []byte

// OwnerSecret is the secret material held by an unseal owner.
// Sigryx returns it once during initialization and never persists it.
type OwnerSecret []byte

// ServerKeyMaterial is random server-side material persisted by Sigryx.
// Together with OwnerSecret, it derives the wrapping key for WrappedUnsealKey.
type ServerKeyMaterial []byte

// WrappedUnsealKey is the authenticated encrypted representation of UnsealKey.
type WrappedUnsealKey []byte

// UnsealPayload is the non-secret payload an owner keeps alongside OwnerSecret.
type UnsealPayload struct {
	SlotID     UnsealSlotID
	WrappedKey WrappedUnsealKey
}

// UnsealCredential is returned once to an owner during initialization.
type UnsealCredential struct {
	Payload     UnsealPayload
	OwnerSecret OwnerSecret
}
