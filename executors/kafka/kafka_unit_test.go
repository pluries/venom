package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/IBM/sarama"
	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

const (
	testSchema = `{"type":"record","name":"R","fields":[{"name":"a","type":"string"}]}`
	testValue  = `{"a":"x"}`
)

func runKafka(t *testing.T, step venom.TestStep) (Result, error) {
	t.Helper()
	venom.InitTestLogger(t)
	got, err := Executor{}.Run(context.Background(), step)
	if err != nil {
		return Result{}, err
	}
	res, ok := got.(Result)
	require.True(t, ok)
	return res, nil
}

// hangUp listens on loopback and closes every connection at once: a broker that never answers.
func hangUp(t *testing.T) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { lis.Close() })
	go func() {
		for {
			c, err := lis.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	return lis.Addr().String()
}

func TestExecutorBasics(t *testing.T) {
	e := Executor{}
	assert.IsType(t, Result{}, e.ZeroValueResult())
	assert.Equal(t, []venom.Assertion{"result.err ShouldBeEmpty"}, e.GetDefaultAssertions().Assertions)
	assert.IsType(t, &Executor{}, New())
}

func TestRun_Validation(t *testing.T) {
	tests := map[string]struct {
		step    venom.TestStep
		wantErr string
	}{
		"undecodable":       {venom.TestStep{"addrs": 5}, "source data must be"},
		"waitfor > timeout": {venom.TestStep{"clientType": "consumer", "waitFor": 5, "timeout": 2}, "can't wait for messages 5s longer than the timeout 2s"},
		"unknown type":      {venom.TestStep{"clientType": "both"}, "type must be a consumer or a producer"},
		"bad registry":      {venom.TestStep{"withAVRO": true, "schemaRegistryAddr": "http://[::1"}, "can't create SchemaRegistry"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := runKafka(t, tc.step)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestRun_ResultErrors(t *testing.T) {
	tests := map[string]struct {
		step venom.TestStep
		want string
	}{
		"producer without messages": {venom.TestStep{"clientType": "producer"}, "either one of `messages` or `messagesFile`"},
		"consumer without topics":   {venom.TestStep{"clientType": "consumer"}, "you must provide topics"},
		"producer bad version": {
			venom.TestStep{"clientType": "producer", "kafka_version": "x.y", "addrs": []any{"127.0.0.1:1"}, "messages": []any{map[string]any{"topic": "t", "value": "v"}}},
			"error parsing Kafka version",
		},
		"consumer bad version": {
			venom.TestStep{"clientType": "consumer", "topics": []any{"t"}, "addrs": []any{"127.0.0.1:1"}, "kafka_version": "x.y"}, "error parsing Kafka version",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			res, err := runKafka(t, tc.step)
			require.NoError(t, err)
			assert.Contains(t, res.Err, tc.want)
		})
	}
}

func TestRun_BrokerHangsUp(t *testing.T) {
	addr := hangUp(t)
	msgs := []any{map[string]any{"topic": "t", "value": "v"}}
	prod, err := runKafka(t, venom.TestStep{"clientType": "producer", "addrs": []any{addr}, "messages": msgs})
	require.NoError(t, err)
	assert.NotEmpty(t, prod.Err)

	cons, err := runKafka(t, venom.TestStep{"clientType": "consumer", "addrs": []any{addr}, "topics": []any{"t"}, "initialOffset": "oldest"})
	require.NoError(t, err)
	assert.Contains(t, cons.Err, "error instantiate consumer")
}

func TestKafkaVersionAndSASL(t *testing.T) {
	cfg, err := Executor{KafkaVersion: "2.8.0", WithSASL: true, User: "u", Password: "p"}.getKafkaConfig(context.Background())
	require.NoError(t, err)
	assert.Equal(t, sarama.V2_8_0_0, cfg.Version)
	assert.True(t, cfg.Net.SASL.Enable)
	assert.Equal(t, "u", cfg.Net.SASL.User)
}

// Both spellings of the key reach the version (#3303): kafka_version is the one the README documents.
func TestKafkaVersionKeys(t *testing.T) {
	for name, step := range map[string]venom.TestStep{
		"kafka_version": {"kafka_version": "2.8.0"},
		"kafkaVersion":  {"kafkaVersion": "2.8.0"},
	} {
		var e Executor
		require.NoError(t, mapstructure.Decode(step, &e), name)
		cfg, err := e.getKafkaConfig(context.Background())
		require.NoError(t, err, name)
		assert.Equal(t, sarama.V2_8_0_0, cfg.Version, name)
	}
}

func TestReadMessagesFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "ok.json", []byte(`[{"topic":"t","key":"k","value":"v"}]`))
	writeFile(t, dir, "bad.json", []byte(`{`))
	got, err := readMessagesFile(dir + "/ok.json")
	require.NoError(t, err)
	assert.Equal(t, []Message{{Topic: "t", Key: "k", Value: "v"}}, got)
	_, err = readMessagesFile(dir + "/bad.json")
	require.Error(t, err)
	_, err = readMessagesFile(dir + "/missing.json")
	require.Error(t, err)
}

func TestConvertToRecordHeaders(t *testing.T) {
	assert.Empty(t, convertToRecordHeaders(nil))
	got := convertToRecordHeaders(map[string]string{"a": "1"})
	require.Len(t, got, 1)
	assert.Equal(t, []byte("a"), got[0].Key)
	assert.Equal(t, []byte("1"), got[0].Value)
}

func TestConvertFromMessage2JSON(t *testing.T) {
	tests := map[string]struct {
		msg      Message
		wantVal  any
		wantKeyT any
	}{
		"map value, string key": {Message{Key: "k", Value: `{"a":1}`}, map[string]interface{}{"a": json.Number("1")}, "k"},
		"text value, map key":   {Message{Key: `{"k":1}`, Value: "text"}, "text", map[string]interface{}{"k": json.Number("1")}},
		"list value, list key":  {Message{Key: `[{"Topic":"t"}]`, Value: `[{"Topic":"t"}]`}, []MessageJSON{{Topic: "t"}}, []MessageJSON{{Topic: "t"}}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var j MessageJSON
			convertFromMessage2JSON(&tc.msg, &j)
			assert.Equal(t, tc.wantVal, j.Value)
			assert.Equal(t, tc.wantKeyT, j.Key)
		})
	}
}

func TestAvroRoundTrip(t *testing.T) {
	bin, err := Convert2Avro([]byte(testValue), testSchema)
	require.NoError(t, err)
	text, err := ConvertFromAvro(bin, testSchema)
	require.NoError(t, err)
	assert.JSONEq(t, testValue, text)

	msg, err := CreateMessage(bin, 7)
	require.NoError(t, err)
	back, id := GetMessageAvroID(msg)
	assert.Equal(t, 7, id)
	assert.Equal(t, bin, back)
}

func TestAvroErrors(t *testing.T) {
	_, err := Convert2Avro([]byte(testValue), "not a schema")
	require.ErrorContains(t, err, "failed to create Avro schema")
	_, err = Convert2Avro([]byte(`{"a":1}`), testSchema)
	require.ErrorContains(t, err, "2 native Avro")
	_, err = ConvertFromAvro(nil, "not a schema")
	require.ErrorContains(t, err, "failed to create Avro schema")
	_, err = ConvertFromAvro(nil, testSchema)
	require.ErrorContains(t, err, "binary 2 native")
}

// fakeRegistry is a SchemaRegistry that answers from fields.
type fakeRegistry struct {
	id  int
	err error
}

func (f fakeRegistry) GetSchemaByID(int) (string, error)             { return testSchema, f.err }
func (f fakeRegistry) RegisterNewSchema(string, string) (int, error) { return f.id, f.err }
func (f fakeRegistry) GetLatestSchema(string) (int, string, error)   { return f.id, testSchema, f.err }

func TestGetMessageValue(t *testing.T) {
	venom.InitTestLogger(t)
	dir := t.TempDir()
	writeFile(t, dir, "schema.avsc", []byte(testSchema))
	writeFile(t, dir, "value.json", []byte(testValue))
	raw := Executor{}
	avro := Executor{WithAVRO: true, schemaReg: fakeRegistry{id: 3}}
	failing := Executor{WithAVRO: true, schemaReg: fakeRegistry{err: errors.New("down")}}

	v, err := raw.getMessageValue(&Message{ValueFile: "value.json"}, dir)
	require.NoError(t, err)
	assert.Equal(t, testValue, string(v))
	_, err = raw.getMessageValue(&Message{ValueFile: "nope.json"}, dir)
	require.ErrorContains(t, err, "can't get value")

	for _, m := range []Message{{Topic: "t", Value: testValue, AvroSchemaFile: "schema.avsc"}, {Topic: "t", Value: testValue}} {
		v, err = avro.getMessageValue(&m, dir)
		require.NoError(t, err)
		assert.Equal(t, byte(0), v[0])
	}

	_, err = avro.getMessageValue(&Message{Topic: "t", Value: testValue, AvroSchemaFile: "none.avsc"}, dir)
	require.ErrorContains(t, err, "can't read from")
	_, err = failing.getMessageValue(&Message{Topic: "t", Value: testValue, AvroSchemaFile: "schema.avsc"}, dir)
	require.ErrorContains(t, err, "can't register new schame")
	_, err = failing.getMessageValue(&Message{Topic: "t", Value: testValue}, dir)
	require.ErrorContains(t, err, "can't get latest schema")
	_, err = avro.getMessageValue(&Message{Topic: "t", Value: `{"a":1}`}, dir)
	require.ErrorContains(t, err, "can't convert value 2 avro")
	_, err = Executor{WithAVRO: true, schemaReg: fakeRegistry{id: -1}}.getMessageValue(&Message{Topic: "t", Value: testValue}, dir)
	require.ErrorContains(t, err, "can't encode avro message")
}

func TestProducerMessagesFromFile(t *testing.T) {
	venom.InitTestLogger(t)
	dir := t.TempDir()
	writeFile(t, dir, "m.json", []byte(`[{"topic":"t","key":"k","value":"v","headers":{"h":"1"}}]`))
	got, err := Executor{MessagesFile: "m.json"}.producerMessages(dir)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "t", got[0].Topic)
	_, err = Executor{MessagesFile: "missing.json"}.producerMessages(dir)
	require.Error(t, err)
}

// registryServer answers the three schema registry calls from fixed bodies; an unknown path is a 404.
func registryServer(t *testing.T) string {
	t.Helper()
	bodies := map[string]string{
		"/schemas/ids/5":                    `{"schema":"\"string\""}`,
		"/subjects/t-value/versions":        `{"id":9}`,
		"/subjects/t-value/versions/latest": `{"subject":"t-value","version":1,"id":4,"schema":"\"string\""}`,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := bodies[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"error_code":40401,"message":"not found"}`)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.schemaregistry.v1+json")
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestSchemaRegistryClient(t *testing.T) {
	reg, err := NewSchemaRegistry(registryServer(t))
	require.NoError(t, err)

	schema, err := reg.GetSchemaByID(5)
	require.NoError(t, err)
	assert.Equal(t, `"string"`, schema)
	id, err := reg.RegisterNewSchema("t-value", `"string"`)
	require.NoError(t, err)
	assert.Equal(t, 9, id)
	id, schema, err = reg.GetLatestSchema("t-value")
	require.NoError(t, err)
	assert.Equal(t, 4, id)
	assert.Equal(t, `"string"`, schema)

	_, err = reg.GetSchemaByID(6)
	require.ErrorContains(t, err, "could not get schema id 6")
	_, err = reg.RegisterNewSchema("none", "x")
	require.ErrorContains(t, err, "failed to register new schema")
	_, _, err = reg.GetLatestSchema("none")
	require.ErrorContains(t, err, "failed to get latest schema ID")
}

func TestNewSchemaRegistryBadURL(t *testing.T) {
	_, err := NewSchemaRegistry("http://[::1")
	require.ErrorContains(t, err, "failed to connect to schema registry")
}

func TestRun_WithAVROBuildsRegistry(t *testing.T) {
	res, err := runKafka(t, venom.TestStep{
		"withAVRO": true, "schemaRegistryAddr": registryServer(t), "clientType": "producer",
	})
	require.NoError(t, err)
	assert.Contains(t, res.Err, "either one of `messages`")
}
