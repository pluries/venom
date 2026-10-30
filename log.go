package venom

import (
	"context"
	"encoding/base64"
	"fmt"
	"maps"
	"net/url"
	"sort"
	"strings"
	"testing"

	"github.com/gosimple/slug"
	"github.com/sirupsen/logrus"
)

func InitTestLogger(_ *testing.T) {
	l := logrus.New()
	logger = logrus.NewEntry(l)
}

var (
	logger *logrus.Entry
	fields = []string{"testsuite", "testcase", "step", "executor"}
)

// fieldsFromContext returns the log fields keys read from ctx, each cleaned of the context's secrets: a test case name
// may hold one. A field without a secret keeps its value and type.
func fieldsFromContext(ctx context.Context, keys ...string) logrus.Fields {
	fields := logrus.Fields{}
	if ctx == nil {
		return fields
	}
	secrets := contextSecrets(ctx)
	for _, k := range keys {
		ck := ContextKey(k)
		i := ctx.Value(ck)
		if i == nil {
			continue
		}
		if len(secrets) > 0 {
			i = redactLogArg(secrets, i)
		}
		fields[k] = i
	}
	return fields
}

// HideSensitive replace the value with __hidden__
func HideSensitive(ctx context.Context, arg interface{}) string {
	secrets, ok := ctx.Value(ContextKey("secrets")).([]string)
	if !ok || len(secrets) == 0 {
		if str, ok := arg.(string); ok {
			return str
		}
		return fmt.Sprint(arg)
	}

	return replaceSecrets(fmt.Sprint(arg), secrets)
}

func replaceSecrets(s string, secrets []string) string {
	sorted := append([]string(nil), secrets...)
	sort.Slice(sorted, func(i, j int) bool { return len(sorted[i]) > len(sorted[j]) })
	for _, secret := range sorted {
		if secret == "" {
			continue
		}
		s = strings.ReplaceAll(s, secret, "__hidden__")
	}
	return s
}

func secretKeySet(secretKeys []string) map[string]struct{} {
	set := make(map[string]struct{}, len(secretKeys))
	for _, k := range secretKeys {
		set[k] = struct{}{}
	}
	return set
}

// contextSecrets returns the secret values ctx holds, nil when it holds none.
func contextSecrets(ctx context.Context) []string {
	if ctx == nil {
		return nil
	}
	if secrets, ok := ctx.Value(ContextKey("secrets")).([]string); ok {
		return secrets
	}
	return nil
}

// redactMapVars hides the secrets of a report's variables in place. A variable named in secretKeys is replaced whole;
// a venom.* value is cleaned of secrets and replaced only when one was found, so its type is kept (numbers stay
// numbers); any other value is cleaned and becomes a string. Keys are cleaned too: a key holding a secret, such as a
// computed variable prefixed with a test case name, is renamed.
func redactMapVars(ctx context.Context, vars H, secretKeys []string) {
	if len(vars) == 0 || len(secretKeys) == 0 {
		return
	}
	secrets := contextSecrets(ctx)
	secretSet := secretKeySet(secretKeys)
	redactKeys(vars, secrets, func(k string, val interface{}) interface{} {
		if strings.HasPrefix(k, "venom.") {
			return redactLogArg(secrets, val)
		}
		if _, ok := secretSet[k]; ok {
			return "__hidden__"
		}
		return HideSensitive(ctx, val)
	})
}

// redactStringMap is redactMapVars for string values.
func redactStringMap(ctx context.Context, vars map[string]string, secretKeys []string) {
	if len(vars) == 0 || len(secretKeys) == 0 {
		return
	}
	secretSet := secretKeySet(secretKeys)
	redactKeys(vars, contextSecrets(ctx), func(k, val string) string {
		if _, ok := secretSet[k]; ok && !strings.HasPrefix(k, "venom.") {
			return "__hidden__"
		}
		return HideSensitive(ctx, val)
	})
}

// redactKeys rewrites vars in place: every key cleaned of secrets, every value passed through value with its original
// key. Keys are walked in sorted order, so when a cleaned key meets another key, the one that sorts last wins whatever
// the map iteration order.
func redactKeys[V any](vars map[string]V, secrets []string, value func(k string, val V) V) {
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make(map[string]V, len(vars))
	for _, k := range keys {
		out[replaceSecrets(k, secrets)] = value(k, vars[k])
	}
	clear(vars)
	maps.Copy(vars, out)
}

func redactLogArgs(ctx context.Context, args ...interface{}) []interface{} {
	if ctx == nil {
		return args
	}
	secrets, ok := ctx.Value(ContextKey("secrets")).([]string)
	if !ok || len(secrets) == 0 {
		return args
	}
	if len(args) == 0 {
		return args
	}
	redacted := make([]interface{}, len(args))
	for i, arg := range args {
		redacted[i] = redactLogArg(secrets, arg)
	}
	return redacted
}

// redactLogArg redacts a single log argument. It only replaces the argument
// with a string when a secret was actually found, so non-string arguments keep
// their original type and format verbs like %d, %f and %t still work.
func redactLogArg(secrets []string, arg interface{}) interface{} {
	if arg == nil {
		return nil
	}
	original := fmt.Sprint(arg)
	cleaned := replaceSecrets(original, secrets)
	if cleaned == original {
		return arg
	}
	return cleaned
}

// hideSensitiveBytes redacts secrets in byte-oriented step output while preserving []byte type for JSON encoding.
func hideSensitiveBytes(ctx context.Context, data interface{}) []byte {
	if data == nil {
		return nil
	}
	switch v := data.(type) {
	case []byte:
		return []byte(HideSensitive(ctx, string(v)))
	case string:
		return []byte(HideSensitive(ctx, v))
	default:
		return []byte(HideSensitive(ctx, fmt.Sprint(v)))
	}
}

// minSlugSecret is the shortest slug form of a secret appendDerivedSecrets registers.
const minSlugSecret = 4

func appendDerivedSecrets(secrets []string, seen map[string]struct{}, vars H, secretKeys []string) []string {
	add := func(value string) {
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		secrets = append(secrets, value)
	}

	for _, key := range secretKeys {
		for _, form := range derivedForms(fmt.Sprint(vars[key])) {
			add(form)
		}
	}

	if _, ok := secretKeySet(secretKeys)["basic_auth_password"]; ok {
		pass := fmt.Sprint(vars["basic_auth_password"])
		if pass != "" && pass != "<nil>" {
			add(base64.StdEncoding.EncodeToString([]byte(fmt.Sprint(vars["basic_auth_user"]) + ":" + pass)))
		}
	}

	return secrets
}

// derivedForms returns the forms other than the value itself in which a secret value can reach a report or a line:
// base64, the slug and the two URL-encoded forms. A form shorter than minSlugSecret is left out where it would blank
// common fragments of every text (the slug and URL forms), and a form equal to the value is not repeated.
func derivedForms(val string) []string {
	if val == "" || val == "<nil>" {
		return nil
	}
	forms := []string{base64.StdEncoding.EncodeToString([]byte(val))}
	// Test case names are turned into slugs when the suite is parsed (parseTestCases), so a secret holding
	// characters the slug changes would only appear there in its slug form (#2132).
	// A secret sent in an URL travels percent-encoded, in the query form ("+" for a space) or the path form
	// ("%20") (#2302).
	for _, form := range []string{slug.Make(val), url.QueryEscape(val), url.PathEscape(val)} {
		if len(form) >= minSlugSecret {
			forms = append(forms, form)
		}
	}
	return forms
}

func Debug(ctx context.Context, format string, args ...interface{}) {
	fields := fieldsFromContext(ctx, fields...)
	logger.WithFields(fields).Debugf(format, redactLogArgs(ctx, args...)...)
}

func Info(ctx context.Context, format string, args ...interface{}) {
	fields := fieldsFromContext(ctx, fields...)
	logger.WithFields(fields).Infof(format, redactLogArgs(ctx, args...)...)
}

func Warn(ctx context.Context, format string, args ...interface{}) {
	fields := fieldsFromContext(ctx, fields...)
	logger.WithFields(fields).Warnf(format, redactLogArgs(ctx, args...)...)
}

func Warning(ctx context.Context, format string, args ...interface{}) {
	fields := fieldsFromContext(ctx, fields...)
	logger.WithFields(fields).Warningf(format, redactLogArgs(ctx, args...)...)
}

func Error(ctx context.Context, format string, args ...interface{}) {
	fields := fieldsFromContext(ctx, fields...)
	logger.WithFields(fields).Errorf(format, redactLogArgs(ctx, args...)...)
}

func Fatal(ctx context.Context, format string, args ...interface{}) {
	fields := fieldsFromContext(ctx, fields...)
	logger.WithFields(fields).Fatalf(format, redactLogArgs(ctx, args...)...)
}
