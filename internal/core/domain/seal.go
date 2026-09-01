package domain

type SealState string

const (
	SealStateUninitialized SealState = "UNINITIALIZED"
	SealStateSealed        SealState = "SEALED"
	SealStateUnsealed      SealState = "UNSEALED"
)

type UnsealKeySlot struct {
	ID                UnsealSlotID
	WrappedKey        WrappedUnsealKey
	ServerKeyMaterial ServerKeyMaterial
}
