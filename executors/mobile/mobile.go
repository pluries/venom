// Package mobile is the venom executor that drives a native Android app through an Appium server (ADR-0013).
package mobile

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"

	"github.com/ovh/venom"
)

var (
	_ venom.Executor          = new(Executor)
	_ venom.ExecutorWithSetup = new(Executor)
)

// Name of the executor and key of its session in the test case context.
const (
	Name       = "mobile"
	ContextKey = venom.ContextKey("mobileContext")
)

// New returns a new Executor
func New() venom.Executor {
	return &Executor{}
}

// Executor is a step of type mobile.
type Executor struct {
	Action string `json:"action,omitempty" yaml:"action,omitempty"`

	// the locator of the element a step targets: exactly one of the six (FR-07, FR-10)
	AccessibilityID string `json:"accessibilityId,omitempty" yaml:"accessibilityId,omitempty" mapstructure:"accessibilityId"`
	Text            string `json:"text,omitempty" yaml:"text,omitempty" mapstructure:"text"`
	ID              string `json:"id,omitempty" yaml:"id,omitempty" mapstructure:"id"`
	XPath           string `json:"xpath,omitempty" yaml:"xpath,omitempty" mapstructure:"xpath"`
	UIAutomator     string `json:"uiautomator,omitempty" yaml:"uiautomator,omitempty" mapstructure:"uiautomator"`
	CSS             string `json:"css,omitempty" yaml:"css,omitempty" mapstructure:"css"` // only inside a web view

	// iOS only locators (XCUITest)
	Predicate  string `json:"predicate,omitempty" yaml:"predicate,omitempty" mapstructure:"predicate"`    // an NSPredicate string
	ClassChain string `json:"classChain,omitempty" yaml:"classChain,omitempty" mapstructure:"classChain"` // an iOS class chain
	Name       string `json:"name,omitempty" yaml:"name,omitempty" mapstructure:"name"`                   // the element name

	// the inputs of the actions (FR-09); see README
	Value     string   `json:"value,omitempty" yaml:"value,omitempty" mapstructure:"value"`             // fill: the text to type
	Clear     bool     `json:"clear,omitempty" yaml:"clear,omitempty" mapstructure:"clear"`             // fill: clear first
	Attribute string   `json:"attribute,omitempty" yaml:"attribute,omitempty" mapstructure:"attribute"` // read: the attribute, else the text
	State     string   `json:"state,omitempty" yaml:"state,omitempty" mapstructure:"state"`             // waitFor: present (default) or absent
	Direction string   `json:"direction,omitempty" yaml:"direction,omitempty" mapstructure:"direction"` // swipe, scroll
	Until     bool     `json:"until,omitempty" yaml:"until,omitempty" mapstructure:"until"`             // scroll: until the element is visible
	Percent   *float64 `json:"percent,omitempty" yaml:"percent,omitempty" mapstructure:"percent"`       // swipe, scroll: extent, 0 to 1
	AppID     string   `json:"appId,omitempty" yaml:"appId,omitempty" mapstructure:"appId"`             // launch, terminate
	Path      string   `json:"path,omitempty" yaml:"path,omitempty" mapstructure:"path"`                // screenshot: the file to write
	Context   string   `json:"context,omitempty" yaml:"context,omitempty" mapstructure:"context"`       // context: native, a web view name, or empty for the first
	URL       string   `json:"url,omitempty" yaml:"url,omitempty" mapstructure:"url"`                   // openURL: the deep link, scheme://...
	Choice    string   `json:"choice,omitempty" yaml:"choice,omitempty" mapstructure:"choice"`          // alert (iOS): accept (default) or dismiss
	Button    string   `json:"button,omitempty" yaml:"button,omitempty" mapstructure:"button"`          // alert (iOS): the label of the button to press

	// a point by coordinates (FR-11): tap at x, y; swipe from x, y to toX, toY. unit: pixels (default, or px) or percent
	X    *float64 `json:"x,omitempty" yaml:"x,omitempty" mapstructure:"x"`
	Y    *float64 `json:"y,omitempty" yaml:"y,omitempty" mapstructure:"y"`
	ToX  *float64 `json:"toX,omitempty" yaml:"toX,omitempty" mapstructure:"toX"`
	ToY  *float64 `json:"toY,omitempty" yaml:"toY,omitempty" mapstructure:"toY"`
	Unit string   `json:"unit,omitempty" yaml:"unit,omitempty" mapstructure:"unit"`

	// SyncTimeout overrides the session's implicit wait for this step, in seconds (FR-08).
	SyncTimeout *float64 `json:"syncTimeout,omitempty" yaml:"syncTimeout,omitempty" mapstructure:"syncTimeout"`
}

// Result represents a step result. Every key is flat and lower case. The run facts (platform to appsha256) repeat in
// every result, so that each test case of a report shows which build ran on which device (FR-19).
type Result struct {
	Text        string   `json:"text,omitempty" yaml:"text,omitempty"`
	Value       string   `json:"value,omitempty" yaml:"value,omitempty"`
	Count       int      `json:"count,omitempty" yaml:"count,omitempty"`
	Found       bool     `json:"found,omitempty" yaml:"found,omitempty"`
	Context     string   `json:"context,omitempty" yaml:"context,omitempty"`
	Contexts    []string `json:"contexts,omitempty" yaml:"contexts,omitempty"`
	Screenshot  string   `json:"screenshot,omitempty" yaml:"screenshot,omitempty"`
	Source      string   `json:"source,omitempty" yaml:"source,omitempty"`
	TimeSeconds float64  `json:"timeseconds" yaml:"timeseconds"`
	Err         string   `json:"err" yaml:"err"`
	Platform    string   `json:"platform,omitempty" yaml:"platform,omitempty"`
	Device      string   `json:"device,omitempty" yaml:"device,omitempty"`
	OSVersion   string   `json:"osversion,omitempty" yaml:"osversion,omitempty"`
	App         string   `json:"app,omitempty" yaml:"app,omitempty"`
	AppSHA256   string   `json:"appsha256,omitempty" yaml:"appsha256,omitempty"`
	AppID       string   `json:"appid,omitempty" yaml:"appid,omitempty"`
}

// ZeroValueResult return an empty implementation of this executor result
func (Executor) ZeroValueResult() interface{} {
	return Result{}
}

// GetDefaultAssertions return default assertions for type mobile
func (Executor) GetDefaultAssertions() *venom.StepAssertions {
	return &venom.StepAssertions{Assertions: []venom.Assertion{"result.err ShouldBeEmpty"}}
}

// actionFunc runs one action on the open session and fills the result.
type actionFunc func(ctx context.Context, s *session, step Executor, result *Result) error

// usageError is a misuse of the executor (a wrong step or setting). It fails the step as a Go error, where a failure
// reported by the server or the device lands in Result.Err (D4).
type usageError struct{ msg string }

func (e *usageError) Error() string { return e.msg }

func usagef(format string, args ...interface{}) error {
	return &usageError{msg: fmt.Sprintf(format, args...)}
}

// Setup opens the device session of the test case (FR-02). A failed Setup has no TearDown, so openSession leaves no
// session behind when it fails.
func (Executor) Setup(ctx context.Context, _ venom.H) (context.Context, error) {
	cfg, err := readConfig(ctx)
	if err != nil {
		return ctx, err
	}
	s, err := openSession(ctx, cfg)
	if err != nil {
		return ctx, err
	}
	venom.Info(ctx, "mobile session opened: %s", s.facts.describe())
	return context.WithValue(ctx, ContextKey, s), nil
}

func getSession(ctx context.Context) *session {
	// a value of another type under the exported key degrades to "no session" rather than a panic
	s, ok := ctx.Value(ContextKey).(*session)
	if !ok {
		return nil
	}
	return s
}

// TearDown closes the device session, also after a failing step (FR-02).
func (Executor) TearDown(ctx context.Context) error {
	s := getSession(ctx)
	if s == nil {
		return nil
	}
	return s.close(ctx)
}

// Run execute TestStep of type mobile
func (Executor) Run(ctx context.Context, step venom.TestStep) (interface{}, error) {
	s := getSession(ctx)
	if s == nil {
		return nil, usagef("no mobile session: the executor's Setup did not run")
	}
	s.runMu.Lock()
	defer s.runMu.Unlock()
	if s.closed {
		return nil, usagef("the mobile session is already closed")
	}

	var e Executor
	if err := mapstructure.Decode(step, &e); err != nil {
		return nil, err
	}
	action, err := lookupAction(e.Action)
	if err != nil {
		return nil, err
	}

	if err := refuseStrayPoint(e); err != nil {
		return nil, err
	}

	start := time.Now()
	result := Result{
		Platform:  s.facts.platform,
		Device:    s.facts.device,
		OSVersion: s.facts.osVersion,
		App:       s.facts.app,
		AppSHA256: s.facts.appSHA256,
		AppID:     s.facts.appID,
	}
	callCtx, done := s.callContext(ctx)
	defer done()
	if err := action(callCtx, s, e, &result); err != nil {
		var misuse *usageError
		if errors.As(err, &misuse) {
			return nil, err
		}
		result.Err = fmt.Sprintf("action %s: %v", e.Action, err)
		s.diagnose(ctx, err)
	}
	result.TimeSeconds = time.Since(start).Seconds()
	return result, nil
}

// lookupAction finds the action of a step; an unknown one is refused with the list of the supported ones (T10).
func lookupAction(name string) (actionFunc, error) {
	if action, ok := actions[name]; ok {
		return action, nil
	}
	names := make([]string, 0, len(actions))
	for k := range actions {
		names = append(names, k)
	}
	sort.Strings(names)
	supported := "none"
	if len(names) > 0 {
		supported = strings.Join(names, ", ")
	}
	return nil, usagef("unknown action %q, supported actions: %s", name, supported)
}
