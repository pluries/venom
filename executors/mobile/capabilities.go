package mobile

import "regexp"

// uuidPattern matches the identifier of a simulator or a device (8-4-4-4-12 hexadecimal digits).
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// iosCapabilities are the executor's own capabilities on iOS. mobile.device is a UDID when it looks like one, and a
// simulator name otherwise: a name goes to deviceName alone, so that the server picks the simulator by name.
func iosCapabilities(cfg setupConfig) map[string]interface{} {
	p := profileOf(platformIOS)
	caps := map[string]interface{}{
		platformNameKey:                        p.platformName,
		capabilityPrefix + "automationName":    p.automationName,
		capabilityPrefix + "deviceName":        cfg.device,
		capabilityPrefix + "newCommandTimeout": cfg.idleTimeout,
	}
	if uuidPattern.MatchString(cfg.device) {
		caps[capabilityPrefix+"udid"] = cfg.device
	}
	if cfg.osVersion != "" {
		caps[capabilityPrefix+"platformVersion"] = cfg.osVersion
	}
	return caps
}

// androidCapabilities are the executor's own capabilities on Android.
func androidCapabilities(cfg setupConfig) map[string]interface{} {
	p := profileOf(platformAndroid)
	caps := map[string]interface{}{
		platformNameKey:                        p.platformName,
		capabilityPrefix + "automationName":    p.automationName,
		capabilityPrefix + "deviceName":        cfg.device,
		capabilityPrefix + "udid":              cfg.device,
		capabilityPrefix + "newCommandTimeout": cfg.idleTimeout,
	}
	if cfg.osVersion != "" {
		caps[capabilityPrefix+"platformVersion"] = cfg.osVersion
	}
	return caps
}
