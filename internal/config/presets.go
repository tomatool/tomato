package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
	kafkaDirectPort        = "9094"  // plain, for tomato's own kafka resource when a network resource proxies the app's listeners
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
			if cont.Cluster || cont.TLS {
				return fmt.Errorf("container %q: cluster and tls need preset: %s", name, PresetRedis)
			}
			continue
		case PresetKafka:
			if err := c.expandKafkaPreset(name, &cont); err != nil {
				return fmt.Errorf("container %q: %w", name, err)
			}
		case PresetRedis:
			if err := c.expandRedisPreset(name, &cont); err != nil {
				return fmt.Errorf("container %q: %w", name, err)
			}
		case PresetChromium:
			if err := c.expandChromiumPreset(&cont); err != nil {
				return fmt.Errorf("container %q: %w", name, err)
			}
		case PresetVeraPDF:
			if err := expandVeraPDFPreset(&cont); err != nil {
				return fmt.Errorf("container %q: %w", name, err)
			}
		default:
			return fmt.Errorf("container %q: unknown preset %q (available: %s, %s, %s, %s)", name, cont.Preset, PresetChromium, PresetKafka, PresetRedis, PresetVeraPDF)
		}
		if cont.Preset != PresetRedis && (cont.Cluster || cont.TLS) {
			return fmt.Errorf("container %q: cluster and tls are options of preset: %s, not %s", name, PresetRedis, cont.Preset)
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

	if err := c.proxyKafkaListeners(name, &listeners, &advertised, &protocols, &ports); err != nil {
		return err
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

// proxyKafkaListeners puts a network resource's proxy in front of the
// broker's host listeners, when one stands in front of this container.
//
// A Kafka client dials the bootstrap address once and then the address the
// broker advertises, so a proxy only in front of the bootstrap port is
// bypassed after the first request. Instead the broker advertises the proxy's
// port, and the network resource listens there; and a listener of its own,
// advertised on its own port, keeps tomato's kafka resource off the proxy, so
// the steps that check a result never see the faults.
func (c *Config) proxyKafkaListeners(name string, listeners, advertised, protocols, ports *[]string) error {
	var networks []string
	for resName, res := range c.Resources {
		if res.Type == "network" && res.Container == name {
			networks = append(networks, resName)
		}
	}
	if len(networks) == 0 {
		return nil
	}
	sort.Strings(networks)
	if len(networks) > 1 {
		return fmt.Errorf("network resources %s all stand in front of it; one per container", strings.Join(networks, ", "))
	}

	listen := map[string]any{}
	for i, a := range *advertised {
		for _, l := range []struct{ name, port string }{{"EXTERNAL", KafkaPlainPort}, {"IAM", KafkaIAMPort}} {
			if !strings.HasPrefix(a, l.name+"://localhost:") {
				continue
			}
			proxyPort, err := freePort()
			if err != nil {
				return fmt.Errorf("picking a port for the network proxy: %w", err)
			}
			(*advertised)[i] = l.name + "://localhost:" + proxyPort
			n, _ := strconv.Atoi(proxyPort)
			listen[l.port+"/tcp"] = n
		}
	}

	directHostPort, err := freePort()
	if err != nil {
		return fmt.Errorf("picking a host port: %w", err)
	}
	*listeners = append(*listeners, "TOMATO://:"+kafkaDirectPort)
	*advertised = append(*advertised, "TOMATO://localhost:"+directHostPort)
	*protocols = append(*protocols, "TOMATO:PLAINTEXT")
	*ports = append(*ports, directHostPort+":"+kafkaDirectPort)

	network := c.Resources[networks[0]]
	if network.Options == nil {
		network.Options = map[string]any{}
	}
	network.Options["listen"] = listen
	c.Resources[networks[0]] = network

	for resName, res := range c.Resources {
		if res.Type == "kafka" && res.Container == name && len(res.Brokers) == 0 {
			res.Brokers = []string{"localhost:" + directHostPort}
			c.Resources[resName] = res
		}
	}
	return nil
}

// PresetRedis is a single Redis server; with cluster and tls it runs the way
// ElastiCache does with cluster mode and in-transit encryption, so an app
// keeps the client it uses in production.
const PresetRedis = "redis"

// RedisImage is what the redis preset runs unless the entry sets image or
// build. With cluster or tls the preset's entrypoint calls redis-server and
// redis-cli, so an override has to have both.
const RedisImage = "redis:7.4"

// RedisPort is where the redis preset listens inside the container, plain or
// TLS.
const RedisPort = "6379"

// redisReady is what the preset's entrypoint prints once the server takes
// commands and, in cluster mode, owns every slot.
const redisReady = "tomato: redis is ready"

// Where the redis preset's certificate and key go in the container.
const (
	redisCertPath = "/tomato/tls/server.pem"
	redisKeyPath  = "/tomato/tls/server.key"
	redisCAPath   = "/tomato/tls/ca.pem"
)

func (c *Config) expandRedisPreset(name string, cont *Container) error {
	if cont.Auth != "" {
		return fmt.Errorf("the redis preset has no auth (auth: %s)", cont.Auth)
	}
	if cont.Image == "" && cont.Build == nil {
		cont.Image = RedisImage
	}
	if (cont.Cluster || cont.TLS) && (c.App.Image != "" || c.App.Build != nil) {
		// A cluster node announces localhost:<host port>, and the CA is a
		// file on the host: neither reaches an app in a container.
		return fmt.Errorf("cluster and tls need the app to run as a command (app.command), not in a container")
	}
	if (cont.Cluster || cont.TLS) && c.Settings.Containers.Reuse {
		// A reused container keeps the CA and the announced port of the run
		// that started it; this run makes new ones.
		return fmt.Errorf("cluster and tls can't be used with settings.containers.reuse")
	}
	if cont.Cluster {
		for resName, res := range c.Resources {
			if res.Type == "network" && res.Container == name {
				// The client follows the address the node announces, which
				// would take it around the proxy after the first request.
				return fmt.Errorf("network resource %q can't stand in front of a redis cluster", resName)
			}
		}
	}

	port := RedisPort + "/tcp"
	if !cont.Cluster && !cont.TLS {
		if !containsPort(cont.Ports, RedisPort) {
			cont.Ports = append([]string{port}, cont.Ports...)
		}
		if cont.WaitFor.Type == "" {
			cont.WaitFor = WaitStrategy{Type: "log", Target: "Ready to accept connections", Timeout: time.Minute}
		}
		return nil
	}

	server := []string{"--appendonly", "no", "--save", "''"}
	cli := []string{}
	if cont.TLS {
		cert, err := newServerCert("localhost", "127.0.0.1", name)
		if err != nil {
			return fmt.Errorf("making a certificate: %w", err)
		}
		cont.CA, cont.CAFile = cert.CA, cert.CAFile
		cont.Files = append(cont.Files,
			ContainerFile{Path: redisCertPath, Content: cert.Cert, Mode: 0o644},
			ContainerFile{Path: redisKeyPath, Content: cert.Key, Mode: 0o644},
			ContainerFile{Path: redisCAPath, Content: cert.CA, Mode: 0o644},
		)
		// TLS only, as ElastiCache with in-transit encryption: a plain
		// client is refused, not served.
		server = append(server, "--port", "0", "--tls-port", RedisPort,
			"--tls-cert-file", redisCertPath, "--tls-key-file", redisKeyPath,
			"--tls-ca-cert-file", redisCAPath, "--tls-auth-clients", "no")
		cli = append(cli, "--tls", "--cacert", redisCAPath)
	}
	if cont.Cluster {
		// A node that has never met another reports its address in CLUSTER
		// SLOTS as an empty host and its port in the container, which a
		// client on the host can't dial. It announces the host port
		// instead, so the preset owns 6379's mapping: a host port the entry
		// fixes is the one announced, and otherwise one is picked here,
		// before the container starts. An entry leaving the port to Docker
		// would publish it somewhere the node doesn't announce.
		var hostPort string
		cont.Ports, hostPort = takePort(cont.Ports, RedisPort)
		if hostPort == "" {
			var err error
			if hostPort, err = freePort(); err != nil {
				return fmt.Errorf("picking a host port: %w", err)
			}
		}
		server = append(server, "--cluster-enabled", "yes",
			"--cluster-announce-ip", "127.0.0.1", "--cluster-announce-port", hostPort)
		if cont.TLS {
			server = append(server, "--cluster-announce-tls-port", hostPort)
		}
		port = hostPort + ":" + RedisPort
	}

	// The server starts in the background; once it answers, a cluster node
	// takes every slot (after a restart it already owns them, and the
	// command fails harmlessly), and the ready line tells tomato it can go.
	// If the server exits on the way, as with a bad argument, the container
	// exits with its status at once, its error last in the log, rather than
	// when the wait times out. Whatever the entry sets as command is added
	// to redis-server's arguments.
	redisCLI := "redis-cli " + strings.Join(cli, " ")
	script := "redis-server " + strings.Join(server, " ") + ` "$@" &
pid=$!
trap 'kill $pid' TERM INT
alive() { kill -0 $pid 2>/dev/null || { wait $pid; exit; }; }
until ` + redisCLI + ` ping >/dev/null 2>&1; do alive; sleep 0.1; done
`
	if cont.Cluster {
		script += redisCLI + ` cluster addslotsrange 0 16383 >/dev/null 2>&1
until ` + redisCLI + ` cluster info 2>/dev/null | grep -q cluster_state:ok; do alive; sleep 0.1; done
`
	}
	script += "echo '" + redisReady + "'\nwait $pid\n"
	cont.Entrypoint = []string{"sh", "-c", script, "redis"}

	if !containsPort(cont.Ports, RedisPort) {
		cont.Ports = append([]string{port}, cont.Ports...)
	}
	if cont.WaitFor.Type == "" {
		cont.WaitFor = WaitStrategy{Type: "log", Target: redisReady, Timeout: time.Minute}
	}

	// tomato's redis resources reach it the way the app does.
	for resName, res := range c.Resources {
		if res.Type != "redis" || res.Container != name {
			continue
		}
		if res.Options == nil {
			res.Options = map[string]any{}
		}
		if _, set := res.Options["cluster"]; !set && cont.Cluster {
			res.Options["cluster"] = true
		}
		if _, set := res.Options["tls_ca_file"]; !set && cont.TLS {
			res.Options["tls_ca_file"] = cont.CAFile
		}
		c.Resources[resName] = res
	}
	return nil
}

// PresetChromium is a headless Chromium for browser resources, serving the
// DevTools protocol on ChromiumDevToolsPort. Pages it opens on localhost reach
// the host tomato runs on, where the app runs.
const PresetChromium = "chromium"

// ChromiumImage is what the chromium preset runs unless the entry sets image or
// build. The preset's entrypoint relies on its layout, so an override has to be
// a chromedp/headless-shell image too.
const ChromiumImage = "chromedp/headless-shell:156.0.8078.36"

// ChromiumDevToolsPort is where the chromium preset serves the DevTools
// protocol. headless-shell only listens on loopback, so socat forwards this
// port to chromiumInternalPort.
const (
	ChromiumDevToolsPort = "9222"
	chromiumInternalPort = "9223"
)

// chromiumHostAlias is the name testcontainers gives the host inside a
// container with host access ports.
const chromiumHostAlias = "host.testcontainers.internal"

func (c *Config) expandChromiumPreset(cont *Container) error {
	if cont.Auth != "" {
		return fmt.Errorf("the chromium preset has no auth (auth: %s)", cont.Auth)
	}
	if cont.Image == "" && cont.Build == nil {
		cont.Image = ChromiumImage
	}

	flags := []string{
		"--no-sandbox",
		"--use-gl=angle",
		"--use-angle=swiftshader",
		// Docker gives a container 64MB of /dev/shm, which a large page
		// exhausts; Chromium then crashes the tab.
		"--disable-dev-shm-usage",
		"--remote-debugging-address=0.0.0.0",
		"--remote-debugging-port=" + chromiumInternalPort,
	}

	// The app runs on the host, and inside the container localhost is the
	// container. Rather than rewrite every URL to another host name, which
	// changes the page's origin (cookies, CORS, the redirects the app builds,
	// and the secure context only localhost gets over plain http), Chromium
	// resolves the name localhost to the host, through a tunnel to each port
	// the suite uses there.
	ports := c.hostPortsForBrowsers()
	if len(ports) > 0 {
		cont.HostAccessPorts = append(ports, cont.HostAccessPorts...)
		flags = append(flags, "--host-resolver-rules=MAP localhost "+chromiumHostAlias)
	}

	quoted := make([]string, len(flags))
	for i, f := range flags {
		quoted[i] = "'" + f + "'"
	}
	// headless-shell's own run.sh splits its arguments on spaces, which the
	// resolver rule has; this script passes them through intact. Whatever
	// the entry sets as command is added to Chromium's flags.
	script := "socat TCP4-LISTEN:" + ChromiumDevToolsPort + ",fork,reuseaddr TCP4:127.0.0.1:" + chromiumInternalPort + " &\n" +
		"exec /headless-shell/headless-shell " + strings.Join(quoted, " ") + ` "$@"`
	cont.Entrypoint = []string{"bash", "-c", script, "chromium"}

	devtools := ChromiumDevToolsPort + "/tcp"
	if !containsPort(cont.Ports, ChromiumDevToolsPort) {
		cont.Ports = append([]string{devtools}, cont.Ports...)
	}
	if cont.WaitFor.Type == "" {
		cont.WaitFor = WaitStrategy{Type: "http", Path: "/json/version", Target: devtools, Timeout: time.Minute}
	}
	return nil
}

// PresetVeraPDF is a veraPDF validation service, for the pdf resource's
// conformance step: POST a document to /api/validate/{profile} on
// VeraPDFPort and get the rule violations back.
const PresetVeraPDF = "verapdf"

// VeraPDFImage is what the verapdf preset runs unless the entry sets image or
// build: the veraPDF consortium's REST image, pinned. It is published for
// amd64 only; on an arm64 machine Docker runs it under emulation, which works
// and is slower.
const VeraPDFImage = "verapdf/rest:v1.30.3"

// VeraPDFPort is where the service answers inside the container.
const VeraPDFPort = "8080"

func expandVeraPDFPreset(cont *Container) error {
	if cont.Auth != "" {
		return fmt.Errorf("the verapdf preset has no auth (auth: %s)", cont.Auth)
	}
	if cont.Image == "" && cont.Build == nil {
		cont.Image = VeraPDFImage
	}
	port := VeraPDFPort + "/tcp"
	if !containsPort(cont.Ports, VeraPDFPort) {
		cont.Ports = append([]string{port}, cont.Ports...)
	}
	if cont.WaitFor.Type == "" {
		// The JVM takes a while to come up, longer under emulation.
		cont.WaitFor = WaitStrategy{Type: "http", Path: "/api/info", Target: port, Timeout: 3 * time.Minute}
	}
	return nil
}

// takePort removes the entries publishing the container port from ports, and
// returns the host port the first of them fixes ("16379:6379"), or "" when
// they leave it to Docker or there are none.
func takePort(ports []string, port string) (rest []string, hostPort string) {
	for _, p := range ports {
		if !containsPort([]string{p}, port) {
			rest = append(rest, p)
			continue
		}
		if i := strings.LastIndex(p, ":"); i >= 0 && hostPort == "" {
			hostPort = p[strings.LastIndex(p[:i], ":")+1 : i]
		}
	}
	return rest, hostPort
}

// containsPort reports whether ports already publish the container port.
func containsPort(ports []string, port string) bool {
	for _, p := range ports {
		if i := strings.LastIndex(p, ":"); i >= 0 {
			p = p[i+1:]
		}
		if strings.TrimSuffix(p, "/tcp") == port {
			return true
		}
	}
	return false
}

// hostPortsForBrowsers lists the ports on this machine a page might load from:
// the app's, and every localhost port the suite's resources name — a browser's
// base_url, the API an http resource calls, the mocks tomato serves. A port
// nothing listens on costs nothing; one missing from here is a page that does
// not load.
func (c *Config) hostPortsForBrowsers() []int {
	seen := map[int]bool{}
	add := func(p int) {
		if p > 0 && p < 65536 {
			seen[p] = true
		}
	}
	if c.App.IsConfigured() {
		add(c.App.Port)
	}
	for _, res := range c.Resources {
		switch res.Type {
		case "browser", "http", "http-client", "websocket", "websocket-client":
			for _, raw := range []string{res.BaseURL, res.URL, optionString(res.Options, "base_url")} {
				add(loopbackPort(raw))
			}
		case "http-server", "websocket-server":
			if p, err := strconv.Atoi(fmt.Sprint(res.Options["port"])); err == nil {
				add(p)
			}
		}
	}
	ports := make([]int, 0, len(seen))
	for p := range seen {
		ports = append(ports, p)
	}
	sort.Ints(ports)
	return ports
}

func optionString(options map[string]any, key string) string {
	s, _ := options[key].(string)
	return s
}

// loopbackPort is the port of a URL on this machine, or 0 for any other URL.
func loopbackPort(raw string) int {
	if raw == "" {
		return 0
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return 0
	}
	host := u.Hostname()
	if ip := net.ParseIP(host); host != "localhost" && (ip == nil || !(ip.IsLoopback() || ip.IsUnspecified())) {
		return 0
	}
	if p := u.Port(); p != "" {
		n, _ := strconv.Atoi(p)
		return n
	}
	switch u.Scheme {
	case "https", "wss":
		return 443
	case "http", "ws":
		return 80
	}
	return 0
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
