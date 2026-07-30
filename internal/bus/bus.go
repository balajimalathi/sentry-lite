package bus

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
)

type Bus struct {
	Client *kgo.Client
	Topic  string
}

func New(brokers []string, topic string) (*Bus, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.AllowAutoTopicCreation(),
		kgo.DefaultProduceTopic(topic),
		kgo.ConsumerGroup("sentry-lite-processor"),
		kgo.ConsumeTopics(topic),
		kgo.DisableAutoCommit(),
	)
	if err != nil {
		return nil, fmt.Errorf("kafka client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	admin := kadm.NewClient(client)
	_, err = admin.CreateTopics(ctx, 1, 1, nil, topic)
	if err != nil {
		// Topic may already exist; franz-go CreateTopics returns per-topic errors
		log.Printf("topic create note: %v", err)
	}

	return &Bus{Client: client, Topic: topic}, nil
}

func (b *Bus) Produce(ctx context.Context, key, value []byte) error {
	r := &kgo.Record{Topic: b.Topic, Key: key, Value: value}
	res := b.Client.ProduceSync(ctx, r)
	return res.FirstErr()
}

func (b *Bus) Close() {
	b.Client.Close()
}
