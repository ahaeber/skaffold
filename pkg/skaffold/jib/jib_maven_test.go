/*
Copyright 2019 The Skaffold Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package jib

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/schema/latest"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/util"
	"github.com/GoogleContainerTools/skaffold/testutil"
	"github.com/pkg/errors"
)

func TestMavenWrapperDefinition(t *testing.T) {
	testutil.CheckDeepEqual(t, "mvn", MavenCommand.Executable)
	testutil.CheckDeepEqual(t, "mvnw", MavenCommand.Wrapper)
}

func TestGetDependenciesMaven(t *testing.T) {
	tmpDir, cleanup := testutil.NewTempDir(t)
	defer cleanup()

	tmpDir.Write("dep1", "")
	tmpDir.Write("dep2", "")

	dep1 := tmpDir.Path("dep1")
	dep2 := tmpDir.Path("dep2")

	ctx := context.Background()

	var tests = []struct {
		description string
		stdout      string
		expected    []string
		err         error
	}{
		{
			description: "success",
			stdout:      fmt.Sprintf("%s\n%s\n\n\n", dep1, dep2),
			expected:    []string{dep1, dep2},
		},
		{
			description: "failure",
			stdout:      "",
			err:         errors.New("error"),
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			defer func(c util.Command) { util.DefaultExecCommand = c }(util.DefaultExecCommand)
			util.DefaultExecCommand = testutil.NewFakeCmd(t).WithRunOutErr(
				strings.Join(getCommandMaven(ctx, tmpDir.Root(), &latest.JibMavenArtifact{}).Args, " "),
				test.stdout,
				test.err,
			)

			deps, err := GetDependenciesMaven(ctx, tmpDir.Root(), &latest.JibMavenArtifact{})
			if test.err != nil {
				testutil.CheckErrorAndDeepEqual(t, true, err, "getting jibMaven dependencies: "+test.err.Error(), err.Error())
			} else {
				testutil.CheckDeepEqual(t, test.expected, deps)
			}
		})
	}
}

func TestGetCommandMaven(t *testing.T) {
	ctx := context.Background()
	var tests = []struct {
		description      string
		jibMavenArtifact latest.JibMavenArtifact
		filesInWorkspace []string
		expectedCmd      func(workspace string) *exec.Cmd
	}{
		{
			description:      "maven no profile",
			jibMavenArtifact: latest.JibMavenArtifact{},
			filesInWorkspace: []string{},
			expectedCmd: func(workspace string) *exec.Cmd {
				return MavenCommand.CreateCommand(ctx, workspace, []string{"--non-recursive", "jib:_skaffold-files", "--quiet"})
			},
		},
		{
			description: "maven with extra flags",
			jibMavenArtifact: latest.JibMavenArtifact{
				BuildArgs: []string{"-DskipTests", "-x"},
			},
			filesInWorkspace: []string{},
			expectedCmd: func(workspace string) *exec.Cmd {
				return MavenCommand.CreateCommand(ctx, workspace, []string{"-DskipTests", "-x", "--non-recursive", "jib:_skaffold-files", "--quiet"})
			},
		},
		{
			description:      "maven with profile",
			jibMavenArtifact: latest.JibMavenArtifact{Profile: "profile"},
			filesInWorkspace: []string{},
			expectedCmd: func(workspace string) *exec.Cmd {
				return MavenCommand.CreateCommand(ctx, workspace, []string{"--activate-profiles", "profile", "--non-recursive", "jib:_skaffold-files", "--quiet"})
			},
		},
		{
			description:      "maven with wrapper no profile",
			jibMavenArtifact: latest.JibMavenArtifact{},
			filesInWorkspace: []string{"mvnw", "mvnw.bat"},
			expectedCmd: func(workspace string) *exec.Cmd {
				return MavenCommand.CreateCommand(ctx, workspace, []string{"--non-recursive", "jib:_skaffold-files", "--quiet"})
			},
		},
		{
			description:      "maven with wrapper no profile",
			jibMavenArtifact: latest.JibMavenArtifact{},
			filesInWorkspace: []string{"mvnw", "mvnw.cmd"},
			expectedCmd: func(workspace string) *exec.Cmd {
				return MavenCommand.CreateCommand(ctx, workspace, []string{"--non-recursive", "jib:_skaffold-files", "--quiet"})
			},
		},
		{
			description:      "maven with wrapper and profile",
			jibMavenArtifact: latest.JibMavenArtifact{Profile: "profile"},
			filesInWorkspace: []string{"mvnw", "mvnw.bat"},
			expectedCmd: func(workspace string) *exec.Cmd {
				return MavenCommand.CreateCommand(ctx, workspace, []string{"--activate-profiles", "profile", "--non-recursive", "jib:_skaffold-files", "--quiet"})
			},
		},
		{
			description:      "maven with multi-modules",
			jibMavenArtifact: latest.JibMavenArtifact{Module: "module"},
			filesInWorkspace: []string{"mvnw", "mvnw.bat"},
			expectedCmd: func(workspace string) *exec.Cmd {
				return MavenCommand.CreateCommand(ctx, workspace, []string{"--projects", "module", "--also-make", "jib:_skaffold-files", "--quiet"})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			tmpDir, cleanup := testutil.NewTempDir(t)
			defer cleanup()

			for _, file := range test.filesInWorkspace {
				tmpDir.Write(file, "")
			}

			cmd := getCommandMaven(ctx, tmpDir.Root(), &test.jibMavenArtifact)
			expectedCmd := test.expectedCmd(tmpDir.Root())
			testutil.CheckDeepEqual(t, expectedCmd.Path, cmd.Path)
			testutil.CheckDeepEqual(t, expectedCmd.Args, cmd.Args)
			testutil.CheckDeepEqual(t, expectedCmd.Dir, cmd.Dir)
		})
	}
}

func TestGenerateMavenArgs(t *testing.T) {
	var testCases = []struct {
		in        latest.JibMavenArtifact
		skipTests bool
		out       []string
	}{
		{latest.JibMavenArtifact{}, false, []string{"--non-recursive", "prepare-package", "jib:goal", "-Dimage=image"}},
		{latest.JibMavenArtifact{}, true, []string{"--non-recursive", "-DskipTests=true", "prepare-package", "jib:goal", "-Dimage=image"}},
		{latest.JibMavenArtifact{Profile: "profile"}, false, []string{"--activate-profiles", "profile", "--non-recursive", "prepare-package", "jib:goal", "-Dimage=image"}},
		{latest.JibMavenArtifact{Profile: "profile"}, true, []string{"--activate-profiles", "profile", "--non-recursive", "-DskipTests=true", "prepare-package", "jib:goal", "-Dimage=image"}},
		{latest.JibMavenArtifact{Module: "module"}, false, []string{"--projects", "module", "--also-make", "package", "-Dimage=image"}},
		{latest.JibMavenArtifact{Module: "module"}, true, []string{"--projects", "module", "--also-make", "-DskipTests=true", "package", "-Dimage=image"}},
		{latest.JibMavenArtifact{Module: "module", Profile: "profile"}, false, []string{"--activate-profiles", "profile", "--projects", "module", "--also-make", "package", "-Dimage=image"}},
		{latest.JibMavenArtifact{Module: "module", Profile: "profile"}, true, []string{"--activate-profiles", "profile", "--projects", "module", "--also-make", "-DskipTests=true", "package", "-Dimage=image"}},
	}

	for _, tt := range testCases {
		args := GenerateMavenArgs("goal", "image", &tt.in, tt.skipTests)

		testutil.CheckDeepEqual(t, tt.out, args)
	}
}
