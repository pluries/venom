package kafka

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/mitchellh/mapstructure"

	"github.com/ovh/venom"
	"github.com/ovh/venom/executors/internal/tlsconfig"
)

const (
	// Name of executor
	Name                          = "kafka"
	defaultExecutorTimeoutSeconds = 5
	defaultProducerMaxRetries     = 10
	defaultDialTimeout            = 10 * time.Second
)

// New returns a new Executor
func New() venom.Executor {
	return &Executor{}
}

type (
	// Message represents the object sended or received from kafka
	Message struct {
		Topic          string            `json:"topic" yaml:"topic"`
		Key            string            `json:"key" yaml:"key"`
		Headers        map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
		Value          string            `json:"value,omitempty" yaml:"value,omitempty"`
		ValueFile      string            `json:"valueFile,omitempty" yaml:"valueFile,omitempty"`
		AvroSchemaFile string            `json:"avroSchemaFile,omitempty" yaml:"avroSchemaFile,omitempty"`
	}

	// MessageJSON represents the object sended or received from kafka
	MessageJSON struct {
		Topic string
		Key   interface{}
		Value interface{}
	}

	// Executor represents a Test Exec
	Executor struct {
		Addrs []string `json:"addrs,omitempty" yaml:"addrs,omitempty"`
		// Registry schema address
		SchemaRegistryAddr string `json:"schema_registry_addr,omitempty" yaml:"schemaRegistryAddr,omitempty"`
		WithAVRO           bool   `json:"with_avro,omitempty" yaml:"withAVRO,omitempty"`
		WithTLS            bool   `json:"with_tls,omitempty" yaml:"withTLS,omitempty"`
		WithSASL           bool   `json:"with_sasl,omitempty" yaml:"withSASL,omitempty"`
		WithSASLHandshaked bool   `json:"with_sasl_handshaked,omitempty" yaml:"withSASLHandshaked,omitempty"`
		User               string `json:"user,omitempty" yaml:"user,omitempty"`
		Password           string `json:"password,omitempty" yaml:"password,omitempty"`

		// TLS Config
		InsecureTLS bool `json:"insecure_tls,omitempty" yaml:"insecure_tls,omitempty"`
		// WithTLSSnake and InsecureTLSSnake take the snake_case keys the README documents (with_tls, insecure_tls), which
		// the Go field names above do not reach; either spelling turns its switch on (#2370). They are decoded only: their
		// json and yaml names belong to WithTLS and InsecureTLS.
		WithTLSSnake     bool `json:"-" yaml:"-" mapstructure:"with_tls"`
		InsecureTLSSnake bool `json:"-" yaml:"-" mapstructure:"insecure_tls"`
		// TLSRootCA, TLSClientCert and TLSClientKey are inline PEM or a path relative to the suite (ADR-0009). They are
		// used only when TLS is on.
		TLSRootCA     string `json:"tls_root_ca" yaml:"tls_root_ca" mapstructure:"tls_root_ca"`
		TLSClientCert string `json:"tls_client_cert" yaml:"tls_client_cert" mapstructure:"tls_client_cert"`
		TLSClientKey  string `json:"tls_client_key" yaml:"tls_client_key" mapstructure:"tls_client_key"`

		// ClientType must be "consumer" or "producer"
		ClientType string `json:"client_type,omitempty" yaml:"clientType,omitempty"`

		// Used when ClientType is consumer
		GroupID string   `json:"group_id,omitempty" yaml:"groupID,omitempty"`
		Topics  []string `json:"topics,omitempty" yaml:"topics,omitempty"`
		// Represents the timeout for reading messages. In Seconds. Default 5
		Timeout int `json:"timeout,omitempty" yaml:"timeout,omitempty"`
		// WaitFor represents the time for reading messages without marking the test as failure.
		WaitFor int `json:"wait_for,omitempty" yaml:"waitFor,omitempty"`
		// Represents the limit of message will be read. After limit, consumer stop read message
		MessageLimit int `json:"message_limit,omitempty" yaml:"messageLimit,omitempty"`
		// InitialOffset represents the initial offset for the consumer. Possible value : newest, oldest. default: newest
		InitialOffset string `json:"initial_offset,omitempty" yaml:"initialOffset,omitempty"`
		// MarkOffset allows to mark offset when consuming message
		MarkOffset bool `json:"mark_offset,omitempty" yaml:"markOffset,omitempty"`

		// KeyFilter determines the key to filter from
		KeyFilter string `json:"key_filter,omitempty" yaml:"keyFilter,omitempty"`

		// Only one of JSON or Avro are currently supported
		ConsumerEncoding string `json:"consumer_encoding,omitempty" yaml:"consumerEncoding,omitempty"`

		// Used when ClientType is producer
		// Messages represents the message sended by producer
		Messages []Message `json:"messages,omitempty" yaml:"messages,omitempty"`

		// MessagesFile represents the messages into the file sended by producer (messages field would be ignored)
		MessagesFile string `json:"messages_file,omitempty" yaml:"messages_file,omitempty"`

		// Kafka version, default is 0.10.2.0
		KafkaVersion string `json:"kafka_version,omitempty" yaml:"kafka_version,omitempty"`
		// KafkaVersionSnake takes the snake_case key the README documents (kafka_version), which the Go field name above does
		// not reach; either spelling sets the version (#3303). It is decoded only: its json and yaml names belong to KafkaVersion.
		KafkaVersionSnake string `json:"-" yaml:"-" mapstructure:"kafka_version"`

		schemaReg SchemaRegistry
	}

	// Result represents a step result.
	Result struct {
		TimeSeconds  float64       `json:"timeseconds,omitempty" yaml:"timeSeconds,omitempty"`
		Messages     []Message     `json:"messages,omitempty" yaml:"messages,omitempty"`
		MessagesJSON []interface{} `json:"messagesjson,omitempty" yaml:"messagesJSON,omitempty"`
		Err          string        `json:"err" yaml:"error"`
	}
)

// ZeroValueResult return an empty implementation of this executor result
func (Executor) ZeroValueResult() interface{} {
	return Result{}
}

// GetDefaultAssertions return default assertions for type exec
func (Executor) GetDefaultAssertions() *venom.StepAssertions {
	return &venom.StepAssertions{Assertions: []venom.Assertion{"result.err ShouldBeEmpty"}}
}

// Run execute TestStep of type exec
func (Executor) Run(ctx context.Context, step venom.TestStep) (interface{}, error) {
	var e Executor
	if err := mapstructure.Decode(step, &e); err != nil {
		return nil, err
	}
	start := time.Now()

	result := Result{}
	if e.WithAVRO && len(e.SchemaRegistryAddr) != 0 {
		var err error
		e.schemaReg, err = NewSchemaRegistry(e.SchemaRegistryAddr)
		if err != nil {
			return nil, fmt.Errorf("can't create SchemaRegistry: %s", err)
		}
	}

	if e.Timeout == 0 {
		e.Timeout = defaultExecutorTimeoutSeconds
	}
	if e.WaitFor > 0 && e.Timeout < e.WaitFor {
		return nil, fmt.Errorf("can't wait for messages %ds longer than the timeout %ds", e.WaitFor, e.Timeout)
	}

	switch e.ClientType {
	case "producer":
		workdir := venom.StringVarFromCtx(ctx, "venom.testsuite.workdir")
		err := e.produceMessages(ctx, workdir)
		if err != nil {
			result.Err = err.Error()
		}
	case "consumer":
		var err error
		result.Messages, result.MessagesJSON, err = e.consumeMessages(ctx)
		if err != nil {
			result.Err = err.Error()
		}
	default:
		return nil, fmt.Errorf("type must be a consumer or a producer")
	}

	elapsed := time.Since(start)
	result.TimeSeconds = elapsed.Seconds()

	return result, nil
}

func (e Executor) produceMessages(ctx context.Context, workdir string) error {
	if len(e.Messages) == 0 && e.MessagesFile == "" {
		return fmt.Errorf("either one of `messages` or `messagesFile` field must be set")
	}

	config, err := e.getKafkaConfig(ctx)
	if err != nil {
		return err
	}

	config.Producer.RequiredAcks = sarama.WaitForLocal
	config.Producer.Retry.Max = defaultProducerMaxRetries
	config.Producer.Return.Successes = true

	sp, err := sarama.NewSyncProducer(e.Addrs, config)
	if err != nil {
		return err
	}
	defer sp.Close()

	messages, err := e.producerMessages(workdir)
	if err != nil {
		return err
	}
	return sp.SendMessages(messages)
}

// producerMessages returns the messages to send: those of messages_file (read under workdir) when it is set, else
// messages.
func (e Executor) producerMessages(workdir string) ([]*sarama.ProducerMessage, error) {
	if e.MessagesFile != "" {
		fileMessages, err := readMessagesFile(filepath.Join(workdir, e.MessagesFile))
		if err != nil {
			return nil, err
		}
		e.Messages = fileMessages
	}

	messages := []*sarama.ProducerMessage{}
	for i := range e.Messages {
		message := e.Messages[i]
		value, err := e.getMessageValue(&message, workdir)
		if err != nil {
			return nil, err
		}
		messages = append(messages, &sarama.ProducerMessage{
			Topic:   message.Topic,
			Key:     sarama.ByteEncoder([]byte(message.Key)),
			Headers: convertToRecordHeaders(message.Headers),
			Value:   sarama.ByteEncoder(value),
		})
	}
	return messages, nil
}

// readMessagesFile returns the JSON array of messages in the file at path.
func readMessagesFile(path string) ([]Message, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	messages := []Message{}
	if err := venom.JSONUnmarshal(content, &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

func (e Executor) getMessageValue(m *Message, workdir string) ([]byte, error) {
	value, err := e.getRAWMessageValue(m, workdir)
	if err != nil {
		return nil, fmt.Errorf("can't get value: %w", err)
	}
	if !e.WithAVRO {
		// This is test without AVRO - value is all we need to have
		return value, nil
	}
	// This is test with Avro
	var (
		schemaID int
		schema   string
	)
	// 1. Get schema with its ID
	// 1.1 Try with the file, if provided
	subject := fmt.Sprintf("%s-value", m.Topic) // Using topic name strategy
	schemaFile := strings.Trim(m.AvroSchemaFile, " ")
	if len(schemaFile) != 0 {
		schemaPath := path.Join(workdir, schemaFile)
		schemaBlob, err := os.ReadFile(schemaPath)
		if err != nil {
			return nil, fmt.Errorf("can't read from %s: %w", schemaPath, err)
		}
		schema = string(schemaBlob)
		// 1.2 Push schema to Schema Registry
		schemaID, err = e.schemaReg.RegisterNewSchema(subject, schema)
		if err != nil {
			return nil, fmt.Errorf("can't register new schame in SchemaRegistry: %s", err)
		}
	} else {
		// 1.3 Get schema from Schema Registry
		schemaID, schema, err = e.schemaReg.GetLatestSchema(subject)
		if err != nil {
			return nil, fmt.Errorf("can't get latest schema for subject %s-value: %w", m.Topic, err)
		}
	}

	// 2. Encode Value with schema
	avroMsg, err := Convert2Avro(value, schema)
	if err != nil {
		return nil, fmt.Errorf("can't convert value 2 avro with schema: %w", err)
	}
	// 3. Create Kafka message with magic byte and schema ID
	encodedAvroMsg, err := CreateMessage(avroMsg, schemaID)
	if err != nil {
		return nil, fmt.Errorf("can't encode avro message with schemaID: %s", err)
	}
	return encodedAvroMsg, nil
}

func (e Executor) getRAWMessageValue(m *Message, workdir string) ([]byte, error) {
	// We have 2 fields Value and ValueFile from where we can get value, we prefer Value
	if len(m.Value) != 0 {
		// Most easiest scenario - Value is present
		return []byte(m.Value), nil
	}
	// Read from file
	s := path.Join(workdir, m.ValueFile)
	value, err := os.ReadFile(s)
	if err != nil {
		return nil, fmt.Errorf("can't read from %s: %w", s, err)
	}
	return value, nil
}

func (e Executor) consumeMessages(ctx context.Context) ([]Message, []interface{}, error) {
	if len(e.Topics) == 0 {
		return nil, nil, fmt.Errorf("you must provide topics")
	}

	consumerGroup, err := e.newConsumerGroup(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = consumerGroup.Close() }()

	return e.consume(ctx, consumerGroup)
}

// consume reads from the group until the message limit, the timeout or an error, whichever comes first. The caller
// closes the group.
func (e Executor) consume(ctx context.Context, consumerGroup sarama.ConsumerGroup) ([]Message, []interface{}, error) {
	ctx, cancel := context.WithTimeout(ctx, e.consumeTimeout())
	defer cancel()

	// Track errors
	go e.logConsumeErrors(ctx, consumerGroup)

	h := &handler{
		withAVRO:     e.WithAVRO,
		messages:     []Message{},
		messagesJSON: []interface{}{},
		markOffset:   e.MarkOffset,
		messageLimit: e.MessageLimit,
		schemaReg:    e.schemaReg,
		keyFilter:    e.KeyFilter,
		done:         make(chan struct{}),
	}

	// Buffered: after the context ends, waitConsumed no longer receives and the goroutine must still be able to exit.
	cherr := make(chan error, 1)
	go func() {
		cherr <- consumerGroup.Consume(ctx, e.Topics, h)
	}()

	if err := e.waitConsumed(ctx, cherr); err != nil {
		return nil, nil, err
	}

	return h.messages, h.messagesJSON, nil
}

// newConsumerGroup builds the client configuration, starting from the oldest offset when asked.
func (e Executor) newConsumerGroup(ctx context.Context) (sarama.ConsumerGroup, error) {
	config, err := e.getKafkaConfig(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(e.InitialOffset) == "oldest" {
		config.Consumer.Offsets.Initial = sarama.OffsetOldest
	}

	consumerGroup, err := sarama.NewConsumerGroup(e.Addrs, e.GroupID, config)
	if err != nil {
		return nil, fmt.Errorf("error instantiate consumer err: %w", err)
	}
	return consumerGroup, nil
}

// consumeTimeout is WaitFor when set, Timeout otherwise.
func (e Executor) consumeTimeout() time.Duration {
	timeout := time.Duration(e.Timeout) * time.Second
	if e.WaitFor > 0 {
		timeout = time.Duration(e.WaitFor) * time.Second
	}
	return timeout
}

// logConsumeErrors logs the consumer group's errors until the channel closes; with WaitFor set, a
// deadline is the expected end of the wait and is not logged.
func (e Executor) logConsumeErrors(ctx context.Context, consumerGroup sarama.ConsumerGroup) {
	for err := range consumerGroup.Errors() {
		if e.WaitFor > 0 && errors.Is(err, context.DeadlineExceeded) {
			continue
		}

		venom.Error(ctx, "error on consume:%s", err)
	}
}

// waitConsumed waits for the consumer to return or the context to end. With WaitFor set, reaching the
// deadline ends the wait normally.
func (e Executor) waitConsumed(ctx context.Context, cherr <-chan error) error {
	select {
	case err := <-cherr:
		if err != nil {
			if e.WaitFor > 0 && errors.Is(err, context.DeadlineExceeded) {
				venom.Info(ctx, "wait ended")
			} else {
				return fmt.Errorf("error on consume: %w", err)
			}
		}
	case <-ctx.Done():
		if ctx.Err() != nil {
			if e.WaitFor > 0 && errors.Is(ctx.Err(), context.DeadlineExceeded) {
				venom.Info(ctx, "wait ended")
			} else {
				return fmt.Errorf("kafka consumed failed: %w", ctx.Err())
			}
		}
	}

	return nil
}

func (e Executor) getKafkaConfig(ctx context.Context) (*sarama.Config, error) {
	tlsConfig, err := e.tlsConfig(ctx)
	if err != nil {
		return nil, err
	}

	config := sarama.NewConfig()
	config.Net.TLS.Enable = e.tlsEnabled()
	config.Net.TLS.Config = tlsConfig
	config.Net.SASL.Enable = e.WithSASL
	config.Net.SASL.User = e.User
	config.Net.SASL.Password = e.Password
	config.Consumer.Return.Errors = true
	config.Net.DialTimeout = defaultDialTimeout
	config.Version = sarama.V2_6_0_0

	version := e.KafkaVersion
	if version == "" {
		version = e.KafkaVersionSnake
	}
	if version != "" {
		kafkaVersion, err := sarama.ParseKafkaVersion(version)
		if err != nil {
			return config, fmt.Errorf("error parsing Kafka version %v err: %w", kafkaVersion, err)
		}
		config.Version = kafkaVersion
	}

	return config, nil
}

// tlsEnabled reports whether withTLS or with_tls is on.
func (e Executor) tlsEnabled() bool {
	return e.WithTLS || e.WithTLSSnake
}

// warnf is venom.Warning; a test replaces it.
var warnf = venom.Warning

// tlsConfig returns the client TLS configuration, built by the shared tlsconfig package (ADR-0009). insecureTLS or
// insecure_tls skips the broker certificate check; the client certificate is still sent. tls_root_ca,
// tls_client_cert and tls_client_key are used only when TLS is on: otherwise they are not read, a warning says so,
// and the configuration is the one kafka always built. Half a client pair fails in every case.
func (e Executor) tlsConfig(ctx context.Context) (*tls.Config, error) {
	if err := tlsconfig.CheckClientPair(e.TLSClientCert, e.TLSClientKey); err != nil {
		return nil, err
	}
	settings := tlsconfig.Settings{InsecureSkipVerify: e.InsecureTLS || e.InsecureTLSSnake}
	switch {
	case e.tlsEnabled():
		settings.Workdir = venom.StringVarFromCtx(ctx, "venom.testsuite.workdir")
		settings.RootCA = e.TLSRootCA
		settings.ClientCert = e.TLSClientCert
		settings.ClientKey = e.TLSClientKey
	case e.TLSRootCA != "" || e.TLSClientCert != "":
		warnf(ctx, "kafka: tls_root_ca, tls_client_cert and tls_client_key are not used without withTLS: the connection is plaintext")
	}
	return tlsconfig.New(ctx, settings)
}

// handler represents a Sarama consumer group consumer
type handler struct {
	withAVRO     bool
	messages     []Message
	messagesJSON []interface{}
	markOffset   bool
	messageLimit int
	schemaReg    SchemaRegistry
	keyFilter    string
	mutex        sync.Mutex
	done         chan struct{}
	once         sync.Once
}

// Setup is run at the beginning of a new session, before ConsumeClaim
func (h *handler) Setup(_ sarama.ConsumerGroupSession) error {
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited
func (h *handler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim must start a consumer loop of ConsumerGroupClaim's Messages().
func (h *handler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	ctx := session.Context()

	for message := range claim.Messages() {
		// Stop consuming if one of the other handler goroutines already hit the message limit
		select {
		case <-h.done:
			return nil
		default:
		}
		consumeFunction := h.consumeJSON
		if h.withAVRO {
			consumeFunction = h.consumeAVRO
		}
		msg, msgJSON, err := consumeFunction(message)
		if err != nil {
			return err
		}
		// Pass filter
		if h.keyFilter != "" && msg.Key != h.keyFilter {
			venom.Info(ctx, "ignore message with key: %s", msg.Key)
			continue
		}

		h.mutex.Lock()
		// Check if message limit is hit *before* adding new message
		messagesLen := len(h.messages)
		if h.messageLimit > 0 && messagesLen >= h.messageLimit {
			h.mutex.Unlock()
			h.messageLimitReached(ctx)
			return nil
		}

		h.messages = append(h.messages, msg)
		h.messagesJSON = append(h.messagesJSON, msgJSON)
		h.mutex.Unlock()
		messagesLen++

		if h.markOffset {
			session.MarkMessage(message, "")
		}

		session.MarkMessage(message, "delivered")

		// Check if the message limit is hit
		if h.messageLimit > 0 && messagesLen >= h.messageLimit {
			h.messageLimitReached(ctx)
			return nil
		}
	}

	return nil
}

func (h *handler) messageLimitReached(ctx context.Context) {
	venom.Info(ctx, "message limit reached")
	// Signal to other handler goroutines that they should stop consuming messages.
	// Only checking the message length isn't enough in case of filtering by key and never reaching the check.
	// Using sync.Once to prevent panics from multiple channel closings.
	h.once.Do(func() { close(h.done) })
}

func (h *handler) consumeJSON(message *sarama.ConsumerMessage) (Message, interface{}, error) {
	msg := Message{
		Topic: message.Topic,
		Key:   string(message.Key),
		Value: string(message.Value),
	}
	msgJSON := MessageJSON{
		Topic: message.Topic,
	}
	convertFromMessage2JSON(&msg, &msgJSON)

	return msg, msgJSON, nil
}

func (h *handler) consumeAVRO(message *sarama.ConsumerMessage) (Message, interface{}, error) {
	msg := Message{
		Topic: message.Topic,
		Key:   string(message.Key),
	}
	msgJSON := MessageJSON{
		Topic: message.Topic,
	}
	// 1. Get Schema ID
	avroMsg, schemaID := GetMessageAvroID(message.Value)
	schema, err := h.schemaReg.GetSchemaByID(schemaID)
	if err != nil {
		return msg, nil, fmt.Errorf("can't get Schema with ID %d: %w", schemaID, err)
	}
	// 2. Decode Avro Msg
	value, err := ConvertFromAvro(avroMsg, schema)
	if err != nil {
		return msg, nil, fmt.Errorf("can't get value from Avro message: %w", err)
	}
	msg.Value = value
	convertFromMessage2JSON(&msg, &msgJSON)
	return msg, msgJSON, nil
}

func convertFromMessage2JSON(message *Message, msgJSON *MessageJSON) {
	// unmarshall the message.Value
	listMessageJSON := []MessageJSON{}
	// try to unmarshall into an array
	if err := venom.JSONUnmarshal([]byte(message.Value), &listMessageJSON); err != nil {
		// try to unmarshall into a map
		mapMessageJSON := map[string]interface{}{}
		if err2 := venom.JSONUnmarshal([]byte(message.Value), &mapMessageJSON); err2 != nil {
			// try to unmarshall into a string
			msgJSON.Value = message.Value
		} else {
			msgJSON.Value = mapMessageJSON
		}
	} else {
		msgJSON.Value = listMessageJSON
	}

	// unmarshall the message.Key
	listMessageJSON = []MessageJSON{}
	// try to unmarshall into an array
	if err := venom.JSONUnmarshal([]byte(message.Key), &listMessageJSON); err != nil {
		// try to unmarshall into a map
		mapMessageJSON := map[string]interface{}{}
		if err2 := venom.JSONUnmarshal([]byte(message.Key), &mapMessageJSON); err2 != nil {
			// try to unmarshall into a string
			msgJSON.Key = message.Key
		} else {
			msgJSON.Key = mapMessageJSON
		}
	} else {
		msgJSON.Key = listMessageJSON
	}
}

func convertToRecordHeaders(headers map[string]string) []sarama.RecordHeader {
	results := make([]sarama.RecordHeader, len(headers))
	idx := 0

	for k, v := range headers {
		results[idx] = sarama.RecordHeader{
			Key:   sarama.ByteEncoder(k),
			Value: sarama.ByteEncoder(v),
		}

		idx++
	}

	return results
}
