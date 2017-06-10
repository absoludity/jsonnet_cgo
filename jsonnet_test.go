package jsonnet_test

import jsonnet "github.com/mmikulicic/jsonnet_cgo"

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"text/template"
)

// Demo for the README.
func Test_Demo(t *testing.T) {
	vm := jsonnet.Make()
	vm.ExtVar("color", "purple")

	x, err := vm.EvaluateSnippet(`Test_Demo`, `"dark " + std.extVar("color")`)
	if err != nil {
		panic(err)
	}
	if x != "\"dark purple\"\n" {
		panic("fail: we got " + x)
	}
	vm.Destroy()
}

// importFunc returns a couple of hardwired responses.
func importFunc(base, rel string) (result string, path string, err error) {
	if rel == "alien.conf" {
		return `{ type: "alien", origin: "Ork", name: "Mork" }`, "alien.conf", nil
	}
	if rel == "human.conf" {
		return `{ type: "human", origin: "Earth", name: "Mendy" }`, "human.conf", nil
	}
	return "", "", errors.New(fmt.Sprintf("Cannot import %q", rel))
}

// check there is no err, and a == b.
func check(t *testing.T, err error, a, b string) {
	if err != nil {
		t.Errorf("got error: %q", err.Error())
	}
	if a != b {
		t.Errorf("got %q but wanted %q", a, b)
	}
}

func Test_Simple(t *testing.T) {

	// Each time there's a new version, this will force an update to this code.
	check(t, nil, jsonnet.Version(), `v0.9.3`)

	vm := jsonnet.Make()
	vm.TlaVar("color", "purple")
	vm.TlaVar("size", "XXL")
	vm.TlaCode("gooselevel", "1234 * 10 + 5")
	vm.ExtVar("color", "purple")
	vm.ExtVar("size", "XXL")
	vm.ExtCode("gooselevel", "1234 * 10 + 5")
	vm.ImportCallback(importFunc)

	x, err := vm.EvaluateSnippet(`test1`, `20 + 22`)
	check(t, err, x, `42`+"\n")
	x, err = vm.EvaluateSnippet(`test2`, `function(color, size, gooselevel) color`)
	check(t, err, x, `"purple"`+"\n")
	x, err = vm.EvaluateSnippet(`test2`, `std.extVar("color")`)
	check(t, err, x, `"purple"`+"\n")
	vm.StringOutput(true)
	x, err = vm.EvaluateSnippet(`test2`, `"whee"`)
	check(t, err, x, `whee`+"\n")
	vm.StringOutput(false)
	x, err = vm.EvaluateSnippet(`test3`, `
    local a = import "alien.conf";
    local b = import "human.conf";
    a.name + b.name
    `)
	check(t, err, x, `"MorkMendy"`+"\n")
	x, err = vm.EvaluateSnippet(`test4`, `
    local a = import "alien.conf";
    local b = a { type: "fictitious" };
    b.type + b.name
    `)
	check(t, err, x, `"fictitiousMork"`+"\n")
}

func Test_FileScript(t *testing.T) {
	vm := jsonnet.Make()
	x, err := vm.EvaluateFile("test2.j")
	check(t, err, x, `{
   "awk": "/usr/bin/awk",
   "shell": "/bin/csh"
}
`)
}

func Test_Misc(t *testing.T) {
	vm := jsonnet.Make()

	vm.MaxStack(10)
	vm.MaxTrace(10)
	vm.GcMinObjects(10)
	vm.GcGrowthTrigger(2.0)

	x, err := vm.EvaluateSnippet("Misc", `
    local a = import "test2.j";
    a.awk + a.shell`)
	check(t, err, x, `"/usr/bin/awk/bin/csh"`+"\n")
}

func Test_FormatFile(t *testing.T) {
	f, err := ioutil.TempFile("", "jsonnet-fmt-test")
	if err != nil {
		t.Fatal(err)
	}
	filename := f.Name()
	defer func() {
		f.Close()
		os.Remove(filename)
	}()

	data := `{
    "quoted": "keys",
    "notevaluated": 20 + 22,
    "trailing": "comma",}
`
	if err := ioutil.WriteFile(filename, []byte(data), 0644); err != nil {
		t.Fatalf("WriteFile %s: %v", filename, err)
	}

	vm := jsonnet.Make()
	result, err := vm.FormatFile(filename)

	check(t, err, result, `{
    quoted: "keys",
    notevaluated: 20 + 22,
    trailing: "comma" }
`)
}

func Test_FormatSnippet(t *testing.T) {
	data := `{
    "quoted": "keys",
    "notevaluated": 20 + 22,
    "trailing": "comma",}
`

	vm := jsonnet.Make()
	result, err := vm.FormatSnippet("testfoo", data)

	check(t, err, result, `{
    quoted: "keys",
    notevaluated: 20 + 22,
    trailing: "comma" }
`)
}

func Test_FormatIndent(t *testing.T) {
	data := `{
  "quoted": "keys",
 "notevaluated": 20 + 22,
   "trailing": "comma",}
`

	vm := jsonnet.Make()
	vm.FormatIndent(1)
	result, err := vm.FormatSnippet("testfoo", data)

	check(t, err, result, `{
 quoted: "keys",
 notevaluated: 20 + 22,
 trailing: "comma" }
`)
}

func Test_NativeCallbackSimple(t *testing.T) {
	do_foo := func(params ...string) (string, error) {
		return "Foo", nil
	}
	do_bar := func(params ...string) (string, error) {
		return "Bar", nil
	}

	vm := jsonnet.Make()
	vm.NativeCallback("call_it_foo", do_foo, []string{"param"})
	vm.NativeCallback("call_it_bar", do_bar, []string{"param"})
	data := `local call_it_foo(vars) = std.native("call_it_foo")(vars);
	local call_it_bar(vars) = std.native("call_it_bar")(vars);
	{
		footest: call_it_foo(""),
		bartest: call_it_bar(""),
	}`

	result, err := vm.EvaluateSnippet("testfoo", data)

	check(t, err, result, `{
   "bartest": "Bar",
   "footest": "Foo"
}
`)
}

// expandTemplate is an example native function which takes multiple params.
func expandTemplate(params ...string) (string, error) {
	templateText := params[0]
	contextJSON := params[1]
	var context interface{}
	if err := json.Unmarshal([]byte(contextJSON), &context); err != nil {
		return "", err
	}

	tmpl, err := template.New("Test").Parse(templateText)
	if err != nil {
		return "", err
	}

	var result bytes.Buffer
	if err := tmpl.Execute(&result, context); err != nil {
		return "", err
	}
	return result.String(), nil
}

func Test_NativeCallbackStructuredInput(t *testing.T) {

	vm := jsonnet.Make()
	vm.NativeCallback("expand_template", expandTemplate, []string{"template", "contextJSON"})
	data := `local expand_template(template, vars) = std.native("expand_template")(
		template, std.toString(vars));
		local template_1 = "Hello {{ .FirstName }} {{ .LastName }}.";
		local template_2 = "Guten Tag {{ .FirstName }} {{ .LastName }}.";
		{
			footest: expand_template(template_1, {
				FirstName: "Name1",
				LastName: "Name2",
			}),
			bartest: expand_template(template_2, {
				FirstName: "Name3",
				LastName: "Name4",
			}),
		}`

	result, err := vm.EvaluateSnippet("testfoo", data)

	check(t, err, result, `{
   "bartest": "Guten Tag Name3 Name4.",
   "footest": "Hello Name1 Name2."
}
`)
}

func Test_NativeCallbackError(t *testing.T) {
	vm := jsonnet.Make()
	vm.NativeCallback("expand_template", expandTemplate, []string{"template", "contextJSON"})
	data := `local expand_template(template, vars) = std.native("expand_template")(
		template, std.toString(vars));
		{
			footest: expand_template("not }} a valid {{ template", {}),
		}`

	result, err := vm.EvaluateSnippet("testfoo", data)

	if result != "" {
		t.Errorf("Unexpected result on error. Want: %q, Got: %q", "", result)
	}

	if err == nil {
		t.Errorf("Expected error but gone nil")
	}
}
