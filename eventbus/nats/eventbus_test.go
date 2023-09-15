package nats_test

import (
	"context"
	"github.com/symsimmy/due/eventbus"
	"github.com/symsimmy/due/eventbus/nats"
	"log"
	"testing"
	"time"
)

const (
	loginTopic = "login"
	paidTopic  = "paid"
)

func loginEventHandler(event *eventbus.Event) {
	log.Printf("%+v\n", event)
}

func paidEventHandler(event *eventbus.Event) {
	log.Printf("%+v\n", event)
}

func TestEventbus_Client1_Subscribe(t *testing.T) {
	var (
		err error
		eb  = nats.NewEventbus()
		ctx = context.Background()
	)

	defer eb.Close()

	err = eb.Subscribe(ctx, loginTopic, loginEventHandler)
	if err != nil {
		t.Fatal(err)
	}

	err = eb.Subscribe(ctx, paidTopic, paidEventHandler)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("subscribe success")

	time.Sleep(30 * time.Second)
}

func TestEventbus_Client2_Subscribe(t *testing.T) {
	var (
		err error
		eb  = nats.NewEventbus()
		ctx = context.Background()
	)

	defer eb.Close()

	err = eb.Subscribe(ctx, loginTopic, loginEventHandler)
	if err != nil {
		t.Fatal(err)
	}

	err = eb.Subscribe(ctx, paidTopic, paidEventHandler)
	if err != nil {
		t.Fatal(err)
	}

	err = eb.Unsubscribe(context.Background(), loginTopic, loginEventHandler)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("subscribe success")

	time.Sleep(30 * time.Second)
}

func TestEventbus_Publish(t *testing.T) {
	var (
		err error
		eb  = nats.NewEventbus()
		ctx = context.Background()
	)

	defer eb.Close()

	err = eb.Publish(ctx, loginTopic, "login")
	if err != nil {
		t.Fatal(err)
	}

	err = eb.Publish(ctx, paidTopic, "paid")
	if err != nil {
		t.Fatal(err)
	}

	err = eb.Publish(ctx, loginTopic, "login")
	if err != nil {
		t.Fatal(err)
	}

	t.Log("publish success")
}
