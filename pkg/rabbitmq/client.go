package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/guyuxiang/gin-apiserver/pkg/config"
)

const publishTimeout = 5 * time.Second

type consumeSession struct {
	conn *amqp.Connection
	ch   *amqp.Channel
}

type client struct {
	cfg      *config.RabbitMQ
	mu       sync.Mutex
	conn     *amqp.Connection
	ch       *amqp.Channel
	returns  chan amqp.Return
	confirms chan amqp.Confirmation
}

var defaultClient = &client{}

func ShouldUse(cfg *config.RabbitMQ) bool {
	return cfg != nil && (cfg.Enabled || strings.TrimSpace(cfg.URL) != "")
}

func Init(rabbitCfg *config.RabbitMQ) (*amqp.Channel, error) {
	normalized, err := normalizeConfig(rabbitCfg)
	if err != nil {
		return nil, err
	}
	if err := defaultClient.init(normalized); err != nil {
		return nil, err
	}
	return defaultClient.Channel(), nil
}

func Connection() *amqp.Connection {
	return defaultClient.Connection()
}

func Channel() *amqp.Channel {
	return defaultClient.Channel()
}

func Publish(body []byte) error {
	return PublishWithTopology(body, defaultTopology(defaultClient.config()))
}

func PublishWithTopology(body []byte, topology Topology) error {
	return defaultClient.publish(body, topology)
}

func Consume(consumer string, autoAck bool) (<-chan amqp.Delivery, func() error, error) {
	return ConsumeWithTopology(defaultTopology(defaultClient.config()), consumer, autoAck)
}

func ConsumeWithTopology(topology Topology, consumer string, autoAck bool) (<-chan amqp.Delivery, func() error, error) {
	return defaultClient.consume(topology, consumer, autoAck)
}

func Close() error {
	return defaultClient.close()
}

type Topology struct {
	Exchange     string
	ExchangeType string
	Queue        string
	RoutingKey   string
	Prefetch     int
}

func (c *client) init(cfg *config.RabbitMQ) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cfg = cfg
	return c.ensureConnectedLocked(defaultTopology(cfg))
}

func (c *client) Connection() *amqp.Connection {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn
}

func (c *client) Channel() *amqp.Channel {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ch
}

func (c *client) config() *config.RabbitMQ {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cfg
}

func (c *client) publish(body []byte, topology Topology) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	cfg := c.cfg
	if cfg == nil {
		return errors.New("rabbitmq is not initialized")
	}
	if err := c.ensureConnectedLocked(topology); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), publishTimeout)
	defer cancel()

	if err := c.ch.PublishWithContext(ctx, topology.Exchange, topology.RoutingKey, true, false, amqp.Publishing{
		ContentType:  "application/octet-stream",
		Body:         body,
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
	}); err != nil {
		c.resetLocked()
		return err
	}

	select {
	case returned := <-c.returns:
		c.resetLocked()
		return fmt.Errorf("rabbitmq returned exchange=%s routingKey=%s replyCode=%d replyText=%s", topology.Exchange, returned.RoutingKey, returned.ReplyCode, returned.ReplyText)
	case confirm := <-c.confirms:
		if !confirm.Ack {
			c.resetLocked()
			return fmt.Errorf("rabbitmq publish nack exchange=%s deliveryTag=%d", topology.Exchange, confirm.DeliveryTag)
		}
		return nil
	case <-ctx.Done():
		c.resetLocked()
		return fmt.Errorf("rabbitmq publish confirm timeout exchange=%s: %w", topology.Exchange, ctx.Err())
	}
}

func (c *client) consume(topology Topology, consumer string, autoAck bool) (<-chan amqp.Delivery, func() error, error) {
	c.mu.Lock()
	cfg := c.cfg
	c.mu.Unlock()

	if cfg == nil {
		return nil, nil, errors.New("rabbitmq is not initialized")
	}
	if strings.TrimSpace(topology.Queue) == "" {
		return nil, nil, errors.New("rabbitmq queue is empty")
	}

	session, err := openConsumeSession(cfg, topology)
	if err != nil {
		return nil, nil, err
	}

	deliveries, err := session.ch.Consume(
		topology.Queue,
		consumer,
		autoAck,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		_ = session.ch.Close()
		_ = session.conn.Close()
		return nil, nil, err
	}

	closeFn := func() error {
		var firstErr error
		if err := session.ch.Close(); err != nil {
			firstErr = err
		}
		if err := session.conn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		return firstErr
	}

	return deliveries, closeFn, nil
}

func (c *client) close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cfg = nil
	return c.closeResourcesLocked()
}

func (c *client) ensureConnectedLocked(topology Topology) error {
	if c.ch != nil && !c.ch.IsClosed() && c.conn != nil && !c.conn.IsClosed() {
		return declareTopology(c.ch, topology)
	}

	c.resetLocked()

	cfg := c.cfg
	if cfg == nil {
		return errors.New("rabbitmq config is nil")
	}

	conn, err := amqp.Dial(cfg.URL)
	if err != nil {
		return err
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return err
	}

	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return err
	}

	if err := declareTopology(ch, topology); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return err
	}

	c.conn = conn
	c.ch = ch
	c.returns = ch.NotifyReturn(make(chan amqp.Return, 1))
	c.confirms = ch.NotifyPublish(make(chan amqp.Confirmation, 1))
	return nil
}

func (c *client) resetLocked() {
	_ = c.closeResourcesLocked()
	c.returns = nil
	c.confirms = nil
}

func (c *client) closeResourcesLocked() error {
	var firstErr error
	if c.ch != nil {
		if err := c.ch.Close(); err != nil {
			firstErr = err
		}
		c.ch = nil
	}
	if c.conn != nil {
		if err := c.conn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		c.conn = nil
	}
	return firstErr
}

func openConsumeSession(cfg *config.RabbitMQ, topology Topology) (*consumeSession, error) {
	conn, err := amqp.Dial(cfg.URL)
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	if topology.Prefetch > 0 {
		if err := ch.Qos(topology.Prefetch, 0, false); err != nil {
			_ = ch.Close()
			_ = conn.Close()
			return nil, err
		}
	}

	if err := declareTopology(ch, topology); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, err
	}

	return &consumeSession{conn: conn, ch: ch}, nil
}

func declareTopology(ch *amqp.Channel, topology Topology) error {
	if strings.TrimSpace(topology.Exchange) != "" {
		if err := ch.ExchangeDeclare(
			topology.Exchange,
			topology.ExchangeType,
			true,
			false,
			false,
			false,
			nil,
		); err != nil {
			return err
		}
	}

	if strings.TrimSpace(topology.Queue) == "" {
		return nil
	}

	if _, err := ch.QueueDeclare(
		topology.Queue,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return err
	}

	if strings.TrimSpace(topology.Exchange) == "" {
		return nil
	}

	return ch.QueueBind(topology.Queue, topology.RoutingKey, topology.Exchange, false, nil)
}

func normalizeConfig(rabbitCfg *config.RabbitMQ) (*config.RabbitMQ, error) {
	if rabbitCfg == nil {
		return nil, errors.New("rabbitmq config is nil")
	}
	if strings.TrimSpace(rabbitCfg.URL) == "" {
		return nil, errors.New("rabbitmq url is empty")
	}

	normalized := *rabbitCfg
	normalized.URL = strings.TrimSpace(normalized.URL)
	normalized.Exchange = strings.TrimSpace(normalized.Exchange)
	normalized.ExchangeType = strings.TrimSpace(normalized.ExchangeType)
	normalized.Queue = strings.TrimSpace(normalized.Queue)
	normalized.RoutingKey = strings.TrimSpace(normalized.RoutingKey)
	normalized.TxExchange = strings.TrimSpace(normalized.TxExchange)
	normalized.RollbackExchange = strings.TrimSpace(normalized.RollbackExchange)
	normalized.TxQueue = strings.TrimSpace(normalized.TxQueue)
	normalized.RollbackQueue = strings.TrimSpace(normalized.RollbackQueue)

	if normalized.ExchangeType == "" {
		normalized.ExchangeType = "direct"
	}
	if normalized.PrefetchCount <= 0 {
		normalized.PrefetchCount = 10
	}
	if normalized.RetryDelayMs <= 0 {
		normalized.RetryDelayMs = 5000
	}
	if normalized.MaxRetry <= 0 {
		normalized.MaxRetry = 3
	}
	if normalized.TxExchange == "" {
		normalized.TxExchange = normalized.Exchange
	}
	if normalized.RollbackExchange == "" {
		normalized.RollbackExchange = normalized.Exchange
	}
	if normalized.TxQueue == "" {
		normalized.TxQueue = normalized.Queue
	}
	if normalized.RollbackQueue == "" {
		normalized.RollbackQueue = normalized.Queue
	}
	if normalized.RoutingKey == "" && normalized.Queue != "" {
		normalized.RoutingKey = normalized.Queue
	}

	return &normalized, nil
}

func defaultTopology(cfg *config.RabbitMQ) Topology {
	if cfg == nil {
		return Topology{}
	}
	return Topology{
		Exchange:     cfg.Exchange,
		ExchangeType: cfg.ExchangeType,
		Queue:        cfg.Queue,
		RoutingKey:   cfg.RoutingKey,
		Prefetch:     cfg.PrefetchCount,
	}
}
