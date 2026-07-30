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
	Producer *kgo.Client
	Consumer *kgo.Client
	Topic    string
}

func New(brokers []string, topic string) (*Bus, error) {
	adminClient, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		return nil, fmt.Errorf("admin client: %w", err)
	}
	defer adminClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin := kadm.NewClient(adminClient)
	if _, err := admin.CreateTopics(ctx, 1, 1, nil, topic); err != nil {
		log.Printf("topic create note: %v", err)
	}

	producer, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.DefaultProduceTopic(topic),
		kgo.AllowAutoTopicCreation(),
	)
	if err != nil {
		return nil, fmt.Errorf("producer: %w", err)
	}

	consumer, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup("sentry-lite-processor"),
		kgo.ConsumeTopics(topic),
		kgo.DisableAutoCommit(),
		kgo.Balancers(kgo.RoundRobinBalancer()),
		kgo.SessionTimeout(10*time.Second),
		kgo.RebalanceTimeout(10*time.Second),
		kgo.HeartbeatInterval(2*time.Second),
	)
	if err != nil {
		producer.Close()
		return nil, fmt.Errorf("consumer: %w", err)
	}

	return &Bus{Producer: producer, Consumer: consumer, Topic: topic}, nil
}

func (b *Bus) Produce(ctx context.Context, key, value []byte) error {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
	}
	r := &kgo.Record{Topic: b.Topic, Key: key, Value: value}
	return b.Producer.ProduceSync(ctx, r).FirstErr()
}

func (b *Bus) Close() {
	b.Consumer.Close()
	b.Producer.Close()
}
