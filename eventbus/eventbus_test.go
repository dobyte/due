package eventbus_test

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/dobyte/due/v2/eventbus"
	"github.com/dobyte/due/v2/eventbus/process"
)

const (
	loginTopic = "login"
	paidTopic  = "paid"
)

var eb = process.NewEventbus()

func loginEventHandler(event *eventbus.Event) {
	log.Printf("%+v\n", event)
}

func paidEventHandler(event *eventbus.Event) {
	log.Printf("%+v\n", event)
}

func TestEventbus_Subscribe(t *testing.T) {
	var (
		err error
		ctx = context.Background()
	)

	err = eb.Subscribe(ctx, loginTopic, loginEventHandler)
	if err != nil {
		t.Fatal(err)
	}

	err = eb.Subscribe(ctx, paidTopic, paidEventHandler)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("subscribe success")

	err = eb.Publish(ctx, loginTopic, "login")
	if err != nil {
		t.Fatal(err)
	}

	err = eb.Publish(ctx, paidTopic, "paid")
	if err != nil {
		t.Fatal(err)
	}

	t.Log("publish success")

	time.Sleep(30 * time.Second)
}
