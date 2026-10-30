package interpolate

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func BenchmarkDo(b *testing.B) {
	for n := 0; n < b.N; n++ {

		type args struct {
			input string
			vars  map[string]string
		}
		test := struct {
			args args
		}{
			args: args{
				input: `
				{
				"env": {
				"KEYA":"{{.cds.env.vAppKey}}",
				"KEYB": "{{.cds.env.vAppKeyHatchery}}",
				"ADDR":"{{.cds.env.addr}}"
				},
				"labels": {
				"TOKEN": "{{.cds.env.token}}",
				"HOST": "cds-hatchery-marathon-{{.cds.env.name}}.{{.cds.env.vHost}}",
				}
				}`,
				vars: map[string]string{"cds.env.name": "", "cds.env.token": "aValidTokenString", "cds.env.addr": "", "cds.env.vAppKey": "aValue"},
			},
		}

		for i := 0; i < 60; i++ {
			test.args.vars[fmt.Sprintf("%d", i)] = fmt.Sprintf(">>%d<<", i)
		}

		_, err := Do(test.args.input, test.args.vars)
		assert.NoError(b, err)
	}
}

func BenchmarkDoNothing(b *testing.B) {
	for n := 0; n < b.N; n++ {

		type args struct {
			input string
			vars  map[string]string
		}
		test := struct {
			args args
		}{
			args: args{
				input: `
				Lorem ipsum dolor sit amet, consectetur adipiscing elit. Aliquam felis nulla, vulputate ac eros vel, placerat dignissim turpis. Sed et ex lectus. Donec viverra nisi vel dictum rhoncus. Sed dictum tempus quam, ut efficitur arcu viverra vitae. Suspendisse aliquam venenatis scelerisque. Praesent et mattis enim. In efficitur imperdiet nulla a sagittis. Maecenas aliquet magna in sollicitudin ornare.

				Suspendisse viverra enim nec ante blandit tempus. Sed ut erat suscipit, semper ex eu, eleifend neque. Sed orci justo, bibendum laoreet libero cursus, venenatis fringilla dui. Curabitur tristique odio ut neque sollicitudin ultrices. Integer metus nibh, dignissim non pellentesque et, volutpat vel ante. Pellentesque ultrices ante vel mauris aliquam porttitor. Nunc nec sem facilisis, ullamcorper ex sed, elementum elit. Nulla risus magna, tempor et ultricies id, vehicula ac massa. Mauris venenatis libero libero, id lobortis mi semper aliquam. Aenean neque turpis, feugiat vel rutrum vitae, auctor quis nisl. Donec placerat nec mauris vitae malesuada. Proin quis gravida nulla. Pellentesque in pellentesque metus, in finibus dui. Sed rutrum, libero sit amet cursus scelerisque, sem orci condimentum nunc, quis egestas tellus orci ac nisl. Mauris viverra tincidunt diam ac sollicitudin. Nunc venenatis, nibh at laoreet pellentesque, lacus tellus molestie lorem, et sollicitudin nunc neque ut turpis.
				`,
				vars: map[string]string{"cds.env.name": "", "cds.env.token": "aValidTokenString", "cds.env.addr": "", "cds.env.vAppKey": "aValue"},
			},
		}

		for i := 0; i < 60; i++ {
			test.args.vars[fmt.Sprintf("%d", i)] = fmt.Sprintf(">>%d<<", i)
		}

		_, err := Do(test.args.input, test.args.vars)
		assert.NoError(b, err)

	}
}

func TestDo(t *testing.T) {
	type args struct {
		input string
		vars  map[string]string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
		enable  bool
	}{
		{
			name: "default value with empty default",
			args: args{
				input: `aa:{{.cds.app.foo | default ""}}end`,
				vars:  map[string]string{},
			},
			want:   `aa:end`,
			enable: true,
		},
		{
			name: "default value",
			args: args{
				input: `aa:{{.cds.app.foo | default "bar" }}:end`,
				vars:  map[string]string{},
			},
			want:   `aa:bar:end`,
			enable: true,
		},
		{
			name: "default value with variables (easy)",
			args: args{
				input: `{{.cds.app.foo | default .val }}`,
				vars: map[string]string{
					"val": "biz",
				},
			},
			want:   `biz`,
			enable: true,
		},
		{
			name: "default value with variables (not so easy)",
			args: args{
				input: `{{.cds.app.foo | default .cds.app.bar }}`,
				vars: map[string]string{
					"cds.app.bar": "biz",
				},
			},
			want:   `biz`,
			enable: true,
		},
		{
			name: "default value with variables (so hard)",
			args: args{
				input: `{{.cds.app.foo | default .cds.app.bar .cds.app.biz }}`,
				vars: map[string]string{
					"cds.app.biz": "biz",
				},
			},
			want:   `biz`,
			enable: true,
		},
		{
			name: "default value with variables (with pipeline)",
			args: args{
				input: `{{.cds.app.foo | default .cds.app.bar | default .cds.app.biz | upper }}`,
				vars: map[string]string{
					"cds.app.biz": "biz",
				},
			},
			want:   `BIZ`,
			enable: true,
		},
		{
			name: "default value with pipeline",
			args: args{
				input: `{{.cds.app.foo | upper}}`,
			},
			want:   `{{.cds.app.foo | upper}}`,
			enable: true,
		},
		{
			name: "default value with pipeline",
			args: args{
				input: `{{.cds.app.foo | upper | lower}}`,
			},
			want:   `{{.cds.app.foo | upper | lower}}`,
			enable: true,
		},
		{
			name: "default value with knowned var",
			args: args{
				input: `aa:{{.cds.app.foo | default "bar"}}end`,
				vars:  map[string]string{"cds.app.foo": "value"},
			},
			want:   `aa:valueend`,
			enable: true,
		},
		{
			name: "default empty value with knowned var",
			args: args{
				input: `aa:{{.cds.app.foo | default ""}}end`,
				vars:  map[string]string{"cds.app.foo": "value"},
			},
			want:   `aa:valueend`,
			enable: true,
		},
		{
			name: "unknown function",
			args: args{
				input: `echo '{{"conf"|uvault}}'`,
				vars:  map[string]string{},
			},
			want:   `echo '{{"conf"|uvault}}'`,
			enable: true,
		},
		{
			name: "simple",
			args: args{
				input: "a {{.cds.app.value}}",
				vars:  map[string]string{"cds.app.value": "value"},
			},
			want:   "a value",
			enable: true,
		},
		{
			name: "only unknown",
			args: args{
				input: "a value unknown {{.cds.app.foo}}",
				vars:  map[string]string{"cds.app.value": "value"},
			},
			want:   "a value unknown {{.cds.app.foo}}",
			enable: true,
		},
		{
			name: "simple with unknown",
			args: args{
				input: "a {{.cds.app.value}} and another value unknown {{.cds.app.foo}}",
				vars:  map[string]string{"cds.app.value": "value"},
			},
			want:   "a value and another value unknown {{.cds.app.foo}}",
			enable: true,
		},
		{
			name: "upper",
			args: args{
				input: "a {{.cds.app.value | upper}} and another value unknown {{.cds.app.foo}}",
				vars:  map[string]string{"cds.app.value": "value"},
			},
			want:   "a VALUE and another value unknown {{.cds.app.foo}}",
			enable: true,
		},
		{
			name: "title and filter on unknow",
			args: args{
				input: "a {{.cds.app.value | title }} and another value unknown {{.cds.app.foo | lower}}",
				vars:  map[string]string{"cds.app.value": "value"},
			},
			want:   "a Value and another value unknown {{.cds.app.foo | lower}}",
			enable: true,
		},
		{
			name: "many",
			args: args{
				input: "{{.cds.app.bar}} a {{.cds.app.valuea | upper }}, a {{.cds.app.valueb | title}}.{{.cds.app.valuec}}-{{.cds.app.foo}}",
				vars:  map[string]string{"cds.app.valuea": "valuea", "cds.app.valueb": "valueb", "cds.app.valuec": "valuec"},
			},
			want:   "{{.cds.app.bar}} a VALUEA, a Valueb.valuec-{{.cds.app.foo}}",
			enable: true,
		},
		{
			name: "two same unknown",
			args: args{
				input: `A:{{.cds.env.myenvpassword}} B:{{.cds.env.myenvpassword}}`,
				vars:  map[string]string{},
			},
			want:   `A:{{.cds.env.myenvpassword}} B:{{.cds.env.myenvpassword}}`,
			enable: true,
		},
		{
			name: "two same unknown, but one with a filter",
			args: args{
				input: `A:{{.cds.env.myenvpassword}} B:{{.cds.env.myenvpassword | upper}}`,
				vars:  map[string]string{},
			},
			want:   `A:{{.cds.env.myenvpassword}} B:{{.cds.env.myenvpassword | upper}}`,
			enable: true,
		},
		{
			name: "empty string",
			args: args{
				input: "a {{.cds.app.myKey}} and another key with empty value *{{.cds.app.myKeyAnother}}*",
				vars:  map[string]string{"cds.app.myKey": "valueKey", "cds.app.myKeyAnother": ""},
			},
			want:   "a valueKey and another key with empty value **",
			enable: true,
		},
		{
			name: "two keys with same first characters",
			args: args{
				input: "a {{.cds.app.myKey}} and another key value {{.cds.app.myKeyAnother}}",
				vars:  map[string]string{"cds.app.myKey": "valueKey", "cds.app.myKeyAnother": "valueKeyAnother"},
			},
			want:   "a valueKey and another key value valueKeyAnother",
			enable: true,
		},
		{
			name: "key with - and a unknown key",
			args: args{
				input: "a {{.cds.app.my-key}}.{{.cds.app.foo-key}} and another key value {{.cds.app.my-key}}",
				vars:  map[string]string{"cds.app.my-key": "value-key"},
			},
			want:   "a value-key.{{.cds.app.foo-key}} and another key value value-key",
			enable: true,
		},
		{
			name: "key with - and a empty key",
			args: args{
				input: "a {{.cds.app.my-key}}.{{.cds.app.foo-key}}.and another key value {{.cds.app.my-key}}",
				vars:  map[string]string{"cds.app.my-key": "value-key", "cds.app.foo-key": ""},
			},
			want:   "a value-key..and another key value value-key",
			enable: true,
		},
		{
			name: "tiret",
			args: args{
				input: `"METRICS_WRITE_TOKEN": "{{.cds.env.metrics-exposer.write.token}}"`,
				vars:  map[string]string{"cds.env.metrics-exposer.write.token": "valueKey"},
			},
			want:   `"METRICS_WRITE_TOKEN": "valueKey"`,
			enable: true,
		},
		{
			name: "espace func",
			args: args{
				input: `a {{.cds.foo}} here, {{.cds.title | title}}, {{.cds.upper | upper}}, {{.cds.lower | lower}}, {{.cds.escape | escape}}`,
				vars: map[string]string{
					"cds.foo":    "valbar",
					"cds.title":  "mytitle-bis",
					"cds.upper":  "toupper",
					"cds.lower":  "TOLOWER",
					"cds.escape": "a/b.c_d",
				},
			},
			want:   `a valbar here, Mytitle-Bis, TOUPPER, tolower, a-b-c-d`,
			enable: true,
		},
		{
			name: "config",
			args: args{
				input: `
				{
				"env": {
				"KEYA":"{{.cds.env.vAppKey}}",
				"KEYB": "{{.cds.env.vAppKeyHatchery}}",
				"ADDR":"{{.cds.env.addr}}"
				},
				"labels": {
				"TOKEN": "{{.cds.env.token}}",
				"HOST": "cds-hatchery-marathon-{{.cds.env.name}}.{{.cds.env.vHost}}",
				}
				}`,
				vars: map[string]string{"cds.env.name": "", "cds.env.token": "aValidTokenString", "cds.env.addr": "", "cds.env.vAppKey": "aValue"},
			},
			want: `
				{
				"env": {
				"KEYA":"aValue",
				"KEYB": "{{.cds.env.vAppKeyHatchery}}",
				"ADDR":""
				},
				"labels": {
				"TOKEN": "aValidTokenString",
				"HOST": "cds-hatchery-marathon-.{{.cds.env.vHost}}",
				}
				}`,
			enable: true,
		},
		{
			name: "same prefix",
			args: args{
				input: `{"HOST": "customer{{.cds.env.lb.prefix}}.{{.cds.env.lb}}"}`,
				vars:  map[string]string{"cds.env.lb": "lb", "cds.env.lb.prefix": "myprefix"},
			},
			want:   `{"HOST": "customermyprefix.lb"}`,
			enable: true,
		},
		{
			name: "git.branch in payload should not be interpolated",
			args: args{
				input: `
		name: "w{{.cds.pip.docker.image}}-generated"
		version: v1.0
		workflow:
		  build-go:
		    pipeline: build-go-generated
		    payload:
		      git.author: ""
		      git.branch: master`,
				vars: map[string]string{
					"git.branch": "master",
					"git.author": "",
				},
			},
			want: `
		name: "w{{.cds.pip.docker.image}}-generated"
		version: v1.0
		workflow:
		  build-go:
		    pipeline: build-go-generated
		    payload:
		      git.author: ""
		      git.branch: master`,
			enable: true,
		},
		{
			name: "- inside function parameter",
			args: args{
				input: `name: "coucou-{{ .name | default "0.0.1-dirty" }}"`,
				vars: map[string]string{
					"git.branch": "master",
					"git.author": "",
				},
			},
			want:   `name: "coucou-0.0.1-dirty"`,
			enable: true,
		},
		{
			name: "- inside function parameter but not used",
			args: args{
				input: `name: "coucou-{{ .name | default "0.0.1-dirty" }}"`,
				vars: map[string]string{
					"name": "toi",
				},
			},
			want:   `name: "coucou-toi"`,
			enable: true,
		},
		{
			name: "- substring",
			args: args{
				input: `name: coucou-{{ .name | substr 0 5 }}`,
				vars: map[string]string{
					"name": "github",
				},
			},
			want:   `name: coucou-githu`,
			enable: true,
		},
		{
			name: "- trunc",
			args: args{
				input: `test_{{.cds.workflow}}_{{.git.hash | trunc 8 }}`,
				vars: map[string]string{
					"cds.workflow":    "myWorkflow",
					"git.hash":        "863ddke13bfef8043960b19cec790f8b9f5435ab",
					"git.hash.before": "863ddke13bfef8043960b19cec790f8b9f5435ab",
				},
			},
			want:   `test_myWorkflow_863ddke1`,
			enable: true,
		},
		{
			name: "add",
			args: args{
				input: "my value {{.cds.app.value | add 3}} {{ add 2 2 }}",
				vars:  map[string]string{"cds.app.value": "1"},
			},
			want:   "my value 4 4",
			enable: true,
		},
		{
			name: "sub",
			args: args{
				input: "my value {{.cds.app.value | sub 1}} {{ sub 5 1 }}",
				vars:  map[string]string{"cds.app.value": "5"},
			},
			want:   "my value -4 4",
			enable: true,
		},
		{
			name: "mul",
			args: args{
				input: "my value {{.cds.app.value | mul 2}} {{ mul 2 2 }}",
				vars:  map[string]string{"cds.app.value": "2"},
			},
			want:   "my value 4 4",
			enable: true,
		},
		{
			name: "div",
			args: args{
				input: "my value {{.cds.app.value | div 10}} {{ div 8 2 }}",
				vars:  map[string]string{"cds.app.value": "2"},
			},
			want:   "my value 5 4",
			enable: true,
		},
		{
			name: "mod",
			args: args{
				input: "my value {{.cds.app.value | mod 6}} {{ mod 10 6 }}",
				vars:  map[string]string{"cds.app.value": "10"},
			},
			want:   "my value 6 4",
			enable: true,
		},
		{
			name: "dirname",
			args: args{
				input: "{{.path | dirname}}",
				vars: map[string]string{
					"path": "/a/b/c",
				},
			},
			want:   "/a/b",
			enable: true,
		},
		{
			name: "basename",
			args: args{
				input: "{{.path | basename}}",
				vars: map[string]string{
					"path": "/ab/c",
				},
			},
			want:   "c",
			enable: true,
		},
		{
			name: "urlencode word",
			args: args{
				input: "{{.query | urlencode}}",
				vars: map[string]string{
					"query": "Trollhättan",
				},
			},
			want:   "Trollh%C3%A4ttan",
			enable: true,
		},
		{
			name: "urlencode query",
			args: args{
				input: "{{.query | urlencode}}",
				vars: map[string]string{
					"query": "zone:eq=Somewhere over the rainbow&name:like=%mydomain.localhost.local",
				},
			},
			want:   "zone%3Aeq%3DSomewhere+over+the+rainbow%26name%3Alike%3D%25mydomain.localhost.local",
			enable: true,
		},
		{
			name: "urlencode nothing to do",
			args: args{
				input: "{{.query | urlencode}}",
				vars: map[string]string{
					"query": "patrick",
				},
			},
			want:   "patrick",
			enable: true,
		},
		{
			name: "ternary truthy",
			args: args{
				input: "{{.assert | ternary .foo .bar}}",
				vars: map[string]string{
					"assert": "true",
					"bar":    "bar",
					"foo":    "foo",
				},
			},
			want:   "foo",
			enable: true,
		},
		{
			name: "ternary truthy integer",
			args: args{
				input: "{{ \"1\" | ternary .foo .bar}}",
				vars: map[string]string{
					"bar": "bar",
					"foo": "foo",
				},
			},
			want:   "foo",
			enable: true,
		},
		{
			name: "ternary falsy",
			args: args{
				input: "{{.assert | ternary .foo .bar}}",
				vars: map[string]string{
					"assert": "false",
					"bar":    "bar",
					"foo":    "foo",
				},
			},
			want:   "bar",
			enable: true,
		},
		{
			name: "ternary undef assert",
			args: args{
				input: "{{.assert | ternary .foo .bar}}",
				vars: map[string]string{
					"bar": "bar",
					"foo": "foo",
				},
			},
			want:   "bar",
			enable: true,
		},
		{
			name: "camelCase variable name",
			args: args{
				input: `{"staticVersionLocator":"{{.staticVersionLocator}}"}`,
				vars: map[string]string{
					"staticVersionLocator": "0000000000Y009R8NV8RMTH9S4",
				},
			},
			want:   `{"staticVersionLocator":"0000000000Y009R8NV8RMTH9S4"}`,
			enable: true,
		},
		{
			name: "camelCase variable name with lowercased key in vars map",
			args: args{
				input: `{"staticVersionLocator":"{{.staticVersionLocator}}"}`,
				vars: map[string]string{
					"staticversionlocator": "0000000000Y009R8NV8RMTH9S4",
				},
			},
			want:   `{"staticVersionLocator":"{{.staticVersionLocator}}"}`,
			enable: true,
		},
	}
	for _, tt := range tests {
		if !tt.enable {
			continue
		}

		t.Run(tt.name, func(t *testing.T) {
			got, err := Do(tt.args.input, tt.args.vars)
			if (err != nil) != tt.wantErr {
				t.Errorf("Do() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Do() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWrapHelpers(t *testing.T) {
	wrappedHelpers := wrapHelpers(template.FuncMap{
		"substr":  substring,
		"default": dfault,
		"toJSON":  toJSON,
		"trunc":   trunc,
	})

	tests := []struct {
		name  string
		input string
		want  string
		err   error
		data  map[string]interface{}
	}{
		{
			name:  "with native types",
			input: `{{"text" | substr 1 3 }}`,
			want:  `ex`,
		},
		{
			name:  "with nil values",
			input: `{{.one | default .two "biz" }}`,
			want:  `biz`,
		},
		{
			name:  "with unknown struct value",
			input: `{{.one | toJSON }}`,
			want:  `{"name":"myName","age":30}`,
			data: map[string]interface{}{
				"one": struct {
					Name string `json:"name"`
					Age  int    `json:"age"`
				}{
					Name: "myName",
					Age:  30,
				},
			},
		},
		{
			name:  "with a pointer to val struct",
			input: `{{.one.two | trunc 7 }}`,
			want:  `1234567`,
			data: map[string]interface{}{
				"one": &val{
					"two": &val{
						"_": "1234567890",
					},
				},
			},
		},
		{
			name:  "with a val struct",
			input: `{{.one.two | trunc 7 }}`,
			want:  `1234567`,
			data: map[string]interface{}{
				"one": val{
					"two": val{
						"_": "1234567890",
					},
				},
			},
		},
		{
			name:  "with a unknown value",
			input: `{{.one}}`,
			want:  `<no value>`,
		},
		{
			name:  "with missing params",
			input: `{{.one | trunc}}`,
			err:   fmt.Errorf(`template: input:1:9: executing "input" at <trunc>: error calling trunc: missing params (expected: int, string)`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp, err := template.New("input").Funcs(wrappedHelpers).Parse(tt.input)
			assert.NoError(t, err)

			var buff bytes.Buffer
			errexec := tmp.Execute(&buff, tt.data)
			if tt.err == nil {
				assert.NoError(t, errexec)
			} else {
				assert.Error(t, errexec, tt.err)
			}

			if buff.String() != tt.want {
				t.Errorf("Do() = %v, want %v", buff.String(), tt.want)
			}
		})
	}
}

func TestDashReplacementWithµµµ(t *testing.T) {
	vars := map[string]string{
		"result.headers.x-cache": "a",
	}
	got, err := Do("result.headers.x-cache is {{.result.headers.x-cache}}", vars)
	assert.NoError(t, err)
	assert.Equal(t, "result.headers.x-cache is a", got)
}

func TestStringQuote(t *testing.T) {
	vars := map[string]string{
		"content": `{"foo": "{\"bar\":\"baz\"}"}`,
	}
	got, err := Do("content is {{.content | stringQuote}}", vars)
	assert.NoError(t, err)
	assert.Equal(t, `content is {\"foo\": \"{\\\"bar\\\":\\\"baz\\\"}\"}`, got)
}

// doCase is one characterisation case of Do: an input, the variables, and either the exact output or the exact error.
type doCase struct {
	name    string
	input   string
	vars    map[string]string
	want    string
	wantErr string // "" = Do succeeds and returns want
}

// runDoCases runs characterisation cases of Do and compares output and error text byte for byte.
func runDoCases(t *testing.T, tests []doCase) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Do(tt.input, tt.vars)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				assert.Empty(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestDoReturnsInputWithoutTemplateUnchanged characterises Do's first branch before #2031 splits it: an input with no
// "{{" is returned as it is, whatever the variables.
func TestDoReturnsInputWithoutTemplateUnchanged(t *testing.T) {
	runDoCases(t, []doCase{
		{name: "empty input", input: "", want: ""},
		{name: "plain text with nil vars", input: "plain text", want: "plain text"},
		{name: "closing braces alone are not a template", input: "plain }} text", want: "plain }} text"},
		{name: "a variable reference outside braces is not replaced", input: ".a and a", vars: map[string]string{"a": "x"}, want: ".a and a"},
		{name: "single braces are not a template", input: "{ .a }", vars: map[string]string{"a": "x"}, want: "{ .a }"},
	})
}

// TestDoResolvesKnownVariables characterises how Do resolves the variables it is given: dotted names become nested
// fields, "-" in a name is supported, and the value is inserted as it is (not re-interpolated, not HTML-escaped).
func TestDoResolvesKnownVariables(t *testing.T) {
	runDoCases(t, []doCase{
		{name: "one variable", input: "{{.a}}", vars: map[string]string{"a": "x"}, want: "x"},
		{name: "spaces inside the braces", input: "{{ .a }}", vars: map[string]string{"a": "x"}, want: "x"},
		{name: "the same variable twice", input: "{{.a}} and {{.a}}", vars: map[string]string{"a": "x"}, want: "x and x"},
		{name: "template inside multi-line text", input: "line1\n{{ .a }}\nline3", vars: map[string]string{"a": "x"}, want: "line1\nx\nline3"},
		{name: "dotted name", input: "{{.a.b}}", vars: map[string]string{"a.b": "x"}, want: "x"},
		{name: "a name and a longer name under it both resolve", input: "{{.a}}-{{.a.b}}", vars: map[string]string{"a": "p", "a.b": "c"}, want: "p-c"},
		{name: "the parent value resolves when a child is also set", input: "{{.a}}", vars: map[string]string{"a.b": "c", "a": "p"}, want: "p"},
		{name: "the child resolves through a pipeline when the parent is also set", input: "{{ .a.b | upper }}", vars: map[string]string{"a.b": "x", "a": "p"}, want: "X"},
		{name: "dash in a name", input: "{{.a-b}} {{.a-b}}", vars: map[string]string{"a-b": "1"}, want: "1 1"},
		// the internal replacement of "-" is visible: a µµµ spelling reaches the dashed variable
		{name: "the internal µµµ spelling of a dash resolves the dashed variable", input: "{{.aµµµb}}", vars: map[string]string{"a-b": "1"}, want: "1"},
		// quirk: "a-b" and "aµµµb" share one internal name, and the second one written replaces the value by a nil pointer
		{name: "a dashed name and its µµµ spelling collide and print <nil>", input: "{{.a-b}}", vars: map[string]string{"a-b": "1", "aµµµb": "2"}, want: "<nil>"},
		// quirk: the dash replacement runs over the whole input, plain text included, and is never undone
		{name: "the dash replacement also rewrites a matching plain-text reference", input: "see .my-key here {{.my-key}}", vars: map[string]string{"my-key": "v"}, want: "see .myµµµkey here v"},
		{name: "a value that looks like a template is not re-interpolated", input: "{{.a}}", vars: map[string]string{"a": "{{.b}}", "b": "y"}, want: "{{.b}}"},
		{name: "a value is not HTML-escaped", input: "{{.x}}", vars: map[string]string{"x": "<b>&"}, want: "<b>&"},
		{name: "empty value", input: "[{{.a}}]", vars: map[string]string{"a": ""}, want: "[]"},
		{name: "template variables assigned in the input work", input: "{{ $x := .a }}{{ $x }}", vars: map[string]string{"a": "x"}, want: "x"},
		{name: "a known and an unknown variable side by side", input: "{{ .a }}{{ .u }}", vars: map[string]string{"a": "x"}, want: "x{{ .u }}"},
	})
}

// TestDoReachesEveryVariableTheTemplateNames pins what a template can reach in the variables (#2464): Do must give the
// same output whether or not the scope holds many variables the text never names, for every way a template reads its
// data: fields, children of a name, ancestors of a name, the whole data, ranges, with, index, $ and dashed names.
func TestDoReachesEveryVariableTheTemplateNames(t *testing.T) {
	scope := map[string]string{"a": "1", "b": "2", "c.d": "3", "cfg.n": "3", "cfg.url": "u", "cfg": "root", "my-cfg.my-key": "dv", "other-key": "ov"}
	runDoCases(t, []doCase{
		// an expression with a helper venom does not know (len, with, index, if, range...) that the regex matches is left as written
		{name: "a bare dot with an unknown helper is left as written", input: "{{ len . }}", vars: scope, want: "{{ len . }}"},
		// an expression the regex does not match (it holds a $) reaches the template engine with the whole data
		{name: "the whole data through $", input: "{{ len $ }}", vars: scope, want: "6"},
		{name: "range is cut into pieces by the protection and fails to parse", input: "{{ range $k, $v := .cfg }}{{ $k }},{{ end }}", vars: scope, wantErr: `invalid template format "{{ range $k, $v := .cfg }}{{ $k }},{{"{{ end }}"}}": template: input:1: unexpected EOF`},
		{name: "with is left as written", input: "{{ with .cfg }}{{ .n }}{{ end }}", vars: scope, want: "{{ with .cfg }}{{ .n }}{{ end }}"},
		{name: "index is left as written", input: `{{ index .cfg "url" }}`, vars: scope, want: `{{ index .cfg "url" }}`},
		{name: "a name through $ and a field", input: "{{ $.a }}{{ $.c.d }}", vars: scope, want: "13"},
		{name: "the names inside if and else are replaced one by one, the keywords stay", input: "{{ if .a }}{{ .b }}{{ else }}{{ .cfg.n }}{{ end }}", vars: scope, want: "{{ if .a }}2{{ else }}3{{ end }}"},
		{name: "a name inside parentheses", input: `{{ printf "%s-%s" (.a) (.cfg.n) }}`, vars: scope, want: "1-3"},
		{name: "a dashed nested name", input: "{{.my-cfg.my-key}}", vars: scope, want: "dv"},
		{name: "the dash replacement rewrites a plain-text reference to a variable the template does not name", input: "see .other-key here {{.a}}", vars: scope, want: "see .otherµµµkey here 1"},
		{name: "a parent and its children when only the parent is named", input: "{{.cfg}}", vars: scope, want: "root"},
		{name: "a missing parent with a named child", input: "{{.c.d}}{{.c}}", vars: scope, want: "3{{.c}}"},
	})
}

// doWithFullIndex is Do as it was before #2464: it indexes every variable of the scope, not only the ones the template
// reads. TestDoMatchesTheFullIndex compares Do with it.
func doWithFullIndex(input string, vars map[string]string) (string, error) {
	if !strings.Contains(input, "{{") {
		return input, nil
	}
	data, flatData, replacements, err := indexVariables(vars)
	if err != nil {
		return "", err
	}
	input = strings.NewReplacer(replacements...).Replace(input)
	input = protectUnknownExpressions(input, flatData)
	return execute(input, data)
}

// differentialScopes are the scopes the differential tests run every input of differentialInputs on: a small one, the
// same with unrelated variables, and none.
func differentialScopes() []map[string]string {
	base := map[string]string{"a": "1", "b": "", "c.d": "3", "cfg.n": "3", "cfg.url": "u", "cfg": "root", "x.y.z": "deep", "x.y": "mid",
		"my-cfg.my-key": "dv", "my-var": "mv", "aµµµb": "mu", "list.0": "zero", "list.1": "one"}
	noisy := map[string]string{}
	for k, v := range base {
		noisy[k] = v
	}
	for i := 0; i < 300; i++ {
		noisy[fmt.Sprintf("noise%d.key", i)] = "n"
		noisy[fmt.Sprintf("noise-%d", i)] = "n"
	}
	return []map[string]string{base, noisy, {}}
}

// differentialInputs read the data in the different ways Do has to handle.
var differentialInputs = []string{
	"{{.a}}", "{{ .b }}", "{{.c.d}}", "{{.c}}", "{{.cfg}} {{.cfg.n}}", "{{.x}} {{.x.y}} {{.x.y.z}}", "{{.x.y.z.w}}",
	"{{.my-cfg.my-key}}", "{{.my-var}} {{.my-var | upper}}", "{{.aµµµb}}", "{{.a-b}}", "see .my-var here {{.a}}", "see .noise-3 here",
	"{{.a | default \"d\"}}", "{{.u | default \"d\"}}", "{{.u.v | default \"d\"}}", "{{.u}}", "{{.a}}{{.u}}", "{{.u | unknownhelper}}",
	"{{ len $ }}", "{{ len . }}", "{{ $.a }}{{ $.cfg.url }}", "{{ $x := .a }}{{ $x }}", "{{ $x := .cfg }}{{ $x.n }}",
	"{{ with .cfg }}{{ .n }}{{ end }}", "{{ range $k, $v := .cfg }}{{ $k }}{{ end }}", "{{ if .a }}{{ .b }}{{ else }}{{ .cfg.n }}{{ end }}",
	`{{ index .cfg "url" }}`, `{{ printf "%s-%s" .a .cfg.n }}`, `{{ printf "%s" (.a) }}`, "{{ (index .list.0 ) }}", `{{ template "t" . }}`,
	`{{ define "t" }}{{ .a }}{{ end }}{{ template "t" . }}`, "{{ .a ", "{{ .a }} }}", "plain text", "", "{{}}", "{{ . }}",
	`{{ "x .a " }}`, `{{ "x .u " }}`, `{{ "x .b" | upper }}`, `{{ "x . " | upper }}`, `{{ print "{{ .a }}" }}`, `{{ printf "%s" "see .cfg.n here" }}`, "{{.a | printf \"%q\"}}", "{{.cfg.n | add 1}}", "{{ .list.0 }}{{ .list.1 }}", "{{ .noise3.key }} {{ .noise-3 }}",
}

// TestDoMatchesTheFullIndex checks that indexing only what a template reads (#2464) gives the output and the error of
// indexing the whole scope, for inputs that read the data in different ways, with and without unrelated variables.
func TestDoMatchesTheFullIndex(t *testing.T) {
	for _, scope := range differentialScopes() {
		for _, in := range differentialInputs {
			want, wantErr := doWithFullIndex(in, scope)
			got, gotErr := Do(in, scope)
			require.Equal(t, wantErr == nil, gotErr == nil, "error for %q: want %v, got %v", in, wantErr, gotErr)
			if wantErr != nil {
				require.EqualError(t, gotErr, wantErr.Error(), "input %q", in)
			}
			require.Equal(t, want, got, "input %q with %d variables", in, len(scope))
		}
	}
}

// TestDoLeavesUnknownVariablesLiterally characterises the compatibility boundary of #1975 before #2031 splits Do: an
// expression is left in the output as written when it names a variable Do does not know or a function it does not
// know — unless it uses default or ternary. Which expressions this protects is decided by a regular expression and a
// split on spaces, so the cases below include the ones where that decision is surprising.
func TestDoLeavesUnknownVariablesLiterally(t *testing.T) {
	runDoCases(t, []doCase{
		{name: "unknown variable with nil vars", input: "{{.a}}", want: "{{.a}}"},
		{name: "unknown variable with empty vars", input: "{{.a}}", vars: map[string]string{}, want: "{{.a}}"},
		{name: "names are case-sensitive", input: "{{ .A }}", vars: map[string]string{"a": "x"}, want: "{{ .A }}"},
		{name: "the parent of a dotted name is unknown", input: "{{.a}}", vars: map[string]string{"a.b": "x"}, want: "{{.a}}"},
		{name: "a child of a known value is unknown", input: "{{.a.b.c}}", vars: map[string]string{"a.b": "x"}, want: "{{.a.b.c}}"},
		{name: "a field of a known leaf is unknown", input: "{{ .a.b }}", vars: map[string]string{"a": "x"}, want: "{{ .a.b }}"},
		{name: "unknown variable in a pipeline", input: "{{ .u | upper | lower }} {{ .a | upper }}", vars: map[string]string{"a": "x"}, want: "{{ .u | upper | lower }} X"},
		{name: "unknown variable with a numeric argument", input: "{{.u | trunc 8}}", want: "{{.u | trunc 8}}"},
		{name: "unknown variable with numeric arguments to add", input: "{{ .u | add 1 2 }}", want: "{{ .u | add 1 2 }}"},
		{name: "unknown variable with a quoted argument keeps its quotes", input: `{{.u | quote "x"}}`, want: `{{.u | quote "x"}}`},
		{name: "unknown variable with a number then a quoted argument", input: `{{.u | substr 0 "5"}}`, want: `{{.u | substr 0 "5"}}`},
		{name: "a variable used as a function is unknown", input: `{{ "a" | .u }}`, want: `{{ "a" | .u }}`},
		{name: "unknown function on a known variable", input: "{{ .a | nofn }}", vars: map[string]string{"a": "x"}, want: "{{ .a | nofn }}"},
		{name: "unknown function on a literal", input: `echo '{{"conf"|uvault}}'`, want: `echo '{{"conf"|uvault}}'`},
		// quirk: text/template's own functions and keywords are not in InterpolateHelperFuncs, so they count as unknown
		{name: "template builtin len is treated as unknown", input: "{{ len .a }}", vars: map[string]string{"a": "xyz"}, want: "{{ len .a }}"},
		{name: "template builtin eq is treated as unknown", input: `{{ eq .a "x" }}`, vars: map[string]string{"a": "x"}, want: `{{ eq .a "x" }}`},
		{name: "if and end are treated as unknown, so the block is not evaluated", input: "{{ if .a }}yes{{ end }}", vars: map[string]string{"a": "x"}, want: "{{ if .a }}yes{{ end }}"},
		// quirk: with no space before "|", the split on spaces keeps "|" glued to the name, which is then unknown
		{name: "a known variable directly followed by | and a space is treated as unknown", input: "{{.a| upper}}", vars: map[string]string{"a": "x"}, want: "{{.a| upper}}"},
		// quirk: characters outside the regular expression's class (here parentheses, %) skip the protection entirely
		{name: "parentheses skip the protection: an unknown variable renders <no value>", input: "{{ (.u) }}", want: "<no value>"},
		{name: "a percent sign skips the protection: printf renders a nil argument", input: `{{ printf "%s" .u }}`, want: "%!s(<nil>)"},
	})
}

// TestDoRunsHelpersAndPipelines characterises what Do evaluates: the helpers of InterpolateHelperFuncs, pipelines, and
// default / ternary, which switch the unknown-variable protection off for their whole expression.
func TestDoRunsHelpersAndPipelines(t *testing.T) {
	runDoCases(t, []doCase{
		{name: "upper on a known variable", input: "{{.a|upper}}", vars: map[string]string{"a": "x"}, want: "X"},
		{name: "upper with | glued to the helper", input: "{{.a |upper}}", vars: map[string]string{"a": "x"}, want: "X"},
		{name: "upper on a literal", input: `{{ "x" | upper }}`, want: "X"},
		{name: "a number literal", input: "{{ 1 }}", want: "1"},
		{name: "add with arguments on a known variable", input: "{{ .a | add 1 2 }}", vars: map[string]string{"a": "3"}, want: "6"},
		{name: "printf on a known variable", input: `{{ printf "%s" .a }}`, vars: map[string]string{"a": "x"}, want: "x"},
		{name: "a trailing pipe is ignored", input: "{{ .a | }}", vars: map[string]string{"a": "x"}, want: "x"},
		{name: "default on an empty value", input: `{{ .a | default "d" }}`, vars: map[string]string{"a": ""}, want: "d"},
		{name: "default on an unknown nested value", input: `{{ .u.x | default "d" }}`, vars: map[string]string{}, want: "d"},
		{name: "default with an unknown fallback renders empty", input: "{{ .u | default .v }}", want: ""},
		{name: "ternary on an unknown condition takes the false branch", input: `{{ .u | ternary "y" "n" }}`, want: "n"},
	})
}

// TestDoReturnsTemplateErrors characterises Do's two error returns before #2031 splits it: a parse failure quotes the
// input as rewritten by the protection step, an execution failure carries text/template's message.
func TestDoReturnsTemplateErrors(t *testing.T) {
	runDoCases(t, []doCase{
		// quirk: "{{" anywhere in the text is taken for a template, even with no closing braces
		{name: "an unclosed {{ in plain text is a parse error", input: "a {{ b", wantErr: `invalid template format "a {{ b": template: input:1: function "b" not defined`},
		{name: "an unclosed {{ fails even when the word is a variable", input: "a {{ b", vars: map[string]string{"b": "x"}, wantErr: `invalid template format "a {{ b": template: input:1: function "b" not defined`},
		{name: "empty braces", input: "{{}}", wantErr: `invalid template format "{{}}": template: input:1: missing value for command`},
		{name: "braces holding only a space", input: "{{ }}", wantErr: `invalid template format "{{ }}": template: input:1: missing value for command`},
		{name: "JSON with double braces", input: `{{"key": "value"}}`, wantErr: `invalid template format "{{"key": "value"}}": template: input:1: expected :=`},
		{name: "unbalanced parenthesis", input: "{{ .a ) }}", vars: map[string]string{"a": "x"}, wantErr: `invalid template format "{{ .a ) }}": template: input:1: unexpected right paren`},
		// the dash replacement only runs before " ", "}" and "|"
		{name: "a dashed name before a parenthesis is not rewritten", input: "{{ (.my-key) }}", vars: map[string]string{"my-key": "v"}, wantErr: `invalid template format "{{ (.my-key) }}": template: input:1: bad character U+002D '-'`},
		// default switches the protection off, so the unknown function reaches the parser
		{name: "an unknown function next to default", input: `{{ .u | nofn | default "d" }}`, wantErr: `invalid template format "{{ .u | nofn | default "d" }}": template: input:1: function "nofn" not defined`},
		// quirk: the protection wraps the expression in a Go string literal, which cannot hold a newline
		{name: "a protected expression spanning lines", input: "{{\n.u\n}}", wantErr: "invalid template format \"{{\"{{\n.u\n}}\"}}\": template: input:1: unterminated quoted string"},
		{name: "a known variable split over lines before a pipe", input: "{{ .a\n| upper }}", vars: map[string]string{"a": "x"}, wantErr: "invalid template format \"{{\"{{ .a\n| upper }}\"}}\": template: input:1: unterminated quoted string"},
		// quirk: the split on spaces cuts a quoted argument holding a space, so only part of it is escaped
		{name: "a protected expression with a quoted argument holding a space", input: `{{.u | replace "a b" "c"}}`, wantErr: `invalid template format "{{"{{.u | replace "a b" \"c\"}}"}}": template: input:1: unexpected "a" in operand`},
		{name: "a helper called without its arguments", input: "{{ .a | trunc }}", vars: map[string]string{"a": "x"}, wantErr: `failed to execute template: template: input:1:8: executing "input" at <trunc>: error calling trunc: missing params (expected: int, string)`},
		{name: "randAlphaNum with a negative count", input: "{{ randAlphaNum (-5) }}", wantErr: `failed to execute template: template: input:1:3: executing "input" at <randAlphaNum (-5)>: error calling randAlphaNum: randAlphaNumeric: randomstringutils illegal argument: Requested random string length -5 is less than 0.`},
		{name: "randAlpha with a negative count", input: "{{ randAlpha (-5) }}", wantErr: `failed to execute template: template: input:1:3: executing "input" at <randAlpha (-5)>: error calling randAlpha: randAlpha: randomstringutils illegal argument: Requested random string length -5 is less than 0.`},
		{name: "randASCII with a negative count", input: "{{ randASCII (-5) }}", wantErr: `failed to execute template: template: input:1:3: executing "input" at <randASCII (-5)>: error calling randASCII: randASCII: randomstringutils illegal argument: Requested random string length -5 is less than 0.`},
		{name: "randNumeric with a negative count", input: "{{ randNumeric (-5) }}", wantErr: `failed to execute template: template: input:1:3: executing "input" at <randNumeric (-5)>: error calling randNumeric: randNumeric: randomstringutils illegal argument: Requested random string length -5 is less than 0.`},
		{name: "a helper that panics", input: "{{ div 1 0 }}", wantErr: `failed to execute template: template: input:1:3: executing "input" at <div 1 0>: error calling div: runtime error: integer divide by zero`},
		// quirk: a parent of dotted names is a nested map, which default cannot take
		{name: "default on the parent of a dotted name", input: `{{.a | default "d"}}`, vars: map[string]string{"a.b": "x"}, wantErr: `failed to execute template: template: input:1:7: executing "input" at <default "d">: error calling default: reflect: Call using zero Value argument`},
		{name: "default on a field of a known leaf", input: `{{ .a.b | default "d" }}`, vars: map[string]string{"a": "x"}, wantErr: `failed to execute template: template: input:1:5: executing "input" at <.a.b>: can't evaluate field b in type interface {}`},
		{name: "default on a child of a known dotted leaf", input: `{{.a.b.c | default "d"}}`, vars: map[string]string{"a.b": "x"}, wantErr: `failed to execute template: template: input:1:4: executing "input" at <.a.b.c>: can't evaluate field c in type interface {}`},
	})
}

// unknownVariablesVars and unknownVariablesCases are shared by TestUnknownVariables and
// TestUnknownVariablesAgreesWithDo. A row with helper set uses a helper venom does not know: Do leaves such an
// expression as written, UnknownVariables reports variables only, so the two are not compared on it.
var unknownVariablesVars = map[string]string{"known": "1", "a.b": "2", "with-dash": "3", "venom.testcase": "tc"}

var unknownVariablesCases = []struct {
	name   string
	input  string
	want   []string
	helper bool
}{
	{name: "no braces", input: "plain text .unknown", want: nil},
	{name: "known", input: "{{.known}} {{.a.b}} {{.venom.testcase}}", want: nil},
	{name: "known with a dash", input: "{{.with-dash}}", want: nil},
	{name: "unknown", input: "x {{.nope}} y", want: []string{"nope"}},
	{name: "unknown with a dash is reported as written", input: "{{.not-here}}", want: []string{"not-here"}},
	{name: "several unknown, sorted and unique", input: "{{.zed}} {{.alpha}} {{.zed}} {{.known}} {{.a.c}}", want: []string{"a.c", "alpha", "zed"}},
	{name: "default is never flagged", input: "{{.nope | default \"x\"}}", want: nil},
	{name: "ternary is never flagged", input: "{{.nope | ternary \"a\" \"b\"}}", want: nil},
	{name: "unknown helper only", input: "{{ unknownhelper \"x\" }}", want: nil, helper: true},
	{name: "known variable with an unknown helper", input: "{{.known | nohelper}}", want: nil, helper: true},
}

func TestUnknownVariables(t *testing.T) {
	for _, tt := range unknownVariablesCases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UnknownVariables(tt.input, unknownVariablesVars)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestUnknownVariablesAgreesWithDo pins the contract of UnknownVariables: it reports a variable exactly when Do
// leaves its expression as written. A change to mustProtect that the helper does not follow fails here.
func TestUnknownVariablesAgreesWithDo(t *testing.T) {
	for _, tt := range unknownVariablesCases {
		if tt.helper {
			continue
		}
		t.Run(tt.name, func(t *testing.T) {
			out, err := Do(tt.input, unknownVariablesVars)
			require.NoError(t, err)
			reported, err := UnknownVariables(tt.input, unknownVariablesVars)
			require.NoError(t, err)

			leftAsWritten := strings.Contains(out, "{{")
			assert.Equal(t, len(reported) > 0, leftAsWritten, "Do output %q, reported %v", out, reported)
			for _, name := range reported {
				assert.Contains(t, out, "."+name, "an unknown variable stays in the text as written")
			}
		})
	}
}

// Scanned tells the expressions the protection can read from the ones it cannot
func TestScanned(t *testing.T) {
	assert.True(t, Scanned(`{{ .a | default "x" }}`))
	assert.False(t, Scanned(`{{ printf "%s" .a }}`))
	assert.False(t, Scanned(`{{ (.a) }}`))
}
