package broker

import (
	"context"
	"os"

	amqp "github.com/rabbitmq/amqp091-go"
)

const queueName = "image_processing_queue"

type RabbitMQ struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

func NewRabbitMQ() (*RabbitMQ, error) {
	conn, err := amqp.Dial(os.Getenv("RABBITMQ_URL"))
	if err != nil {
		return nil, err
	}

	channel, err := conn.Channel()
	if err != nil {
		return nil, err
	}

	// Создаём очередь (если она уже есть - ничего не произойдёт).
	// true = durable: очередь не пропадёт после перезапуска RabbitMQ
	_, err = channel.QueueDeclare(queueName, true, false, false, false, nil)
	if err != nil {
		return nil, err
	}

	return &RabbitMQ{conn: conn, channel: channel}, nil
}

// Publish кладёт сообщение в очередь (это делает API)
func (r *RabbitMQ) Publish(message []byte) error {
	return r.channel.PublishWithContext(context.Background(), "", queueName, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent, // сохранить сообщение на диск
		Body:         message,
	})
}

// Consume возвращает канал, из которого приходят сообщения (это использует воркер)
func (r *RabbitMQ) Consume() (<-chan amqp.Delivery, error) {
	// Давать воркеру по одной задаче за раз
	r.channel.Qos(1, 0, false)

	// false = autoAck выключен: воркер сам скажет "готово" через msg.Ack
	return r.channel.Consume(queueName, "", false, false, false, false, nil)
}
