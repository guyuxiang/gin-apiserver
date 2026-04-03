package config

const (
	SERVICE_NAME = "gin-apiserver"

	FLAG_KEY_SERVER_HOST        = "server.host"
	FLAG_KEY_SERVER_PORT        = "server.port"
	FLAG_KEY_SERVER_VERSION     = "0.1.0"
	FLAG_KEY_AUTH_USERNAME      = "auth.username"
	FLAG_KEY_AUTH_PASSWORD      = "auth.password"
	FLAG_KEY_GIN_MODE           = "gin.mode"
	FLAG_KEY_LOG_LEVEL          = "log.level"
	FLAG_KEY_MYSQL_DSN          = "mysql.dsn"
	FLAG_KEY_MYSQL_MAX_IDLE     = "mysql.maxIdleConns"
	FLAG_KEY_MYSQL_MAX_OPEN     = "mysql.maxOpenConns"
	FLAG_KEY_RABBIT_URL         = "rabbitmq.url"
	FLAG_KEY_RABBIT_ENABLED     = "rabbitmq.enabled"
	FLAG_KEY_RABBIT_EXCH        = "rabbitmq.exchange"
	FLAG_KEY_RABBIT_TYPE        = "rabbitmq.exchangeType"
	FLAG_KEY_RABBIT_QUEUE       = "rabbitmq.queue"
	FLAG_KEY_RABBIT_ROUTING     = "rabbitmq.routingKey"
	FLAG_KEY_RABBIT_TX_EXCH     = "rabbitmq.txExchange"
	FLAG_KEY_RABBIT_RB_EXCH     = "rabbitmq.rollbackExchange"
	FLAG_KEY_RABBIT_TX_QUEUE    = "rabbitmq.txQueue"
	FLAG_KEY_RABBIT_RB_QUEUE    = "rabbitmq.rollbackQueue"
	FLAG_KEY_RABBIT_RETRY_DELAY = "rabbitmq.retryDelayMs"
	FLAG_KEY_RABBIT_MAX_RETRY   = "rabbitmq.maxRetry"
	FLAG_KEY_RABBIT_PREFETCH    = "rabbitmq.prefetchCount"
)
