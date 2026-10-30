// Package yamljson reads and writes YAML through encoding/json: a YAML document becomes a generic
// object, then JSON, then the target value. venom's types carry json tags and UnmarshalJSON methods,
// so they work unchanged, and a YAML number or boolean read into a string field becomes that string.
//
// It is a port of the four functions of github.com/rockbears/yaml v0.4.0 that venom uses, on
// go.yaml.in/yaml/v3. See NOTICE.md.
package yamljson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strconv"

	"github.com/pkg/errors"
	"go.yaml.in/yaml/v3"
)

// Marshal converts the object to JSON, then the JSON to YAML.
func Marshal(o interface{}) ([]byte, error) {
	j, err := json.Marshal(o)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	y, err := JSONToYAML(j)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return y, nil
}

// Unmarshal converts the first YAML document of y to JSON, then decodes that JSON into o.
// An input without a document returns an error wrapping io.EOF.
func Unmarshal(y []byte, o interface{}) error {
	dec := yaml.NewDecoder(bytes.NewReader(y))
	vo := reflect.ValueOf(o)
	j, err := decodeToJSON(dec, &vo)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return err
		}
		return errors.Wrap(err, "error converting YAML to JSON")
	}

	if err := json.NewDecoder(bytes.NewReader(j)).Decode(&o); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

// JSONToYAML converts JSON to YAML, with a two-space indent and sorted keys.
func JSONToYAML(j []byte) ([]byte, error) {
	// yaml.Unmarshal rather than json.Unmarshal: it picks int, float or bool types where encoding/json
	// would turn every number into a float64, so number types survive the round trip.
	var jsonObj interface{}
	if err := yaml.Unmarshal(j, &jsonObj); err != nil {
		return nil, errors.WithStack(err)
	}

	var b bytes.Buffer
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(&jsonObj); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// YAMLToJSON converts YAML to JSON. JSON is a subset of YAML, so JSON input comes out unchanged in
// content. Integer, float and boolean map keys become strings; null and binary keys are an error.
func YAMLToJSON(y []byte) ([]byte, error) {
	return decodeToJSON(yaml.NewDecoder(bytes.NewReader(y)), nil)
}

func decodeToJSON(dec *yaml.Decoder, jsonTarget *reflect.Value) ([]byte, error) {
	var yamlObj interface{}
	if err := dec.Decode(&yamlObj); err != nil {
		return nil, errors.WithStack(err)
	}

	jsonObj, err := convertToJSONableObject(yamlObj, jsonTarget)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return json.Marshal(jsonObj)
}

// stringKey renders a YAML map key as a JSON object key.
func stringKey(k, v interface{}) (string, error) {
	switch typedKey := k.(type) {
	case string:
		return typedKey, nil
	case int:
		return strconv.Itoa(typedKey), nil
	case int64:
		return strconv.FormatInt(typedKey, 10), nil
	case float64:
		return strconv.FormatFloat(typedKey, 'g', -1, 64), nil
	case bool:
		return strconv.FormatBool(typedKey), nil
	default:
		return "", errors.WithStack(fmt.Errorf("unsupported map key of type: %s, key: %+#v, value: %+#v",
			reflect.TypeOf(k), k, v))
	}
}

// stringifyKeys turns a map[interface{}]interface{} (YAML v3 returns one when a key is not a string)
// into a map[string]interface{}.
func stringifyKeys(m map[interface{}]interface{}) (map[string]interface{}, error) {
	strMap := make(map[string]interface{}, len(m))
	for k, v := range m {
		ks, err := stringKey(k, v)
		if err != nil {
			return nil, err
		}
		strMap[ks] = v
	}
	return strMap, nil
}

// resolveTarget walks down jsonTarget to a concrete value. A target with a json.Unmarshaler or a
// encoding.TextUnmarshaler is not a plain string target, so it returns nil.
func resolveTarget(jsonTarget *reflect.Value) *reflect.Value {
	if jsonTarget == nil {
		return nil
	}
	ju, tu, pv := indirect(*jsonTarget, false)
	if ju != nil || tu != nil {
		return nil
	}
	return &pv
}

// memberTarget finds what the member k of a map or struct target is decoded into: the struct field the
// JSON library would pick (exact name first, else the first case-insensitive match), or a zero value of
// the map element type. It returns nil when the target says nothing about k.
func memberTarget(jsonTarget *reflect.Value, k string) *reflect.Value {
	if jsonTarget == nil {
		return nil
	}
	t := *jsonTarget
	switch t.Kind() {
	case reflect.Struct:
		f := findField(cachedTypeFields(t.Type()), []byte(k))
		if f == nil {
			return nil
		}
		jtf := t.Field(f.index[0])
		return &jtf
	case reflect.Map:
		jtv := reflect.Zero(t.Type().Elem())
		return &jtv
	}
	return nil
}

// findField returns the field named key, an exact match winning over the first case-insensitive one.
func findField(fields []field, key []byte) *field {
	var f *field
	for i := range fields {
		ff := &fields[i]
		if bytes.Equal(ff.nameBytes, key) {
			return ff
		}
		if f == nil && ff.equalFold(ff.nameBytes, key) {
			f = ff
		}
	}
	return f
}

// convertMap converts every value of m in place, each against the target of its key.
func convertMap(m map[string]interface{}, jsonTarget *reflect.Value) (interface{}, error) {
	for k, v := range m {
		converted, err := convertToJSONableObject(v, memberTarget(jsonTarget, k))
		if err != nil {
			return nil, errors.WithStack(err)
		}
		m[k] = converted
	}
	return m, nil
}

// convertSlice converts every element of s against the element type of a slice target.
func convertSlice(s []interface{}, jsonTarget *reflect.Value) (interface{}, error) {
	var elem *reflect.Value
	if jsonTarget != nil && jsonTarget.Kind() == reflect.Slice {
		// A slice points to nil by default, so build a value of the element type to stand for it.
		ev := reflect.Indirect(reflect.New(jsonTarget.Type().Elem()))
		elem = &ev
	}

	arr := make([]interface{}, len(s))
	for i, v := range s {
		converted, err := convertToJSONableObject(v, elem)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		arr[i] = converted
	}
	return arr, nil
}

// scalarToString renders a YAML number or boolean as a string; ok is false for anything else
// (and for an empty rendering, which is left as it was).
func scalarToString(v interface{}) (string, bool) {
	var s string
	switch typedVal := v.(type) {
	case int:
		s = strconv.FormatInt(int64(typedVal), 10)
	case int64:
		s = strconv.FormatInt(typedVal, 10)
	case float64:
		s = strconv.FormatFloat(typedVal, 'g', -1, 64)
	case uint64:
		s = strconv.FormatUint(typedVal, 10)
	case bool:
		s = strconv.FormatBool(typedVal)
	}
	return s, len(s) > 0
}

// convertToJSONableObject makes a decoded YAML value fit encoding/json: non-string map keys become
// strings, and a number or boolean headed for a string target becomes a string. jsonTarget is what
// the value will be decoded into, or nil when unknown.
func convertToJSONableObject(yamlObj interface{}, jsonTarget *reflect.Value) (interface{}, error) {
	jsonTarget = resolveTarget(jsonTarget)

	if m, ok := yamlObj.(map[interface{}]interface{}); ok {
		strMap, err := stringifyKeys(m)
		if err != nil {
			return nil, err
		}
		yamlObj = strMap
	}

	switch typed := yamlObj.(type) {
	case map[string]interface{}:
		return convertMap(typed, jsonTarget)
	case []interface{}:
		return convertSlice(typed, jsonTarget)
	}

	if jsonTarget != nil && jsonTarget.Kind() == reflect.String {
		if s, ok := scalarToString(yamlObj); ok {
			return s, nil
		}
	}
	return yamlObj, nil
}
