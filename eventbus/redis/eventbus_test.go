package redis_test

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/dobyte/due/eventbus/redis/v2"
	"github.com/dobyte/due/v2/eventbus"
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
		eb  = redis.NewEventbus()
		ctx = context.Background()
	)

	defer eb.Close()

	if _, err = eb.Subscribe(ctx, loginTopic, loginEventHandler); err != nil {
		t.Fatal(err)
	}

	if _, err = eb.Subscribe(ctx, paidTopic, paidEventHandler); err != nil {
		t.Fatal(err)
	}

	t.Log("subscribe success")

	time.Sleep(30 * time.Second)
}

func TestEventbus_Client2_Subscribe(t *testing.T) {
	var (
		err error
		eb  = redis.NewEventbus()
		ctx = context.Background()
	)

	defer eb.Close()

	sub1, err := eb.Subscribe(ctx, loginTopic, loginEventHandler)
	if err != nil {
		t.Fatal(err)
	}

	_, err = eb.Subscribe(ctx, paidTopic, paidEventHandler)
	if err != nil {
		t.Fatal(err)
	}

	if err = sub1.Unsubscribe(ctx); err != nil {
		t.Fatal(err)
	}

	t.Log("subscribe success")

	time.Sleep(30 * time.Second)
}

func TestEventbus_Publish(t *testing.T) {
	var (
		err error
		eb  = redis.NewEventbus()
		ctx = context.Background()
	)

	defer eb.Close()

	if err = eb.Publish(ctx, loginTopic, "login"); err != nil {
		t.Fatal(err)
	}

	if err = eb.Publish(ctx, paidTopic, "paid"); err != nil {
		t.Fatal(err)
	}

	t.Log("publish success")
}

func TestEventbus_Subscribe_SingleConsumer(t *testing.T) {
	var (
		err error
		eb  = redis.NewEventbus()
		ctx = context.Background()
	)

	defer eb.Close()

	if _, err = eb.Subscribe(ctx, loginTopic, func(event *eventbus.Event) {
		fmt.Println("1-------------", event)
	}, true); err != nil {
		t.Fatal(err)
	}

	if _, err = eb.Subscribe(ctx, loginTopic, func(event *eventbus.Event) {
		fmt.Println("2-------------", event)
	}, true); err != nil {
		t.Fatal(err)
	}

	if _, err = eb.Subscribe(ctx, loginTopic, func(event *eventbus.Event) {
		fmt.Println("3-------------", event)
	}, true); err != nil {
		t.Fatal(err)
	}

	t.Log("subscribe queue success")

	time.Sleep(time.Second)

	for i := 0; i < 10; i++ {
		if err = eb.Publish(ctx, loginTopic, "login"); err != nil {
			t.Fatal(err)
		}
	}

	t.Log("publish login success")

	time.Sleep(3 * time.Second)
}
