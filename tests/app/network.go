package main

// Probes for the network feature: one operation against each dependency, on
// the long-lived client an application keeps, so a scenario can see what a
// fault on the network does to it. Every dependency the suite runs in a
// container is here, because the network resource works on any TCP one.

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	amqp "github.com/rabbitmq/amqp091-go"
)

// probeTimeout bounds every operation, as an application's own timeout would.
const probeTimeout = 2 * time.Second

func registerNetworkProbes(mux *http.ServeMux) {
	probes := map[string]func(ctx context.Context, fresh bool) error{
		"postgres": probePostgres,
		"redis":    func(ctx context.Context, _ bool) error { return rdb.Ping(ctx).Err() },
		"s3":       probeS3,
		"kafka":    probeKafka,
		"rabbitmq": probeRabbitMQ,
	}
	// GET /deps/{name}: one operation on that dependency. ?fresh=1 opens a new
	// connection for it instead of using the pool; ?retry=5s tries again
	// every 200ms until it works or the time is up, as an application that
	// rides out a short outage does.
	mux.HandleFunc("/deps/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/deps/")
		probe, ok := probes[name]
		if !ok {
			http.Error(w, "unknown dependency", http.StatusNotFound)
			return
		}
		var retry time.Duration
		if v := r.URL.Query().Get("retry"); v != "" {
			d, err := time.ParseDuration(v)
			if err != nil {
				http.Error(w, "retry: "+err.Error(), http.StatusBadRequest)
				return
			}
			retry = d
		}
		fresh := r.URL.Query().Get("fresh") == "1"

		start, attempts := time.Now(), 0
		var err error
		for {
			attempts++
			ctx, cancel := context.WithTimeout(r.Context(), probeTimeout)
			err = runWithin(ctx, func(ctx context.Context) error { return probe(ctx, fresh) })
			cancel()
			if err == nil || time.Since(start)+200*time.Millisecond > retry {
				break
			}
			time.Sleep(200 * time.Millisecond)
		}
		body := map[string]any{"dependency": name, "ms": time.Since(start).Milliseconds(), "attempts": attempts}
		status := http.StatusOK
		if err != nil {
			status = http.StatusBadGateway
			body["error"] = err.Error()
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	})
}

// runWithin gives up on an operation whose client ignores its context, as
// lib/pq does on a database that stops answering, so the probe still answers.
func runWithin(ctx context.Context, op func(context.Context) error) error {
	done := make(chan error, 1)
	go func() { done <- op(ctx) }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return fmt.Errorf("gave up after %s: %w", probeTimeout, ctx.Err())
	}
}

func probePostgres(ctx context.Context, fresh bool) error {
	if !fresh {
		_, err := db.ExecContext(ctx, "SELECT 1")
		return err
	}
	c, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer c.Close()
	return c.PingContext(ctx)
}

var (
	s3Once   sync.Once
	s3Client *s3.Client
)

func newS3Client() *s3.Client {
	return s3.New(s3.Options{
		Region:           "us-east-1",
		BaseEndpoint:     aws.String(os.Getenv("S3_ENDPOINT")),
		UsePathStyle:     true,
		Credentials:      credentials.NewStaticCredentialsProvider("minioadmin", "minioadmin", ""),
		RetryMaxAttempts: 1,
		// Its own transport, so a fresh client opens exactly one connection.
		HTTPClient: &http.Client{Transport: &http.Transport{}},
	})
}

// probeS3 writes an object. fresh uses a new client, so exactly one new
// connection, which is what "fails the next connections" counts.
func probeS3(ctx context.Context, fresh bool) error {
	s3Once.Do(func() { s3Client = newS3Client() })
	c := s3Client
	if fresh {
		c = newS3Client()
	}
	_, err := c.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String("uploads"), Key: aws.String("network-probe.txt"), Body: bytes.NewReader([]byte("ok")),
	})
	return err
}

var (
	kafkaMu       sync.Mutex
	kafkaProducer sarama.SyncProducer
)

func probeKafka(ctx context.Context, _ bool) error {
	kafkaMu.Lock()
	defer kafkaMu.Unlock()
	if kafkaProducer == nil {
		cfg := sarama.NewConfig()
		cfg.Producer.Return.Successes = true
		cfg.Producer.Retry.Max = 0
		cfg.Net.DialTimeout, cfg.Net.ReadTimeout, cfg.Net.WriteTimeout = probeTimeout, probeTimeout, probeTimeout
		p, err := sarama.NewSyncProducer(strings.Split(os.Getenv("KAFKA_BROKERS"), ","), cfg)
		if err != nil {
			return err
		}
		kafkaProducer = p
	}
	_, _, err := kafkaProducer.SendMessage(&sarama.ProducerMessage{Topic: "network-probe", Value: sarama.StringEncoder("ok")})
	return err
}

// The RabbitMQ client keeps one connection and dials a new one when it
// drops, as amqp091 does not on its own.
var (
	amqpMu   sync.Mutex
	amqpConn *amqp.Connection
	amqpCh   *amqp.Channel
)

func probeRabbitMQ(ctx context.Context, _ bool) error {
	amqpMu.Lock()
	defer amqpMu.Unlock()
	if amqpConn == nil || amqpConn.IsClosed() {
		conn, err := amqp.DialConfig(os.Getenv("AMQP_URL"), amqp.Config{Dial: amqp.DefaultDial(probeTimeout)})
		if err != nil {
			return err
		}
		ch, err := conn.Channel()
		if err != nil {
			conn.Close()
			return err
		}
		amqpConn, amqpCh = conn, ch
	}
	if _, err := amqpCh.QueueDeclare("network-probe", false, true, false, false, nil); err != nil {
		amqpConn.Close()
		return err
	}
	if amqpConn.IsClosed() {
		return errors.New("connection closed")
	}
	return nil
}
