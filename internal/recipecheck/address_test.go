package recipecheck

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// navigateStep is the web step YAML (after the "- type: web" line) of a navigate to url, whose url is on line 10 of
// suiteWithStep.
func navigateStep(url string) string {
	return "    action:\n      navigate:\n        url: " + yamlQuote(url) + "\n"
}

// US1-AS4, FR-005: a navigate address with a scheme and no template is reported, on the line of the url.
func TestRC09ReportsANavigateWithASchemeAndNoTemplate(t *testing.T) {
	for _, url := range []string{
		"https://app.example.org/",
		"http://localhost:8000/",
		"file:///C:/work/index.html",
	} {
		t.Run(url, func(t *testing.T) {
			folder := oneFile(t, suiteWithStep(navigateStep(url)), specOf)

			findings := onlyRule(t, ruleRC09(folder), "RC09")

			require.Len(t, findings, 1)
			assert.Equal(t, 10, findings[0].Line)
			assert.Equal(t, "a.yml", findings[0].File)
			assert.Equal(t, "US1-AS1 one", findings[0].Testcase)
		})
	}
}

// US1-AS4, FR-005: a relative path and an address that comes from a template are not reported.
func TestRC09QuietForARelativePathAndATemplate(t *testing.T) {
	for _, url := range []string{
		"/login",
		"index.html",
		"{{.appUrl}}",
		"{{.appUrl}}/login",
	} {
		t.Run(url, func(t *testing.T) {
			folder := oneFile(t, suiteWithStep(navigateStep(url)), specOf)

			assert.Empty(t, ruleRC09(folder))
		})
	}
}

func executeCommandStep(script string) string {
	return "    action:\n      execute:\n        command: " + yamlQuote(script) + "\n"
}

func executeArgsStep(arg string) string {
	return "    action:\n      execute:\n        command: run\n        args:\n        - " + yamlQuote(arg) + "\n"
}

// US3-AS4, FR-014: a non-local http or https host in an execute script is reported.
func TestRC09ReportsANonLocalHostInAnExecuteScript(t *testing.T) {
	for _, host := range []string{
		"https://api.example.org/v1",
		"http://10.0.0.5/x",
		"http://notlocalhost/",
		"https://app.localhost.example.org/",
	} {
		t.Run(host, func(t *testing.T) {
			folder := oneFile(t, suiteWithStep(executeCommandStep("return fetch('"+host+"').then(r => r.text())")), specOf)

			findings := onlyRule(t, ruleRC09(folder), "RC09")

			require.Len(t, findings, 1)
			assert.Equal(t, 10, findings[0].Line)
		})
	}
}

// US3-AS4, FR-014: the arguments of an execute are scanned too.
func TestRC09ReportsANonLocalHostInAnExecuteArgument(t *testing.T) {
	folder := oneFile(t, suiteWithStep(executeArgsStep("https://api.example.org/v1")), specOf)

	findings := onlyRule(t, ruleRC09(folder), "RC09")

	require.Len(t, findings, 1)
	assert.Equal(t, 12, findings[0].Line)
}

// US3-AS4, FR-014: the local hosts of the contract are not reported.
func TestRC09QuietForLocalHostsInAnExecuteScript(t *testing.T) {
	for _, host := range []string{
		"http://localhost:3000/x",
		"https://127.0.0.1/",
		"http://[::1]:8080/",
		"http://app.localhost/",
		"http://shop.test/",
		"http://nothing.invalid/",
	} {
		t.Run(host, func(t *testing.T) {
			folder := oneFile(t, suiteWithStep(executeCommandStep("return fetch('"+host+"').then(r => r.text())")), specOf)

			assert.Empty(t, ruleRC09(folder))
		})
	}
}

// US3-AS4: a script with a local and a non-local host is one finding on its line.
func TestRC09ReportsOnceAScriptWithSeveralHosts(t *testing.T) {
	script := "fetch('http://localhost:3000/a'); fetch('https://api.example.org/b'); fetch('https://cdn.example.org/c')"
	folder := oneFile(t, suiteWithStep(executeCommandStep(script)), specOf)

	assert.Len(t, onlyRule(t, ruleRC09(folder), "RC09"), 1)
}

func mockStep(body string) string {
	return "    mock:\n    - url: 'http://api.test/*'\n      status: 200\n      body: " + yamlQuote(body) + "\n"
}

// US3-AS4, FR-014: a non-local host in a mock body is reported, on the line of the body.
func TestRC09ReportsANonLocalHostInAMockBody(t *testing.T) {
	folder := oneFile(t, suiteWithStep(mockStep(`{"script":"https://cdn.example.org/a.js"}`)), specOf)

	findings := onlyRule(t, ruleRC09(folder), "RC09")

	require.Len(t, findings, 1)
	assert.Equal(t, 11, findings[0].Line)
}

// US3-AS4, FR-014: a local host, plain data and an empty body in a mock are not reported.
func TestRC09QuietForLocalAndEmptyMockBodies(t *testing.T) {
	for _, body := range []string{`{"next":"http://localhost:8000/p"}`, `{"ok":true}`, ""} {
		t.Run(body, func(t *testing.T) {
			folder := oneFile(t, suiteWithStep(mockStep(body)), specOf)

			assert.Empty(t, ruleRC09(folder))
		})
	}
}

func suiteVarsWith(value string) string {
	return "name: Suite\nvars:\n  spec: 014-web-mobile-recipes\n  appUrl: " + yamlQuote(value) + "\ntestcases:\n" + liveTestcase("US1-AS1 one")
}

// US3-AS4, FR-014: a non-local host in the default of a suite variable is reported, on its line.
func TestRC09ReportsANonLocalHostInASuiteVariableDefault(t *testing.T) {
	folder := oneFile(t, suiteVarsWith("https://prod.example.org/"), specOf)

	findings := onlyRule(t, ruleRC09(folder), "RC09")

	require.Len(t, findings, 1)
	assert.Equal(t, 4, findings[0].Line)
}

// US3-AS4, FR-014: the variables of a testcase and of a step are variable defaults too.
func TestRC09ReportsANonLocalHostInATestcaseAndStepVariable(t *testing.T) {
	text := header("014-web-mobile-recipes") + "testcases:\n" +
		"- name: US1-AS1 one\n  vars:\n    base: 'https://prod.example.org/'\n  steps:\n  - type: web\n    vars:\n      other: 'http://10.1.2.3/'\n    action:\n      find: h1\n    assertions:\n    - result.find ShouldEqual 1\n"

	findings := onlyRule(t, ruleRC09(oneFile(t, text, specOf)), "RC09")

	require.Len(t, findings, 2)
	assert.Equal(t, 7, findings[0].Line)
	assert.Equal(t, 11, findings[1].Line)
}

// US3-AS4, FR-014: a local host and an empty value are not reported.
func TestRC09QuietForLocalAndEmptyVariableDefaults(t *testing.T) {
	for _, value := range []string{"http://localhost:8000", "http://127.0.0.1:9000/", "", "not an address", "milk"} {
		t.Run(value, func(t *testing.T) {
			assert.Empty(t, ruleRC09(oneFile(t, suiteVarsWith(value), specOf)))
		})
	}
}

// US3-AS4, FR-014: the finding names the host only; the userinfo of the address (a token, a password) is never printed.
func TestRC09DoesNotPrintTheUserinfoOfAnAddress(t *testing.T) {
	token := "gh" + "p_" + strings.Repeat("aB3dE5", 6)
	folder := oneFile(t, suiteWithStep(executeCommandStep("git clone https://"+token+"@github.com/x")), specOf)

	findings := onlyRule(t, ruleRC09(folder), "RC09")

	require.Len(t, findings, 1)
	assert.Contains(t, findings[0].Message, "github.com")
	assert.NotContains(t, findings[0].Message, token)
	assert.NotContains(t, findings[0].Message, "ghp_")
}

// US3-AS4, FR-014: the host of an address with userinfo is the one after the "@": a local-looking user name is no
// escape, and the user name and password are not taken for the host.
func TestRC09ReadsTheHostAfterTheUserinfo(t *testing.T) {
	for name, address := range map[string]string{
		"local user name with a password": "http://localhost:" + "pw" + "@evil.com/",
		"user name and password":          "http://user:" + "pass" + "@evil.com",
	} {
		t.Run(name, func(t *testing.T) {
			folder := oneFile(t, suiteWithStep(executeCommandStep("fetch('"+address+"')")), specOf)

			findings := onlyRule(t, ruleRC09(folder), "RC09")

			require.Len(t, findings, 1)
			assert.Contains(t, findings[0].Message, "the host evil.com is not local")
			assert.NotContains(t, findings[0].Message, "pass")
			assert.NotContains(t, findings[0].Message, "pw")
		})
	}
}

// US1-AS4, FR-005: a navigate url with a scheme is reported when its authority (between "://" and the first "/", "?"
// or "#") holds no template, whatever follows the authority.
func TestRC09ReportsANavigateWhoseAuthorityHasNoTemplate(t *testing.T) {
	for _, url := range []string{
		"https://prod.example.com/{{.path}}",
		"http://localhost:8000/x",
		"http://localhost:8000/{{.page}}",
		"https://prod.example.com?next={{.page}}",
		"https://prod.example.com#{{.anchor}}",
	} {
		t.Run(url, func(t *testing.T) {
			folder := oneFile(t, suiteWithStep(navigateStep(url)), specOf)

			findings := onlyRule(t, ruleRC09(folder), "RC09")

			require.Len(t, findings, 1)
			assert.Equal(t, 10, findings[0].Line)
		})
	}
}

// US1-AS4, FR-005: an address taken from a variable, in the authority or as the whole url, is not reported.
func TestRC09QuietForANavigateWhoseAuthorityHasATemplate(t *testing.T) {
	for _, url := range []string{
		"{{.appUrl}}/x",
		"{{.appUrl}}",
		"/fr/",
		"https://{{.host}}/x",
		"https://{{.host}}:{{.port}}/x",
	} {
		t.Run(url, func(t *testing.T) {
			folder := oneFile(t, suiteWithStep(navigateStep(url)), specOf)

			assert.Empty(t, ruleRC09(folder))
		})
	}
}
