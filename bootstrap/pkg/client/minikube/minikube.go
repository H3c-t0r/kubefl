/*
Copyright The Kubernetes Authors.

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

package minikube

import (
	"fmt"
	kftypes "github.com/kubeflow/kubeflow/bootstrap/pkg/apis/apps"
	kstypes "github.com/kubeflow/kubeflow/bootstrap/pkg/apis/apps/ksonnet/v1alpha1"
	/* DEBUG
	"github.com/kubeflow/kubeflow/bootstrap/pkg/client/ksonnet"
	-DEBUG */
	// NO_DEBUG
	log "github.com/sirupsen/logrus"
	// NO_DEBUG //
	"os/user"
	"strconv"
	"strings"
)

// Minikube implements KfApp Interface
// It includes the Ksonnet along with functionality needed for minikube
type Minikube struct {
	Ksonnet kftypes.KfApp
	//TODO add additional types required for minikube platform
}

func GetKfApp(options map[string]interface{}) kftypes.KfApp {
	/* DEBUG
	ksonnet := ksonnet.GetKfApp(options)
	-DEBUG */
	// NO_DEBUG
	options[string(kftypes.PLATFORM)] = "ksonnet"
	ksonnet, ksonnetErr := kftypes.LoadPlatform(options)
	if ksonnetErr != nil {
		log.Errorf("loadplatform failed for ksonnet: %v", ksonnetErr)
		return nil
	}
	options[string(kftypes.PLATFORM)] = "minikube"
	// NO_DEBUG //
	_minikubeapp := &Minikube{
		Ksonnet: ksonnet,
	}
	return _minikubeapp
}

func (minikube *Minikube) Apply(resources kftypes.ResourceEnum, options map[string]interface{}) error {
	ksApplyErr := minikube.Ksonnet.Apply(resources, options)
	if ksApplyErr != nil {
		return fmt.Errorf("minikube apply failed for ksonnet: %v", ksApplyErr)
	}
	//mount_local_fs
	//setup_tunnels
	return nil
}

func (minikube *Minikube) Delete(resources kftypes.ResourceEnum, options map[string]interface{}) error {
	ksDeleteErr := minikube.Ksonnet.Delete(resources, options)
	if ksDeleteErr != nil {
		return fmt.Errorf("minikube delete failed for ksonnet: %v", ksDeleteErr)
	}
	return nil
}

func (minikube *Minikube) generateKsApp(options map[string]interface{}) error {
	platform := options[string(kftypes.PLATFORM)].(string)
	mountLocal := false
	if options[string(kftypes.MOUNT_LOCAL)] != nil {
		mountLocal = options[string(kftypes.MOUNT_LOCAL)].(bool)
	}
	// remove Katib package and component
	kstypes.DefaultPackages = kstypes.RemoveItem(kstypes.DefaultPackages, "katib")
	kstypes.DefaultComponents = kstypes.RemoveItem(kstypes.DefaultComponents, "katib")
	kstypes.DefaultParameters["application"] = []kstypes.NameValue{
		{
			Name:  "components",
			Value: "[" + strings.Join(kstypes.QuoteItems(kstypes.DefaultComponents), ",") + "]",
		},
	}
	usr, err := user.Current()
	if err != nil {
		return fmt.Errorf("Could not get current user; error %v", err)
	}
	uid := usr.Uid
	gid := usr.Gid
	kstypes.DefaultParameters["jupyter"] = []kstypes.NameValue{
		{
			Name:  string(kftypes.PLATFORM),
			Value: platform,
		},
		{
			Name:  "accessLocalFs",
			Value: strconv.FormatBool(mountLocal),
		},
		{
			Name:  "disks",
			Value: "local-notebooks",
		},
		{
			Name:  "notebookUid",
			Value: uid,
		},
		{
			Name:  "notebookGid",
			Value: gid,
		},
	}
	kstypes.DefaultParameters["ambassador"] = []kstypes.NameValue{
		{
			Name:  string(kftypes.PLATFORM),
			Value: platform,
		},
		{
			Name:  "replicas",
			Value: "1",
		},
	}
	ksGenerateErr := minikube.Ksonnet.Generate(kftypes.ALL, options)
	if ksGenerateErr != nil {
		return fmt.Errorf("minikube generate failed for ksonnet: %v", ksGenerateErr)
	}
	return nil
}

func (minikube *Minikube) Generate(resources kftypes.ResourceEnum, options map[string]interface{}) error {
	switch resources {
	case kftypes.ALL:
		fallthrough
	case kftypes.K8S:
		ksErr := minikube.generateKsApp(options)
		if ksErr != nil {
			return fmt.Errorf("could not generate kssonnet under %v Error: %v", kstypes.KsName, ksErr)
		}
	case kftypes.PLATFORM:
	}
	return nil
}

func (minikube *Minikube) Init(options map[string]interface{}) error {
	ksInitErr := minikube.Ksonnet.Init(options)
	if ksInitErr != nil {
		return fmt.Errorf("minikube init failed for ksonnet: %v", ksInitErr)
	}
	return nil
}
