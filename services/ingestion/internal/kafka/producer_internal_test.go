package kafka

import (
	"testing"

	kafkago "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
)

// A kafka.Writer built as a struct literal defaults RequiredAcks to
// RequireNone (fire-and-forget); only kafka.NewWriter maps 0 to RequireAll.
// Without an explicit RequireAll, Publish returns before the broker has the
// record, and serial writes can be appended out of order.
func TestNewKafkaProducer_RequiresAllAcks(t *testing.T) {
	p := NewKafkaProducer(DefaultConfig())
	defer p.Close()

	assert.Equal(t, kafkago.RequireAll, p.writer.RequiredAcks)
}
