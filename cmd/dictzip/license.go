// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"

	"github.com/urfave/cli/v2"
	"sigs.k8s.io/release-utils/version"
)

func printVersion(c *cli.Context) error {
	versionInfo := version.GetVersionInfo()
	_, err := fmt.Fprintf(c.App.Writer, `%s %s
Copyright 2024 Google LLC

%s
`, c.App.Name, versionInfo.GitVersion, versionInfo.String())
	if err != nil {
		return fmt.Errorf("%w: %w", ErrDictzip, err)
	}
	return nil
}

func printLicense(c *cli.Context) error {
	err := printVersion(c)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(c.App.Writer, `    Licensed under the Apache License, Version 2.0 (the "License");
    you may not use this file except in compliance with the License.
    You may obtain a copy of the License at
    
         http://www.apache.org/licenses/LICENSE-2.0
    
    Unless required by applicable law or agreed to in writing, software
    distributed under the License is distributed on an "AS IS" BASIS,
    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    See the License for the specific language governing permissions and
    limitations under the License.`)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrDictzip, err)
	}
	return nil
}
