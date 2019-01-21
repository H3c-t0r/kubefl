// Copyright © 2018 NAME HERE <EMAIL ADDRESS>
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

package cmd

import (
	"github.com/kubeflow/kubeflow/bootstrap/pkg/client/kfapi/typed/apps/v1alpha1"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// generateCmd represents the generate command
var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a kubeflow application using <name>.yaml.",
	Long:  `Generate a kubeflow application using <name>.yaml.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.SetLevel(log.WarnLevel)
		kfApi, kfApiErr := v1alpha1.NewKfApiWithConfig(kfctlConfig, kfctlEnv)
		if kfApiErr != nil {
			log.Errorf("couldn't create KfApi: %v", kfApiErr)
			return
		}
		host, k8sSpec, err := ServerVersion()
		if err != nil {
			log.Errorf("couldn't get server version: %v", err)
			return
		}
		namespace := kfctlEnv.GetString("K8S_NAMESPACE")
		initErr := kfApi.Init("default", k8sSpec, host, namespace)
		if initErr != nil {
			log.Errorf("couldn't initialize KfApi: %v", initErr)
			return
		}
		for _, registry := range kfApi.Application().Spec.App.Registries {
			registryAddErr := kfApi.RegistryAdd(registry)
			if registryAddErr != nil {
				log.Errorf("couldn't add registry %v. Error: %v", registry.Name, registryAddErr)
				return
			}
		}
		for _, pkg := range kfApi.Application().Spec.App.Packages {
			packageAddErr := kfApi.PkgInstall(pkg)
			if packageAddErr != nil {
				log.Errorf("couldn't add registry %v. Error: %v", pkg.Name, packageAddErr)
				return
			}
		}
		for _, component := range kfApi.Application().Spec.App.Components {
			componentAddErr := kfApi.ComponentAdd(component, []string{})
			if componentAddErr != nil {
				log.Errorf("couldn't add registry %v. Error: %v", component.Name, componentAddErr)
				return
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(generateCmd)
}
