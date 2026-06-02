package ocpp

import "context"

type Client struct {
	protocol Protocol
}

func NewClient(p Protocol) *Client {
	return &Client{protocol: p}
}

func (c *Client) SendBootNotification(ctx context.Context, input BootNotificationInput) (Message, error) {
	return c.protocol.BuildBootNotification(ctx, input)
}
