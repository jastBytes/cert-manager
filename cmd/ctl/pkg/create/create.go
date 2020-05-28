/*
Copyright 2020 The Jetstack cert-manager contributors.

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

package create

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

func NewCmdCreate(ioStreams genericclioptions.IOStreams, factory cmdutil.Factory) *cobra.Command {
	cmds := &cobra.Command{
		Use:   "create",
		Short: "Create something",
		Long:  `Create something e.g. CertificateRequest`,
	}
	//cmds.SetUsageTemplate(usageTemplate)

	// TODO: add flags

	cmds.AddCommand(NewCmdCreateCertficate(ioStreams, factory))

	return cmds
}
