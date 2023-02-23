package nats_test

import (
	"context"
	"github.com/dobyte/due/eventbus"
	"github.com/dobyte/due/eventbus/nats"
	"testing"
)

var eb = nats.NewEventbus()

const (
	loginTopic = "login"
	paidTopic  = "paid"
)

func TestEventbus_Publish(t *testing.T) {
	defer eb.Close()

	fn := func(event *eventbus.Event) {
		t.Log(event.Payload.String())
	}

	err := eb.Subscribe(context.Background(), loginTopic, fn)
	if err != nil {
		t.Fatal(err)
	}

	err = eb.Subscribe(context.Background(), loginTopic, fn)
	if err != nil {
		t.Fatal(err)
	}

	err = eb.Unsubscribe(context.Background(), loginTopic, fn)
	if err != nil {
		t.Fatal(err)
	}

	err = eb.Subscribe(context.Background(), loginTopic, func(event *eventbus.Event) {
		t.Logf("%+v", event.Payload.String())
	})
	if err != nil {
		t.Fatal(err)
	}

	err = eb.Publish(context.Background(), loginTopic, "login")
	if err != nil {
		t.Fatal(err)
	}

	t.Log("publish success")

	select {}
}
