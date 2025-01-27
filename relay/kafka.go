package relay

import (
	"context"
	"fmt"
	"time"

	"github.com/batchcorp/schemas/build/go/events/records"
	"github.com/batchcorp/schemas/build/go/services"
	"github.com/pkg/errors"
	"google.golang.org/grpc"

	"github.com/batchcorp/plumber/backends/kafka/types"
)

var (
	errMissingMessage      = errors.New("msg cannot be nil")
	errMissingMessageValue = errors.New("msg.Value cannot be nil")
)

// handleKafka sends a Kafka relay message to the GRPC server
func (r *Relay) handleKafka(ctx context.Context, conn *grpc.ClientConn, messages []interface{}) error {
	sinkRecords, err := r.convertMessagesToKafkaSinkRecords(messages)
	if err != nil {
		return fmt.Errorf("unable to convert messages to kafka sink records: %s", err)
	}

	client := services.NewGRPCCollectorClient(conn)

	return r.CallWithRetry(ctx, "AddKafkaRecord", func(ctx context.Context) error {
		_, err := client.AddKafkaRecord(ctx, &services.KafkaSinkRecordRequest{
			Records: sinkRecords,
		}, grpc.MaxCallRecvMsgSize(MaxGRPCMessageSize))
		return err
	})
}

// validateKafkaRelayMessage ensures all necessary values are present for a Kafka relay message
func (r *Relay) validateKafkaRelayMessage(msg *types.RelayMessage) error {
	if msg == nil {
		return errMissingMessage
	}

	if msg.Value == nil {
		return errMissingMessageValue
	}

	return nil
}

// convertKafkaMessageToProtobufRecord creates a records.KafkaSinkRecord from a kafka.Message which can then
// be sent to the GRPC server
func (r *Relay) convertMessagesToKafkaSinkRecords(messages []interface{}) ([]*records.KafkaSinkRecord, error) {
	sinkRecords := make([]*records.KafkaSinkRecord, 0)

	for i, v := range messages {
		relayMessage, ok := v.(*types.RelayMessage)
		if !ok {
			return nil, fmt.Errorf("unable to type assert incoming message as RelayMessage (index: %d)", i)
		}

		if err := r.validateKafkaRelayMessage(relayMessage); err != nil {
			return nil, fmt.Errorf("unable to validate kafka relay message (index: %d): %s", i, err)
		}

		sinkRecords = append(sinkRecords, &records.KafkaSinkRecord{
			Topic:     relayMessage.Value.Topic,
			Key:       relayMessage.Value.Key,
			Value:     relayMessage.Value.Value,
			Timestamp: time.Now().UTC().UnixNano(),
			Offset:    relayMessage.Value.Offset,
			Partition: int32(relayMessage.Value.Partition),
		})
	}

	return sinkRecords, nil
}
