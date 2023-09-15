module github.com/symsimmy/due/eventbus/kafka

go 1.16

require (
	github.com/Shopify/sarama v1.38.1
	github.com/dobyte/due v0.0.24
)

replace github.com/dobyte/due => ./../../
