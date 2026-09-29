package config

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tomatool/tomato/internal/awsid"
	"github.com/tomatool/tomato/internal/presets"
)

// A preset expands one container entry into a working configuration for a
// service tomato knows how to run, so a tomato.yml does not spell out images,
// listeners and ports. The expansion only fills what the entry leaves empty; env
// keys the entry sets win.

const (
	// PresetKafka is a single-node Kafka (KRaft) with a plain listener and,
	// with auth: aws_msk_iam, a listener that behaves like MSK's IAM port.
	PresetKafka = "kafka"

	// KafkaAuthPlaintext is the default: plain listeners only.
	KafkaAuthPlaintext = "plaintext"
	// KafkaAuthAWSMSKIAM adds a listener speaking SASL AWS_MSK_IAM, the
	// mechanism aws-msk-iam-auth uses against MSK. When the config has an aws
	// resource, only the role sessions it issues get in, and any other identity
	// is told "Access denied", as MSK does.
	KafkaAuthAWSMSKIAM = "aws_msk_iam"
)

// KafkaImage is what the kafka preset runs unless the entry sets image or
// build: the stock broker image. For auth: aws_msk_iam, tomato copies the
// AWS_MSK_IAM server it carries (presets.MskIamJar) into kafkaPluginPath, on the
// broker's classpath, so any apache/kafka image works.
const KafkaImage = "apache/kafka:3.9.1"

const kafkaPluginPath = "/opt/kafka/libs/tomato-msk-iam.jar"

// Kafka preset ports inside the container. The two host-facing listeners are
// published on host ports tomato picks, and advertised as localhost:<that port>,
// so clients on the host (tomato, an app in command mode) reach them with
// {{.<name>.host}}:{{.<name>.port.9092}} and {{.<name>.port.9098}}.
const (
	KafkaPlainPort = "9092" // plain, for the host
	KafkaIAMPort   = "9098" // AWS_MSK_IAM, for the host (auth: aws_msk_iam)

	kafkaInternalPlainPort = "29092" // plain, for containers on tomato's network: <name>:29092
	kafkaInternalIAMPort   = "29098" // AWS_MSK_IAM, for containers on tomato's network: <name>:29098
	kafkaControllerPort    = "9093"
)

// freePort asks the OS for a port nothing listens on, for a fixed host port.
// Kafka has to advertise its host port before the container starts, so the port
// cannot be left for Docker to pick.
var freePort = func() (string, error) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return "", err
	}
	defer l.Close()
	return fmt.Sprintf("%d", l.Addr().(*net.TCPAddr).Port), nil
}

func (c *Config) expandPresets() error {
	names := make([]string, 0, len(c.Containers))
	for name := range c.Containers {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		cont := c.Containers[name]
		switch cont.Preset {
		case "":
			if cont.Auth != "" {
				return fmt.Errorf("container %q: auth needs a preset (auth: %s)", name, cont.Auth)
			}
			continue
		case PresetKafka:
			if err := c.expandKafkaPreset(name, &cont); err != nil {
				return fmt.Errorf("container %q: %w", name, err)
			}
		default:
			return fmt.Errorf("container %q: unknown preset %q (available: %s)", name, cont.Preset, PresetKafka)
		}
		c.Containers[name] = cont
	}
	return nil
}

func (c *Config) expandKafkaPreset(name string, cont *Container) error {
	auth := cont.Auth
	if auth == "" {
		auth = KafkaAuthPlaintext
	}
	if auth != KafkaAuthPlaintext && auth != KafkaAuthAWSMSKIAM {
		return fmt.Errorf("unknown kafka auth %q (available: %s, %s)", cont.Auth, KafkaAuthPlaintext, KafkaAuthAWSMSKIAM)
	}
	iam := auth == KafkaAuthAWSMSKIAM

	if cont.Image == "" && cont.Build == nil {
		cont.Image = KafkaImage
	}

	plainHostPort, err := freePort()
	if err != nil {
		return fmt.Errorf("picking a host port: %w", err)
	}
	listeners := []string{
		"EXTERNAL://:" + KafkaPlainPort,
		"INTERNAL://:" + kafkaInternalPlainPort,
		"CONTROLLER://:" + kafkaControllerPort,
	}
	advertised := []string{
		"EXTERNAL://localhost:" + plainHostPort,
		"INTERNAL://" + name + ":" + kafkaInternalPlainPort,
	}
	protocols := []string{"EXTERNAL:PLAINTEXT", "INTERNAL:PLAINTEXT", "CONTROLLER:PLAINTEXT"}
	ports := []string{plainHostPort + ":" + KafkaPlainPort}

	env := map[string]string{
		"KAFKA_NODE_ID":                                  "1",
		"KAFKA_PROCESS_ROLES":                            "broker,controller",
		"KAFKA_CONTROLLER_QUORUM_VOTERS":                 "1@localhost:" + kafkaControllerPort,
		"KAFKA_CONTROLLER_LISTENER_NAMES":                "CONTROLLER",
		"KAFKA_INTER_BROKER_LISTENER_NAME":               "INTERNAL",
		"KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR":         "1",
		"KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR": "1",
		"KAFKA_TRANSACTION_STATE_LOG_MIN_ISR":            "1",
		"KAFKA_GROUP_INITIAL_REBALANCE_DELAY_MS":         "0",
		"KAFKA_AUTO_CREATE_TOPICS_ENABLE":                "true",
	}

	if iam {
		iamHostPort, err := freePort()
		if err != nil {
			return fmt.Errorf("picking a host port: %w", err)
		}
		listeners = append(listeners, "IAM://:"+KafkaIAMPort, "IAM_INTERNAL://:"+kafkaInternalIAMPort)
		advertised = append(advertised, "IAM://localhost:"+iamHostPort, "IAM_INTERNAL://"+name+":"+kafkaInternalIAMPort)
		protocols = append(protocols, "IAM:SASL_PLAINTEXT", "IAM_INTERNAL:SASL_PLAINTEXT")
		ports = append(ports, iamHostPort+":"+KafkaIAMPort)

		const loginModule = "io.github.tomatool.mskiam.MskIamLoginModule required;"
		for _, listener := range []string{"IAM", "IAM_INTERNAL"} {
			// listener.name.<listener>.sasl.enabled.mechanisms and
			// listener.name.<listener>.aws_msk_iam.sasl.jaas.config, in the
			// image's env-to-property naming (_ is ., __ is _), which the
			// listener's own name has to follow too.
			envName := strings.ReplaceAll(listener, "_", "__")
			env["KAFKA_LISTENER_NAME_"+envName+"_SASL_ENABLED_MECHANISMS"] = "AWS_MSK_IAM"
			env["KAFKA_LISTENER_NAME_"+envName+"_AWS__MSK__IAM_SASL_JAAS_CONFIG"] = loginModule
		}
		env["TOMATO_MSK_IAM_ALLOWED_ACCESS_KEY_IDS"] = strings.Join(c.awsRoleSessionKeys(), ",")
		cont.Files = append(cont.Files, ContainerFile{Path: kafkaPluginPath, Content: presets.MskIamJar, Mode: 0o644})
	}

	env["KAFKA_LISTENERS"] = strings.Join(listeners, ",")
	env["KAFKA_ADVERTISED_LISTENERS"] = strings.Join(advertised, ",")
	env["KAFKA_LISTENER_SECURITY_PROTOCOL_MAP"] = strings.Join(protocols, ",")

	for k, v := range cont.Env {
		env[k] = v
	}
	cont.Env = env
	cont.Ports = append(ports, cont.Ports...)

	if cont.WaitFor.Type == "" {
		cont.WaitFor = WaitStrategy{Type: "log", Target: "Kafka Server started", Timeout: 2 * time.Minute}
	}
	return nil
}

// awsRoleSessionKeys lists the access key ids of the role sessions the config's
// aws resources issue, which is who an AWS_MSK_IAM listener lets in.
func (c *Config) awsRoleSessionKeys() []string {
	var keys []string
	for _, res := range c.Resources {
		if res.Type != "aws" {
			continue
		}
		role, _ := res.Options["role_arn"].(string)
		if role == "" {
			role = awsid.DefaultRoleARN
		}
		keys = append(keys, awsid.Session(role).AccessKeyID)
	}
	sort.Strings(keys)
	return keys
}

// resolvePaths makes the host paths a container names absolute, relative to the
// directory of the config file, so they mean the same wherever tomato runs from.
func (c *Config) resolvePaths(dir string) {
	for name, cont := range c.Containers {
		if cont.Build != nil && cont.Build.Context != "" && !filepath.IsAbs(cont.Build.Context) {
			cont.Build.Context = filepath.Join(dir, cont.Build.Context)
		}
		for i, v := range cont.Volumes {
			cont.Volumes[i] = resolveVolume(dir, v)
		}
		c.Containers[name] = cont
	}
}

// resolveVolume makes the source of a "source:target[:mode]" bind absolute. A
// source without a slash is a named volume and stays as it is.
func resolveVolume(dir, spec string) string {
	source, rest, ok := strings.Cut(spec, ":")
	if !ok || source == "" {
		return spec
	}
	switch {
	case strings.HasPrefix(source, "~/"):
		if home, err := os.UserHomeDir(); err == nil {
			source = filepath.Join(home, source[2:])
		}
	case filepath.IsAbs(source):
	case source == "." || strings.HasPrefix(source, "./") || strings.HasPrefix(source, "../") || strings.Contains(source, "/"):
		source = filepath.Join(dir, source)
	default:
		return spec
	}
	return source + ":" + rest
}
