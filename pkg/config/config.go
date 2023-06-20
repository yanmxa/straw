package config

import (
	"os"

	goflag "flag"

	flag "github.com/spf13/pflag"
)

var (
	ReceiveTopic string
	SendTopic    string
	QoS          = byte(0)
	Retained     = false
)

type ClientConfig struct {
	*TLSConfig
	KubeConfig string
	Broker     string
	Topic      string
	QoS        byte
	ClientID   string
	Retained   bool
}

type TLSConfig struct {
	EnableTLS  bool
	CACert     string
	ClientCert string
	ClientKey  string
}

func SetReceiveTopic(topic string) {
	ReceiveTopic = topic
}

func SetSendTopic(topic string) {
	SendTopic = topic
}

func SetQoS(qos byte) {
	QoS = qos
}

func SetRetained(retain bool) {
	Retained = retain
}

func GetClientConfig() *ClientConfig {
	clientConfig := &ClientConfig{
		TLSConfig: &TLSConfig{},
	}
	flag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	flag.StringVarP(&clientConfig.KubeConfig, "kubeconfig", "k", "", "the kubeconfig for apiserver")
	flag.StringVarP(&clientConfig.Broker, "broker", "b", "", "the MQTT server")
	flag.BoolVarP(&clientConfig.EnableTLS, "tls", "", false, "whether to enable the TLS connection")
	flag.StringVarP(&clientConfig.CACert, "ca-crt", "", "", "the ca certificate path")
	flag.StringVarP(&clientConfig.ClientCert, "client-crt", "", "", "the client certificate path")
	flag.StringVarP(&clientConfig.ClientKey, "client-key", "", "", "the client key path")
	flag.StringVarP(&clientConfig.ClientID, "client-id", "", "sender", "the client id for the MQTT producer")
	flag.StringVarP(&clientConfig.Topic, "topic", "", "", "the topic for the MQTT consumer")
	QoS := flag.IntP("QoS", "q", 0,
		"the level of reliability and assurance of message delivery between an MQTT client and broker")
	flag.BoolVarP(&clientConfig.Retained, "retained", "", false, "retain the MQTT message or not")

	flag.Parse()
	clientConfig.QoS = byte(*QoS)
	if clientConfig.Broker == "" {
		clientConfig.Broker = os.Getenv("BROKER")
	}
	if clientConfig.KubeConfig == "" {
		clientConfig.KubeConfig = os.Getenv("KUBECONFIG")
	}

	SetReceiveTopic(clientConfig.Topic)
	SetSendTopic(clientConfig.Topic)
	SetRetained(clientConfig.Retained)
	SetQoS(clientConfig.QoS)

	return clientConfig
}
