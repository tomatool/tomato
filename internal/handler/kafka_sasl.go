package handler

import (
	"fmt"
	"strings"

	"github.com/IBM/sarama"
	"github.com/xdg-go/scram"
)

// applyKafkaSecurity configures TLS and SASL from options.tls and
// options.sasl, for brokers tomato did not start (a shared cluster, a cloud
// broker):
//
//	sasl:
//	  mechanism: SCRAM-SHA-512   # PLAIN, SCRAM-SHA-256 or SCRAM-SHA-512
//	  user: ${KAFKA_USER}
//	  password: ${KAFKA_PASSWORD}
func applyKafkaSecurity(cfg *sarama.Config, opts map[string]any) error {
	tlsCfg, err := tlsFromOptions(opts)
	if err != nil {
		return err
	}
	if tlsCfg != nil {
		cfg.Net.TLS.Enable = true
		cfg.Net.TLS.Config = tlsCfg
	}

	raw, ok := opts["sasl"]
	if !ok || raw == nil {
		return nil
	}
	sasl, ok := raw.(map[string]any)
	if !ok {
		return fmt.Errorf("options.sasl must be a map with mechanism, user and password")
	}
	user, _ := sasl["user"].(string)
	password, _ := sasl["password"].(string)
	if user == "" {
		return fmt.Errorf("options.sasl.user is required")
	}
	mechanism, _ := sasl["mechanism"].(string)
	if mechanism == "" {
		mechanism = sarama.SASLTypePlaintext
	}

	cfg.Net.SASL.Enable = true
	cfg.Net.SASL.User = user
	cfg.Net.SASL.Password = password
	switch strings.ToUpper(mechanism) {
	case sarama.SASLTypePlaintext:
		cfg.Net.SASL.Mechanism = sarama.SASLTypePlaintext
	case sarama.SASLTypeSCRAMSHA256:
		cfg.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA256
		cfg.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient {
			return &scramClient{hash: scram.SHA256}
		}
	case sarama.SASLTypeSCRAMSHA512:
		cfg.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
		cfg.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient {
			return &scramClient{hash: scram.SHA512}
		}
	default:
		return fmt.Errorf("options.sasl.mechanism %q is not supported (use PLAIN, SCRAM-SHA-256 or SCRAM-SHA-512)", mechanism)
	}
	return nil
}

// scramClient adapts xdg-go/scram to sarama's SCRAMClient interface.
type scramClient struct {
	hash         scram.HashGeneratorFcn
	conversation *scram.ClientConversation
}

func (c *scramClient) Begin(user, password, authzID string) error {
	client, err := c.hash.NewClient(user, password, authzID)
	if err != nil {
		return err
	}
	c.conversation = client.NewConversation()
	return nil
}

func (c *scramClient) Step(challenge string) (string, error) {
	return c.conversation.Step(challenge)
}

func (c *scramClient) Done() bool {
	return c.conversation.Done()
}
