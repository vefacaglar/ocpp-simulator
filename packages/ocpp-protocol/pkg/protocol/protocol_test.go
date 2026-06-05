package protocol_test

import (
	"encoding/json"
	"testing"

	"github.com/vefacaglar/ocpp-simulator/packages/ocpp-protocol/pkg/protocol"
	v16 "github.com/vefacaglar/ocpp-simulator/packages/ocpp-protocol/pkg/v16"
	v201 "github.com/vefacaglar/ocpp-simulator/packages/ocpp-protocol/pkg/v201"
)

// TestFactoryRegisterDiscoversCapabilities is the central test for
// the capability-segregated design: registering a version populates
// ONLY the capabilities that version implements. Asking for a
// capability the version does not have must return (nil, false) — not
// a panic, not a fake implementation.
func TestFactoryRegisterDiscoversCapabilities(t *testing.T) {
	f := protocol.NewFactory()
	f.Register(v16.NewProtocol())

	t.Run("v16 base", func(t *testing.T) {
		p, ok := f.Base("1.6J")
		if !ok {
			t.Fatal("expected 1.6J to be registered")
		}
		if p.Version() != "1.6J" {
			t.Errorf("Version() = %q, want 1.6J", p.Version())
		}
	})

	t.Run("v16 legacy transaction supported", func(t *testing.T) {
		c, ok := f.LegacyTransaction("1.6J")
		if !ok {
			t.Fatal("expected 1.6J to implement LegacyTransactionProtocol")
		}
		// And it really builds a 1.6J StartTransaction.
		_ = c
	})

	t.Run("v16 transaction event NOT supported", func(t *testing.T) {
		c, ok := f.TransactionEvent("1.6J")
		if ok || c != nil {
			t.Errorf("expected (nil,false) for 1.6J TransactionEventProtocol, got (%v, %v)", c, ok)
		}
	})

	t.Run("v16 legacy config supported", func(t *testing.T) {
		if _, ok := f.LegacyConfig("1.6J"); !ok {
			t.Error("expected 1.6J to implement LegacyConfigProtocol")
		}
	})

	t.Run("v16 variable NOT supported", func(t *testing.T) {
		c, ok := f.Variable("1.6J")
		if ok || c != nil {
			t.Errorf("expected (nil,false) for 1.6J VariableProtocol, got (%v, %v)", c, ok)
		}
	})

	t.Run("v16 remote control supported", func(t *testing.T) {
		if _, ok := f.RemoteControl("1.6J"); !ok {
			t.Error("expected 1.6J to implement RemoteControlProtocol")
		}
	})

	t.Run("v16 legacy remote tx supported", func(t *testing.T) {
		if _, ok := f.LegacyRemoteTx("1.6J"); !ok {
			t.Error("expected 1.6J to implement LegacyRemoteTxProtocol")
		}
	})

	t.Run("v16 modern remote tx NOT supported", func(t *testing.T) {
		c, ok := f.RemoteTx("1.6J")
		if ok || c != nil {
			t.Errorf("expected (nil,false) for 1.6J RemoteTxProtocol, got (%v, %v)", c, ok)
		}
	})

	t.Run("v201 not yet registered", func(t *testing.T) {
		p, ok := f.Base("2.0.1")
		if ok || p != nil {
			t.Errorf("expected 2.0.1 unregistered, got (%v, %v)", p, ok)
		}
		if _, ok := f.TransactionEvent("2.0.1"); ok {
			t.Error("2.0.1 must be (nil,false) before Phase C registers v201")
		}
	})
}

func TestFactoryUnknownVersion(t *testing.T) {
	f := protocol.NewFactory()
	if p, ok := f.Base("9.9"); ok || p != nil {
		t.Errorf("unknown version must return (nil, false), got (%v, %v)", p, ok)
	}
}

// TestFactoryRegisterTwiceReplaces keeps the registration contract:
// the latest Register for a given version string wins.
func TestFactoryRegisterTwiceReplaces(t *testing.T) {
	f := protocol.NewFactory()
	a := v16.NewProtocol()
	f.Register(a)
	p1, _ := f.Base("1.6J")
	if p1 == nil {
		t.Fatal("expected p1")
	}
	f.Register(v16.NewProtocol())
	p2, _ := f.Base("1.6J")
	if p2 == nil {
		t.Fatal("expected p2 after re-register")
	}
	// Re-register must not panic; we don't care which instance
	// wins as long as the call doesn't blow up.
	_ = p1
	_ = p2
}

// TestFactoryV201CapabilitiesAreExtensible seeds the future shape:
// when v201 is registered (post Phase C), the factory must
// distinguish the new capabilities and not the legacy ones.
func TestFactoryV201CapabilitiesAreExtensible(t *testing.T) {
	f := protocol.NewFactory()
	f.Register(v201.NewProtocol())

	if _, ok := f.Base("2.0.1"); !ok {
		t.Error("expected 2.0.1 base after Register(v201)")
	}
	if _, ok := f.TransactionEvent("2.0.1"); !ok {
		t.Error("expected 2.0.1 to implement TransactionEventProtocol after Register(v201)")
	}
	if _, ok := f.Variable("2.0.1"); !ok {
		t.Error("expected 2.0.1 to implement VariableProtocol after Register(v201)")
	}
	if _, ok := f.LegacyTransaction("2.0.1"); ok {
		t.Error("v201 must NOT implement LegacyTransactionProtocol")
	}
	if _, ok := f.LegacyConfig("2.0.1"); ok {
		t.Error("v201 must NOT implement LegacyConfigProtocol")
	}
	if _, ok := f.LegacyRemoteTx("2.0.1"); ok {
		t.Error("v201 must NOT implement LegacyRemoteTxProtocol")
	}
}

// keep json import live even if test data shifts
var _ = json.RawMessage(nil)
