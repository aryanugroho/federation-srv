package handler

import (
	"github.com/micro/federation-srv/federation"
	proto "github.com/micro/federation-srv/proto/federation"
)

func brokerToProto(b federation.Broker) *proto.Broker {
	plugins := make(map[string]*proto.Plugin)

	for name, p := range b {
		plugins[name] = &proto.Plugin{
			Hosts: p.Hosts,
		}
	}

	return &proto.Broker{
		Plugins: plugins,
	}
}

func topicToProto(t federation.Topic) *proto.Topic {
	return &proto.Topic{
		Rate:      t.Rate,
		Publish:   t.Publish,
		Subscribe: t.Subscribe,
	}
}

func configToProto(c federation.Config) *proto.Config {
	topics := make(map[string]*proto.Topic)
	brokers := make(map[string]*proto.Broker)

	for name, topic := range c.Topics {
		topics[name] = topicToProto(topic)
	}

	for region, b := range c.Brokers {
		brokers[region] = brokerToProto(b)
	}

	return &proto.Config{
		Topics:  topics,
		Brokers: brokers,
	}
}
