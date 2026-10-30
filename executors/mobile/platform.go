package mobile

// platformProfile is what differs between the platforms in the session request: the names Appium wants, spelled once.
type platformProfile struct {
	platformName   string // the W3C platformName
	automationName string // the driver
	appIDKey       string // the capability that names the app by its id, without the appium: prefix
}

var profiles = map[string]platformProfile{
	platformAndroid: {platformName: "Android", automationName: "UiAutomator2", appIDKey: "appPackage"},
	platformIOS:     {platformName: "iOS", automationName: "XCUITest", appIDKey: "bundleId"},
}

// profileOf returns the profile of a platform that checkTarget accepted; any other value gets the Android one.
func profileOf(platform string) platformProfile {
	if p, ok := profiles[platform]; ok {
		return p
	}
	return profiles[platformAndroid]
}

// ios tells whether the session runs on iOS.
func (s *session) ios() bool { return s.facts.platform == platformIOS }
