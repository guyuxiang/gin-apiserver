package rabbitmq

import (
	"errors"

	"github.com/guyuxiang/gin-apiserver/pkg/config"
	"github.com/streadway/amqp"
)

var (
	conn    *amqp.Connection
	channel *amqp.Channel
)

func Init(cfg *config.RabbitMQ) (*amqp.Channel, error) {
	if cfg == nil {
		return nil, errors.New("rabbitmq config is nil")
	}
	if cfg.URL == "" {
		return nil, errors.New("rabbitmq url is empty")
	}

	var err error
	conn, err = amqp.Dial(cfg.URL)
	if err != nil {
		return nil, err
	}

	channel, err = conn.Channel()
	if err != nil {
		_ = conn.Close()
		conn = nil
		return nil, err
	}

	if cfg.Exchange != "" {
		err = channel.ExchangeDeclare(
			cfg.Exchange,
			"direct",
			true,
			false,
			false,
			false,
			nil,
		)
		if err != nil {
			_ = Close()
			return nil, err
		}
	}

	if cfg.Queue != "" {
		_, err = channel.QueueDeclare(
			cfg.Queue,
			true,
			false,
			false,
			false,
			nil,
		)
		if err != nil {
			_ = Close()
			return nil, err
		}

		if cfg.Exchange != "" {
			err = channel.QueueBind(
				cfg.Queue,
				cfg.Queue,
				cfg.Exchange,
				false,
				nil,
			)
			if err != nil {
				_ = Close()
				return nil, err
			}
		}
	}

	return channel, nil
}

func Connection() *amqp.Connection {
	return conn
}

func Channel() *amqp.Channel {
	return channel
}

func Close() error {
	var firstErr error

	if channel != nil {
		if err := channel.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		channel = nil
	}

	if conn != nil {
		if err := conn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		conn = nil
	}

	return firstErr
}
