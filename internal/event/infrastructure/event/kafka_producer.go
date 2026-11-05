package event

import (
	"context"
	"log/slog"

	"github.com/refreshTechRusuh/event-service/pkg/kafka"
)

// IPublisher adalah interface minimal untuk kafka producer.
// *kafka.Producer memenuhi interface ini di production.
type IPublisher interface {
	Publish(ctx context.Context, topic, key string, payload any) error
}

// KafkaProducer mengimplementasikan outbound.IEventProducer.
type KafkaProducer struct {
	p   IPublisher
	log *slog.Logger
}

func NewKafkaProducer(p *kafka.Producer, log *slog.Logger) *KafkaProducer {
	return &KafkaProducer{p: p, log: log}
}

// NewKafkaProducerWithPublisher membuat KafkaProducer dengan custom publisher (untuk test).
func NewKafkaProducerWithPublisher(p IPublisher, log *slog.Logger) *KafkaProducer {
	return &KafkaProducer{p: p, log: log}
}

func (k *KafkaProducer) PublishEventSoldOut(ctx context.Context, eventID string, payload any) error {
	return k.p.Publish(ctx, kafka.TopicEventSoldOut, eventID, payload)
}
