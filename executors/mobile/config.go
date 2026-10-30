package mobile

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/ovh/venom"
)

const (
	platformAndroid = "android"
	platformIOS     = "ios"

	// capabilityPrefix is the vendor prefix of the Appium capabilities.
	capabilityPrefix = "appium:"

	// platformNameKey is the one capability without the vendor prefix.
	platformNameKey = "platformName"

	// deviceVar is the suite variable that sets both deviceName and udid.
	deviceVar = "mobile.device"

	// appIDVar is the suite variable that sets the package (Android) or the bundle id (iOS).
	appIDVar = "mobile.appId"

	defaultIdleTimeout = 120 // seconds, FR-18
	defaultSyncTimeout = 10  // seconds, FR-08
)

// ownedCapabilities are the capabilities the executor sets itself, with what sets them (FR-06). A user capability
// of the same name is refused, so that the test cannot run elsewhere than the report says.
var ownedCapabilities = map[string]string{
	platformNameKey:     "mobile.platform",
	"deviceName":        deviceVar,
	"udid":              deviceVar,
	"platformVersion":   "mobile.osVersion",
	"app":               "mobile.app",
	"appPackage":        appIDVar,
	"bundleId":          appIDVar,
	"automationName":    "the executor",
	"newCommandTimeout": "mobile.idleTimeout",
	// the alert step answers alerts, so a session that answers them by itself would hide them from the test
	"autoAcceptAlerts":  "the alert action",
	"autoDismissAlerts": "the alert action",
}

// setupConfig is what Setup reads from the suite variables (mobile.*).
type setupConfig struct {
	url            string
	platform       string
	device         string
	osVersion      string
	app            string
	appPath        string // absolute path of app, set by resolveApp
	appBundle      bool   // app is a .app folder (iOS): zipped before a copy through SSH, never hashed
	appSHA256      string
	appID          string
	capabilities   map[string]interface{}
	syncTimeout    time.Duration
	sessionTimeout time.Duration
	idleTimeout    int          // seconds, as the server takes it
	ssh            *sshSettings // nil without mobile.ssh.host: the server is reached directly
}

// readConfig reads and checks the settings; every refusal happens here, before any request.
func readConfig(ctx context.Context) (setupConfig, error) {
	cfg := setupConfig{
		url:          strings.TrimRight(venom.StringVarFromCtx(ctx, "mobile.url"), "/"),
		platform:     strings.ToLower(venom.StringVarFromCtx(ctx, "mobile.platform")),
		device:       venom.StringVarFromCtx(ctx, deviceVar),
		osVersion:    venom.StringVarFromCtx(ctx, "mobile.osVersion"),
		app:          venom.StringVarFromCtx(ctx, "mobile.app"),
		appSHA256:    venom.StringVarFromCtx(ctx, "mobile.appSHA256"),
		appID:        venom.StringVarFromCtx(ctx, appIDVar),
		capabilities: normaliseMap(venom.StringMapInterfaceVarFromCtx(ctx, "mobile.capabilities")),
	}
	if err := cfg.checkTarget(); err != nil {
		return cfg, err
	}
	var err error
	if cfg.ssh, err = readSSH(ctx); err != nil {
		return cfg, err
	}
	cfg.warnPlainHTTP(ctx)
	if err := resolveApp(venom.StringVarFromCtx(ctx, "venom.testsuite.workdir"), &cfg); err != nil {
		return cfg, err
	}
	if err := cfg.readLimits(ctx); err != nil {
		return cfg, err
	}
	return cfg, checkCapabilities(cfg.capabilities)
}

// checkTarget validates the server address, the platform and the device.
func (c setupConfig) checkTarget() error {
	if c.url == "" {
		return usagef("mobile.url is not set")
	}
	u, err := url.Parse(c.url)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return usagef("mobile.url must be an http or https address")
	}
	if u.User != nil {
		return usagef("mobile.url must not carry credentials")
	}
	switch c.platform {
	case platformAndroid, platformIOS:
	default:
		return usagef("mobile.platform must be %s or %s, got %q", platformAndroid, platformIOS, c.platform)
	}
	if c.device == "" {
		return usagef("mobile.device is not set: the executor never picks a device")
	}
	return nil
}

func (c *setupConfig) readLimits(ctx context.Context) error {
	idle, err := seconds(ctx, "mobile.idleTimeout", defaultIdleTimeout)
	if err != nil {
		return err
	}
	c.idleTimeout = int(idle / time.Second)
	if c.syncTimeout, err = seconds(ctx, "mobile.syncTimeout", defaultSyncTimeout); err != nil {
		return err
	}
	c.sessionTimeout, err = seconds(ctx, "mobile.sessionTimeout", 0)
	if err != nil {
		return err
	}
	if c.sessionTimeout == 0 {
		c.sessionTimeout = sessionOpenTimeout
		if c.platform == platformIOS {
			// the first session of a run builds WebDriverAgent on the device, which takes minutes
			c.sessionTimeout = iosSessionOpenTimeout
		}
	}
	return nil
}

// seconds reads a duration in seconds; unset gives def, a negative value is refused.
func seconds(ctx context.Context, name string, def int) (time.Duration, error) {
	n := venom.IntVarFromCtx(ctx, name)
	if n < 0 {
		return 0, usagef("%s must not be negative", name)
	}
	if n == 0 {
		n = def
	}
	return time.Duration(n) * time.Second, nil
}

// checkCapabilities refuses a capability the executor sets itself, naming the conflict (FR-06). Names are compared
// without the appium: prefix, and inside appium:options too.
func checkCapabilities(caps map[string]interface{}) error {
	names := make([]string, 0, len(caps))
	for k := range caps {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		if owner, ok := ownedCapabilities[strings.TrimPrefix(k, capabilityPrefix)]; ok {
			return usagef("mobile.capabilities sets %q, which is set by %s", k, owner)
		}
		if err := checkNested(k, caps[k]); err != nil {
			return err
		}
	}
	return nil
}

// checkNested checks the capabilities that appium:options carries.
func checkNested(key string, value interface{}) error {
	nested, ok := value.(map[string]interface{})
	if !ok || key != capabilityPrefix+"options" {
		return nil
	}
	return checkCapabilities(nested)
}

func normaliseMap(m map[string]interface{}) map[string]interface{} {
	out, ok := normaliseValue(m).(map[string]interface{})
	if !ok {
		return nil
	}
	return out
}

// normaliseValue turns the YAML maps of a variable into JSON-encodable ones.
func normaliseValue(v interface{}) interface{} {
	switch typed := v.(type) {
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(typed))
		for k, item := range typed {
			out[fmt.Sprint(k)] = normaliseValue(item)
		}
		return out
	case map[string]interface{}:
		out := make(map[string]interface{}, len(typed))
		for k, item := range typed {
			out[k] = normaliseValue(item)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(typed))
		for i, item := range typed {
			out[i] = normaliseValue(item)
		}
		return out
	default:
		return v
	}
}
