package redis_test

import (
	"context"
	"github.com/dobyte/due/eventbus"
	"github.com/dobyte/due/eventbus/redis"
	"testing"
)

var eb = redis.NewEventbus(
	redis.WithAddrs("127.0.0.1:6379"),
)

const (
	loginTopic = "login"
	paidTopic  = "paid"
)

func TestEventbus_Publish(t *testing.T) {
	defer eb.Close()

	err := eb.Subscribe(context.Background(), loginTopic, func(payload *eventbus.Event) {
		t.Log(payload)
	})
	if err != nil {
		t.Fatal(err)
	}

	err = eb.Publish(context.Background(), loginTopic, "login")
	if err != nil {
		t.Fatal(err)
	}

	err = eb.Subscribe(context.Background(), paidTopic, func(event *eventbus.Event) {
		t.Logf("%+v", event.Payload.Int64())
	})
	if err != nil {
		t.Fatal(err)
	}

	//time.Sleep(1 * time.Second)

	//err = eb.Publish(context.Background(), loginTopic, "login")
	//if err != nil {
	//	t.Fatal(err)
	//}

	for i := 0; i < 10; i++ {
		err = eb.Publish(context.Background(), paidTopic, 100)
		if err != nil {
			t.Fatal(err)
		}
	}

	t.Log("publish success")

	select {}
}
