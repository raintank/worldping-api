package jobqueue

import (
	"time"

	"github.com/raintank/worldping-api/pkg/log"
	"github.com/streadway/amqp"
	"golang.org/x/net/context"
)

// session composes an amqp.Connection with an amqp.Channel
type session struct {
	*amqp.Connection
	*amqp.Channel
}

// Close tears the connection down, taking the channel with it.
func (s session) Close() error {
	if s.Connection == nil {
		return nil
	}
	return s.Connection.Close()
}

// redial continually connects to the URL, exiting the program when no longer possible
func redial(ctx context.Context, url, exchange string) chan chan session {
	sessions := make(chan chan session)

	go func() {
		sess := make(chan session)
		defer close(sessions)

		for {
			select {
			case sessions <- sess:
			case <-ctx.Done():
				log.Info("shutting down session factory")
				return
			}

			connected := false
			var conn *amqp.Connection
			var ch *amqp.Channel
			var err error
			for !connected {
				log.Debug("dialing amqp url: %s", url)
				conn, err = amqp.Dial(url)
				if err != nil {
					log.Error(3, "cannot (re)dial: %v: %q", err, url)
					time.Sleep(time.Second)
					continue
				}
				log.Debug("connected to %s", url)

				log.Debug("creating new channel on AMQP connection.")
				ch, err = conn.Channel()
				if err != nil {
					log.Error(3, "cannot create channel: %v", err)
					conn.Close()
					time.Sleep(time.Second)
					continue
				}
				log.Debug("Ensuring that %s x-consistent-hash exchange exists on AMQP server.", exchange)
				if err := ch.ExchangeDeclare(exchange, "x-consistent-hash", true, false, false, false, nil); err != nil {
					log.Error(3, "cannot declare x-consistent-hash exchange: %v", err)
					conn.Close()
					time.Sleep(time.Second)
				}
				log.Debug("Successfully connected to RabbitMQ.")
				connected = true
			}

			select {
			case sess <- session{conn, ch}:
			case <-ctx.Done():
				log.Info("shutting down new session")
				return
			}
		}
	}()

	return sessions
}

// publish publishes messages to a reconnecting session to a topic exchange.
// It receives from the application specific source of messages.
func publish(sessions chan chan session, exchange string, messages <-chan Message) {
	var (
		running bool
		reading = messages
		pending = make(chan Message, 1)
		confirm = make(chan amqp.Confirmation, 1)
	)

	for session := range sessions {
		log.Debug("waiting for new session to be established.")
		pub := <-session

		// publisher confirms for this channel/connection
		if err := pub.Confirm(false); err != nil {
			log.Info("publisher confirms not supported")
			close(confirm) // confirms not supported, simulate by always nacking
		} else {
			pub.NotifyPublish(confirm)
		}

		log.Info("Event publisher started...")

		for {
			var body Message
			select {
			case confirmed := <-confirm:
				if !confirmed.Ack {
					log.Error(3, "nack message %d, body: %q", confirmed.DeliveryTag, string(body.Payload))
				}
				reading = messages

			case body = <-pending:
				err := pub.Publish(exchange, body.RoutingKey, false, false, amqp.Publishing{
					Body: body.Payload,
				})
				// Retry failed delivery on the next session
				if err != nil {
					pending <- body
					pub.Close()
					break
				}

			case body, running = <-reading:
				// all messages consumed
				if !running {
					return
				}
				// work on pending delivery until ack'd
				pending <- body
				reading = nil
			}
		}
	}
}

// subscribe consumes deliveries from an exclusive queue from a fanout exchange and sends to the application specific messages chan.
func subscribe(sessions chan chan session, exchange string, messages chan<- Message) {
	for session := range sessions {
		log.Debug("alerting: waiting for new session to be established.")
		sub := <-session

		log.Debug("alerting: declaring new ephemeral Queue %v", sub)
		q, err := sub.QueueDeclare("", false, true, true, false, nil)
		if err != nil {
			log.Error(3, "cannot consume from exclusive: %v", err)
			sub.Close()
			continue
		}

		log.Debug("alerting: binding queue %s to routingKey 10", q.Name)
		routingKey := "10"
		if err := sub.QueueBind(q.Name, routingKey, exchange, false, nil); err != nil {
			log.Error(3, "alerting: cannot consume without a binding to exchange: %q, %v", exchange, err)
			sub.Close()
			continue
		}

		deliveries, err := sub.Consume(q.Name, "", false, true, false, false, nil)
		if err != nil {
			log.Error(3, "alerting: cannot consume from queue: %q, %v", q.Name, err)
			sub.Close()
			continue
		}

		log.Info("alerting: subscribed to rabbitmq %s exchange...", exchange)

		for msg := range deliveries {
			log.Debug("alerting: new message received from rabbitmq")
			messages <- Message{RoutingKey: msg.RoutingKey, Payload: msg.Body}
			sub.Ack(msg.DeliveryTag, false)
		}
	}
}

func Run(rabbitmqUrl, exchange string, pub, sub chan Message) {
	ctx, done := context.WithCancel(context.Background())
	go func() {
		publish(redial(ctx, rabbitmqUrl, exchange), exchange, pub)
		done()
	}()

	go func() {
		subscribe(redial(ctx, rabbitmqUrl, exchange), exchange, sub)
		done()
	}()

	<-ctx.Done()
}
