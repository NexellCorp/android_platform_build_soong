// Copyright 2018 Google Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package genrule

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"android/soong/android"
)

var buildDir string

func setUp() {
	var err error
	buildDir, err = ioutil.TempDir("", "soong_java_test")
	if err != nil {
		panic(err)
	}
}

func tearDown() {
	os.RemoveAll(buildDir)
}

func TestMain(m *testing.M) {
	run := func() int {
		setUp()
		defer tearDown()

		return m.Run()
	}

	os.Exit(run())
}

func testContext(config android.Config, bp string,
	fs map[string][]byte) *android.TestContext {

	ctx := android.NewTestArchContext()
	ctx.RegisterModuleType("filegroup", android.ModuleFactoryAdaptor(android.FileGroupFactory))
	ctx.RegisterModuleType("genrule", android.ModuleFactoryAdaptor(GenRuleFactory))
	ctx.RegisterModuleType("tool", android.ModuleFactoryAdaptor(toolFactory))
	ctx.Register()

	bp += `
		tool {
			name: "tool",
		}

		filegroup {
			name: "tool_files",
			srcs: [
				"tool_file1",
				"tool_file2",
			],
		}

		filegroup {
			name: "1tool_file",
			srcs: [
				"tool_file1",
			],
		}

		filegroup {
			name: "ins",
			srcs: [
				"in1",
				"in2",
			],
		}

		filegroup {
			name: "1in",
			srcs: [
				"in1",
			],
		}

		filegroup {
			name: "empty",
		}
	`

	mockFS := map[string][]byte{
		"Android.bp": []byte(bp),
		"tool":       nil,
		"tool_file1": nil,
		"tool_file2": nil,
		"in1":        nil,
		"in2":        nil,
	}

	for k, v := range fs {
		mockFS[k] = v
	}

	ctx.MockFileSystem(mockFS)

	return ctx
}

func TestGenruleCmd(t *testing.T) {
	testcases := []struct {
		name string
		prop string

		err    string
		expect string
	}{
		{
			name: "empty location tool",
			prop: `
				tools: ["tool"],
				out: ["out"],
				cmd: "$(location) > $(out)",
			`,
			expect: "out/tool > __SBOX_OUT_FILES__",
		},
		{
			name: "empty location tool file",
			prop: `
				tool_files: ["tool_file1"],
				out: ["out"],
				cmd: "$(location) > $(out)",
			`,
			expect: "tool_file1 > __SBOX_OUT_FILES__",
		},
		{
			name: "empty location tool file fg",
			prop: `
				tool_files: [":1tool_file"],
				out: ["out"],
				cmd: "$(location) > $(out)",
			`,
			expect: "tool_file1 > __SBOX_OUT_FILES__",
		},
		{
			name: "empty location tool and tool file",
			prop: `
				tools: ["tool"],
				tool_files: ["tool_file1"],
				out: ["out"],
				cmd: "$(location) > $(out)",
			`,
			expect: "out/tool > __SBOX_OUT_FILES__",
		},
		{
			name: "tool",
			prop: `
				tools: ["tool"],
				out: ["out"],
				cmd: "$(location tool) > $(out)",
			`,
			expect: "out/tool > __SBOX_OUT_FILES__",
		},
		{
			name: "tool file",
			prop: `
				tool_files: ["tool_file1"],
				out: ["out"],
				cmd: "$(location tool_file1) > $(out)",
			`,
			expect: "tool_file1 > __SBOX_OUT_FILES__",
		},
		{
			name: "tool file fg",
			prop: `
				tool_files: [":1tool_file"],
				out: ["out"],
				cmd: "$(location tool_file1) > $(out)",
			`,
			expect: "tool_file1 > __SBOX_OUT_FILES__",
		},
		{
			name: "tool files",
			prop: `
				tool_files: [":tool_files"],
				out: ["out"],
				cmd: "$(location tool_file1) $(location tool_file2) > $(out)",
			`,
			expect: "tool_file1 tool_file2 > __SBOX_OUT_FILES__",
		},
		{
			name: "in1",
			prop: `
				srcs: ["in1"],
				out: ["out"],
				cmd: "cat $(in) > $(out)",
			`,
			expect: "cat ${in} > __SBOX_OUT_FILES__",
		},
		{
			name: "in1 fg",
			prop: `
				srcs: [":1in"],
				out: ["out"],
				cmd: "cat $(in) > $(out)",
			`,
			expect: "cat ${in} > __SBOX_OUT_FILES__",
		},
		{
			name: "ins",
			prop: `
				srcs: ["in1", "in2"],
				out: ["out"],
				cmd: "cat $(in) > $(out)",
			`,
			expect: "cat ${in} > __SBOX_OUT_FILES__",
		},
		{
			name: "ins fg",
			prop: `
				srcs: [":ins"],
				out: ["out"],
				cmd: "cat $(in) > $(out)",
			`,
			expect: "cat ${in} > __SBOX_OUT_FILES__",
		},
		{
			name: "outs",
			prop: `
				out: ["out", "out2"],
				cmd: "echo foo > $(out)",
			`,
			expect: "echo foo > __SBOX_OUT_FILES__",
		},
		{
			name: "depfile",
			prop: `
				out: ["out"],
				depfile: true,
				cmd: "echo foo > $(out) && touch $(depfile)",
			`,
			expect: "echo foo > __SBOX_OUT_FILES__ && touch __SBOX_DEPFILE__",
		},
		{
			name: "gendir",
			prop: `
				out: ["out"],
				cmd: "echo foo > $(genDir)/foo && cp $(genDir)/foo $(out)",
			`,
			expect: "echo foo > __SBOX_OUT_DIR__/foo && cp __SBOX_OUT_DIR__/foo __SBOX_OUT_FILES__",
		},

		{
			name: "error empty location",
			prop: `
				out: ["out"],
				cmd: "$(location) > $(out)",
			`,
			err: "at least one `tools` or `tool_files` is required if $(location) is used",
		},
		{
			name: "error location",
			prop: `
				out: ["out"],
				cmd: "echo foo > $(location missing)",
			`,
			err: `unknown location label "missing"`,
		},
		{
			name: "error variable",
			prop: `
					out: ["out"],
					srcs: ["in1"],
					cmd: "echo $(foo) > $(out)",
			`,
			err: `unknown variable '$(foo)'`,
		},
		{
			name: "error depfile",
			prop: `
				out: ["out"],
				cmd: "echo foo > $(out) && touch $(depfile)",
			`,
			err: "$(depfile) used without depfile property",
		},
		{
			name: "error no depfile",
			prop: `
				out: ["out"],
				depfile: true,
				cmd: "echo foo > $(out)",
			`,
			err: "specified depfile=true but did not include a reference to '${depfile}' in cmd",
		},
		{
			name: "error no out",
			prop: `
				cmd: "echo foo > $(out)",
			`,
			err: "must have at least one output file",
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			config := android.TestArchConfig(buildDir, nil)
			bp := "genrule {\n"
			bp += "name: \"gen\",\n"
			bp += test.prop
			bp += "}\n"

			ctx := testContext(config, bp, nil)

			_, errs := ctx.ParseFileList(".", []string{"Android.bp"})
			if errs == nil {
				_, errs = ctx.PrepareBuildActions(config)
			}
			if errs == nil && test.err != "" {
				t.Fatalf("want error %q, got no error", test.err)
			} else if errs != nil && test.err == "" {
				android.FailIfErrored(t, errs)
			} else if test.err != "" {
				if len(errs) != 1 {
					t.Errorf("want 1 error, got %d errors:", len(errs))
					for _, err := range errs {
						t.Errorf("   %s", err.Error())
					}
					t.FailNow()
				}
				if !strings.Contains(errs[0].Error(), test.err) {
					t.Fatalf("want %q, got %q", test.err, errs[0].Error())
				}
				return
			}

			gen := ctx.ModuleForTests("gen", "").Module().(*Module)
			if gen.rawCommand != "'"+test.expect+"'" {
				t.Errorf("want %q, got %q", test.expect, gen.rawCommand)
			}
		})
	}

}

type testTool struct {
	android.ModuleBase
	outputFile android.Path
}

func toolFactory() android.Module {
	module := &testTool{}
	android.InitAndroidArchModule(module, android.HostSupported, android.MultilibFirst)
	return module
}

func (t *testTool) DepsMutator(ctx android.BottomUpMutatorContext) {}

func (t *testTool) GenerateAndroidBuildActions(ctx android.ModuleContext) {
	t.outputFile = android.PathForTesting("out", ctx.ModuleName())
}

func (t *testTool) HostToolPath() android.OptionalPath {
	return android.OptionalPathForPath(t.outputFile)
}

var _ HostToolProvider = (*testTool)(nil)