package v201

import (
	"context"
	"testing"

	"github.com/vefacaglar/ocpp-simulator/packages/ocpp-protocol/pkg/protocol"
)

func TestProtocol_Version(t *testing.T) {
	if got := NewProtocol().Version(); got != "2.0.1" {
		t.Fatalf("Version() = %q, want %q", got, "2.0.1")
	}
}

func TestProtocol_BuildMethodsNotImplemented(t *testing.T) {
	p := NewProtocol()
	ctx := context.Background()

	checks := []struct {
		name string
		fn   func() error
	}{
		{"BuildBootNotification", func() error { _, err := p.BuildBootNotification(ctx, protocol.BootNotificationInput{ChargePointVendor: "v", ChargePointModel: "m"}); return err }},
		{"BuildHeartbeat", func() error { _, err := p.BuildHeartbeat(ctx); return err }},
		{"BuildStatusNotification", func() error { _, err := p.BuildStatusNotification(ctx, protocol.StatusNotificationInput{ConnectorID: 1, Status: "Available", ErrorCode: "NoError"}); return err }},
		{"BuildAuthorize", func() error { _, err := p.BuildAuthorize(ctx, protocol.AuthorizeInput{IDTag: "x"}); return err }},
		{"BuildStartTransaction", func() error { _, err := p.BuildStartTransaction(ctx, protocol.StartTransactionInput{}); return err }},
		{"BuildMeterValues", func() error { _, err := p.BuildMeterValues(ctx, protocol.MeterValuesInput{}); return err }},
		{"BuildStopTransaction", func() error { _, err := p.BuildStopTransaction(ctx, protocol.StopTransactionInput{}); return err }},
	}
	for _, c := range checks {
		if err := c.fn(); err == nil {
			t.Errorf("%s: expected ErrNotImplemented, got nil", c.name)
		}
	}
}
