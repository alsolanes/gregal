// Package client exposes the shared Go client for custom chat interfaces and
// server-side adapters. It uses the same implementation as the terminal client.
package client

import (
	"context"
	internalclient "gregal/internal/client"
)

type Client = internalclient.Client
type Health = internalclient.Health
type Event = internalclient.Event
type EventsPage = internalclient.EventsPage
type Session = internalclient.Session
type Run = internalclient.Run
type Submission = internalclient.Submission

const Protocol = internalclient.Protocol

var ErrEventsStreamUnsupported = internalclient.ErrEventsStreamUnsupported

func New(baseURL, token, instanceID string) *Client {
	return internalclient.New(baseURL, token, instanceID)
}

func Discover(ctx context.Context) (*Client, Health, error) {
	return internalclient.Discover(ctx)
}
