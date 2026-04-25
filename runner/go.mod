module spade_runner

go 1.25.3

replace core => ../core

require (
	core v0.0.0-00010101000000-000000000000
	github.com/Azure/go-amqp v1.6.0
	github.com/google/uuid v1.6.0
	github.com/rabbitmq/rabbitmq-amqp-go-client v1.0.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/mattn/go-sqlite3 v1.14.22 // indirect
	go.opentelemetry.io/otel v1.40.0 // indirect
	go.opentelemetry.io/otel/metric v1.40.0 // indirect
	golang.org/x/text v0.26.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	gorm.io/driver/sqlite v1.6.0 // indirect
	gorm.io/gorm v1.31.1 // indirect
)
