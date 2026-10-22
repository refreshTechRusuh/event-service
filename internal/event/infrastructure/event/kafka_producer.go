package event

import (
	"context"
	"log/slog"

	"github.com/refreshTechRusuh/event-service/pkg/kafka"
)

// KafkaProducer mengimplementasikan outbound.IEventProducer.
type KafkaProducer struct {
	p   *kafka.Producer
	log *slog.Logger
}

func NewKafkaProducer(p *kafka.Producer, log *slog.Logger) *KafkaProducer {
	return &KafkaProducer{p: p, log: log}
}

func (k *KafkaProducer) PublishEventSoldOut(ctx context.Context, eventID string, payload any) error {
	return k.p.Publish(ctx, kafka.TopicEventSoldOut, eventID, payload)
}
