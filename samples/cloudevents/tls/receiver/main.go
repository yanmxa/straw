package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"

	mqtt_paho "github.com/cloudevents/sdk-go/protocol/mqtt_paho/v2"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/eclipse/paho.golang/paho"
	"github.com/yanmxa/straw/pkg/utils"
)

const (
	Broker     = "**.iot.us-east-1.amazonaws.com:8883" // "localhost:1883"
	CACert     = "./samples/cloudevents/certs/root-CA.crt"
	ClientCert = "./samples/cloudevents/certs/myan.cert.pem"
	ClientKey  = "./samples/cloudevents/certs/myan.private.key"
	Topic      = "sdk/test/python"
	ClientID   = "sdk-java"
)

func main() {
	ctx := context.Background()
	tlsConfig := utils.NewTLSConfig(CACert, ClientCert, ClientKey)

	conn, err := tls.Dial("tcp", Broker, tlsConfig)
	if err != nil {
		log.Fatalf("failed to connect to %s: %s", Broker, err.Error())
	}

	subscribeOpt := &paho.Subscribe{
		Subscriptions: map[string]paho.SubscribeOptions{
			Topic: {QoS: 0},
		},
	}

	p, err := mqtt_paho.New(ctx,
		&paho.ClientConfig{
			ClientID: ClientID,
			Conn:     conn,
		},
		mqtt_paho.WithSubscribe(subscribeOpt),
	)

	c, err := cloudevents.NewClient(p, cloudevents.WithTimeNow(), cloudevents.WithUUIDs())
	if err != nil {
		log.Fatalf("failed to create client, %v", err)
	}
	log.Printf("receiver start consuming messages from test-topic\n")
	err = c.StartReceiver(ctx, receive)
	if err != nil {
		log.Fatalf("failed to start receiver: %s", err)
	} else {
		log.Printf("receiver stopped\n")
	}
}

func receive(ctx context.Context, event cloudevents.Event) {
	fmt.Printf("%s", event)
}
