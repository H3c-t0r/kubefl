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

package ksonnet

import (
	"fmt"
	"github.com/cenkalti/backoff"
	"github.com/ghodss/yaml"
	gogetter "github.com/hashicorp/go-getter"
	"github.com/ksonnet/ksonnet/pkg/actions"
	"github.com/ksonnet/ksonnet/pkg/app"
	"github.com/ksonnet/ksonnet/pkg/client"
	"github.com/ksonnet/ksonnet/pkg/component"
	kftypes "github.com/kubeflow/kubeflow/bootstrap/pkg/apis/apps"
	kstypes "github.com/kubeflow/kubeflow/bootstrap/pkg/apis/apps/ksonnet/v1alpha1"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"io/ioutil"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Ksonnet implements the KfApp Interface
type KsApp struct {
	// ksonnet root name
	KsName string
	// ksonnet env name
	KsEnvName string
	KApp      app.App
	// kstypes.Ksonnet is autogenerated similar to k8 types
	// It has a Spec member of type KsonnetSpec that holds common fields like platform, version, repo
	// components, packages, parameters. This information is used to generate a ksonnet app.
	// It is not persisted into kfctl's app.yaml.
	KsApp *kstypes.Ksonnet
}

func GetKfApp(options map[string]interface{}) kftypes.KfApp {
	_kfapp := &KsApp{
		KsName:    kstypes.KsName,
		KsEnvName: kstypes.KsEnvName,
		KsApp: &kstypes.Ksonnet{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Ksonnet",
				APIVersion: "ksonnet.apps.kubeflow.org/v1alpha1",
			},
		},
	}
	_kfapp.KsApp.Spec.Platform = options[string(kftypes.PLATFORM)].(string)
	if options[string(kftypes.APPNAME)] != nil {
		_kfapp.KsApp.Name = options[string(kftypes.APPNAME)].(string)
	}
	if options[string(kftypes.APPDIR)] != nil {
		_kfapp.KsApp.Spec.AppDir = options[string(kftypes.APPDIR)].(string)
	}
	if options[string(kftypes.KAPP)] != nil {
		_kfapp.KApp = options[string(kftypes.KAPP)].(app.App)
	}
	if options[string(kftypes.NAMESPACE)] != nil {
		namespace := options[string(kftypes.NAMESPACE)].(string)
		_kfapp.KsApp.Namespace = namespace
	}
	if options[string(kftypes.REPO)] != nil {
		kubeflowRepo := options[string(kftypes.REPO)].(string)
		re := regexp.MustCompile(`(^\$GOPATH)(.*$)`)
		goPathVar := os.Getenv("GOPATH")
		if goPathVar != "" {
			kubeflowRepo = re.ReplaceAllString(kubeflowRepo, goPathVar+`$2`)
		}
		_kfapp.KsApp.Spec.Repo = path.Join(kubeflowRepo, "kubeflow")
	}
	if options[string(kftypes.VERSION)] != nil {
		kubeflowVersion := options[string(kftypes.VERSION)].(string)
		_kfapp.KsApp.Spec.Version = kubeflowVersion
	}
	if options[string(kftypes.DATA)] != nil {
		dat := options[string(kftypes.DATA)].([]byte)
		specErr := yaml.Unmarshal(dat, _kfapp.KsApp)
		if specErr != nil {
			log.Errorf("couldn't unmarshal Ksonnet. Error: %v", specErr)
		}
	}
	return _kfapp
}

func (ksApp *KsApp) Apply(resources kftypes.ResourceEnum, options map[string]interface{}) error {
	host, _, err := kftypes.ServerVersion()
	if err != nil {
		return fmt.Errorf("couldn't get server version: %v", err)
	}
	cli, cliErr := kftypes.GetClientOutOfCluster()
	if cliErr != nil {
		return fmt.Errorf("couldn't create client Error: %v", cliErr)
	}
	envSetErr := ksApp.envSet(kstypes.KsEnvName, host)
	if envSetErr != nil {
		return fmt.Errorf("couldn't create ksonnet env %v Error: %v", kstypes.KsEnvName, envSetErr)
	}
	//ks param set application name ${DEPLOYMENT_NAME}
	name := ksApp.KsApp.Name
	paramSetErr := ksApp.paramSet("application", "name", name)
	if paramSetErr != nil {
		return fmt.Errorf("couldn't set application component's name to %v Error: %v", name, paramSetErr)
	}
	namespace := ksApp.KsApp.ObjectMeta.Namespace
	log.Infof(string(kftypes.NAMESPACE)+": %v", namespace)
	_, nsMissingErr := cli.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{})
	if nsMissingErr != nil {
		nsSpec := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
		_, nsErr := cli.CoreV1().Namespaces().Create(nsSpec)
		if nsErr != nil {
			return fmt.Errorf("couldn't create "+string(kftypes.NAMESPACE)+" %v Error: %v", namespace, nsErr)
		}
	}
	clientConfig, clientConfigErr := kftypes.GetClientConfig()
	if clientConfigErr != nil {
		return fmt.Errorf("couldn't load client config Error: %v", clientConfigErr)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("could not get current directory %v", err)
	}
	if cwd != ksApp.KsApp.Spec.AppDir {
		err = os.Chdir(ksApp.KsApp.Spec.AppDir)
		if err != nil {
			return fmt.Errorf("could not change directory to %v Error %v", ksApp.KsApp.Spec.AppDir, err)
		}
	}
	applyErr := ksApp.applyComponent([]string{"metacontroller"}, clientConfig)
	if applyErr != nil {
		return fmt.Errorf("couldn't create metacontroller component Error: %v", applyErr)
	}
	applyErr = ksApp.applyComponent([]string{"application"}, clientConfig)
	if applyErr != nil {
		return fmt.Errorf("couldn't create application component Error: %v", applyErr)
	}
	return nil
}

func (ksApp *KsApp) applyComponent(components []string, cfg *clientcmdapi.Config) error {
	applyOptions := map[string]interface{}{
		actions.OptionApp: ksApp.KApp,
		actions.OptionClientConfig: &client.Config{
			Overrides: &clientcmd.ConfigOverrides{},
			Config:    clientcmd.NewDefaultClientConfig(*cfg, &clientcmd.ConfigOverrides{}),
		},
		actions.OptionComponentNames: components,
		actions.OptionCreate:         true,
		actions.OptionDryRun:         false,
		actions.OptionEnvName:        kstypes.KsEnvName,
		actions.OptionGcTag:          "gc-tag",
		actions.OptionSkipGc:         true,
	}
	bo := backoff.WithMaxRetries(backoff.NewConstantBackOff(5*time.Second), 6)
	doneApply := make(map[string]bool)
	err := backoff.Retry(func() error {
		for _, comp := range components {
			if _, ok := doneApply[comp]; ok {
				continue
			}
			applyOptions[actions.OptionComponentNames] = []string{comp}
			err := actions.RunApply(applyOptions)
			if err == nil {
				log.Infof("Component %v apply succeeded", comp)
				doneApply[comp] = true
			} else {
				log.Errorf("(Will retry) Component %v apply failed; Error: %v", comp, err)
			}
		}
		if len(doneApply) == len(components) {
			return nil
		}
		return fmt.Errorf("%v failed components in last try", len(components)-len(doneApply))
	}, bo)
	if err != nil {
		log.Errorf("components apply failed; Error: %v", err)
	} else {
		log.Infof("All components apply succeeded")
	}
	return err

}

func (ksApp *KsApp) componentAdd(component kstypes.KsComponent, args []string) error {
	componentPath := filepath.Join(ksApp.ksRoot(), "components", component.Name+".jsonnet")
	componentArgs := make([]string, 0)
	componentArgs = append(componentArgs, component.Prototype)
	componentArgs = append(componentArgs, component.Name)
	if args != nil && len(args) > 0 {
		componentArgs = append(componentArgs, args[0:]...)
	}
	if exists, _ := afero.Exists(afero.NewOsFs(), componentPath); !exists {
		log.Infof("Creating Component: %v ...", component.Name)
		err := actions.RunPrototypeUse(map[string]interface{}{
			actions.OptionAppRoot:   ksApp.ksRoot(),
			actions.OptionArguments: componentArgs,
		})
		if err != nil {
			return fmt.Errorf("there was a problem adding component %v: %v", component.Name, err)
		}
	} else {
		log.Infof("Component %v already exists", component.Name)
	}
	return nil
}

func (ksApp *KsApp) components() (map[string]*kstypes.KsComponent, error) {
	moduleName := "/"
	topModule := component.NewModule(ksApp.KApp, moduleName)
	components, err := topModule.Components()
	if err != nil {
		return nil, fmt.Errorf("there was a problem getting the components %v. Error: %v", ksApp.KsApp.Name, err)
	}
	comps := make(map[string]*kstypes.KsComponent)
	for _, comp := range components {
		name := comp.Name(false)
		comps[name] = &kstypes.KsComponent{
			Name:      name,
			Prototype: name,
		}
	}
	return comps, nil
}

func (ksApp *KsApp) deleteGlobalResources() error {
	crdClient, clientErr := kftypes.GetApiExtensionsClientOutOfCluster()
	if clientErr != nil {
		return fmt.Errorf("couldn't get  client Error: %v", clientErr)
	}
	crds, crdsErr := crdClient.CustomResourceDefinitions().List(metav1.ListOptions{})
	if crdsErr != nil {
		return fmt.Errorf("couldn't get list of customresourcedefinitions Error: %v", crdsErr)
	}
	for _, crd := range crds.Items {
		if crd.Labels["app.kubernetes.io/name"] == ksApp.KsApp.Name {
			do := &metav1.DeleteOptions{}
			dErr := crdClient.CustomResourceDefinitions().Delete(crd.Name, do)
			if dErr != nil {
				log.Errorf("could not delete %v Error %v", crd.Name, dErr)
			}
		} else if crd.Name == "compositecontrollers.metacontroller.k8s.io" ||
			crd.Name == "controllerrevisions.metacontroller.k8s.io" ||
			crd.Name == "decoratorcontrollers.metacontroller.k8s.io" ||
			crd.Name == "applications.app.k8s.io" {
			do := &metav1.DeleteOptions{}
			dErr := crdClient.CustomResourceDefinitions().Delete(crd.Name, do)
			if dErr != nil {
				log.Errorf("could not delete %v Error %v", crd.Name, dErr)
			}
		}
	}
	cli, cliErr := kftypes.GetClientOutOfCluster()
	if cliErr != nil {
		return fmt.Errorf("couldn't create client Error: %v", cliErr)
	}
	crbs, crbsErr := cli.RbacV1().ClusterRoleBindings().List(metav1.ListOptions{})
	if crbsErr != nil {
		return fmt.Errorf("couldn't get list of clusterrolebindings Error: %v", crbsErr)
	}
	for _, crb := range crbs.Items {
		if crb.Labels[kftypes.DefaultAppLabel] == ksApp.KsApp.Name {
			do := &metav1.DeleteOptions{}
			dErr := cli.RbacV1().ClusterRoleBindings().Delete(crb.Name, do)
			if dErr != nil {
				log.Errorf("could not delete %v Error %v", crb.Name, dErr)
			}
		}
	}
	crbName := "meta-controller-cluster-role-binding"
	do := &metav1.DeleteOptions{}
	dErr := cli.RbacV1().ClusterRoleBindings().Delete(crbName, do)
	if dErr != nil {
		log.Errorf("could not delete %v Error %v", crbName, dErr)
	}
	crs, crsErr := cli.RbacV1().ClusterRoles().List(metav1.ListOptions{})
	if crsErr != nil {
		return fmt.Errorf("couldn't get list of clusterroles Error: %v", crsErr)
	}
	for _, cr := range crs.Items {
		if cr.Labels["app.kubernetes.io/name"] == ksApp.KsApp.Name {
			do := &metav1.DeleteOptions{}
			dErr := cli.RbacV1().ClusterRoles().Delete(cr.Name, do)
			if dErr != nil {
				log.Errorf("could not delete %v Error %v", cr.Name, dErr)
			}
		}
	}
	return nil
}

func (ksApp *KsApp) Delete(resources kftypes.ResourceEnum, options map[string]interface{}) error {
	err := ksApp.deleteGlobalResources()
	if err != nil {
		log.Errorf("there was a problem deleting global resources: %v", err)
	}
	host, _, serverErr := kftypes.ServerVersion()
	if serverErr != nil {
		return fmt.Errorf("couldn't get server version: %v", serverErr)
	}
	cli, cliErr := kftypes.GetClientOutOfCluster()
	if cliErr != nil {
		return fmt.Errorf("couldn't create client Error: %v", cliErr)
	}
	envSetErr := ksApp.envSet(kstypes.KsEnvName, host)
	if envSetErr != nil {
		return fmt.Errorf("couldn't create ksonnet env %v Error: %v", kstypes.KsEnvName, envSetErr)
	}
	clientConfig, clientConfigErr := kftypes.GetClientConfig()
	if clientConfigErr != nil {
		return fmt.Errorf("couldn't load client config Error: %v", clientConfigErr)
	}
	components := []string{"application", "metacontroller"}
	err = actions.RunDelete(map[string]interface{}{
		actions.OptionApp: ksApp.KApp,
		actions.OptionClientConfig: &client.Config{
			Overrides: &clientcmd.ConfigOverrides{},
			Config:    clientcmd.NewDefaultClientConfig(*clientConfig, &clientcmd.ConfigOverrides{}),
		},
		actions.OptionEnvName:        ksApp.KsEnvName,
		actions.OptionComponentNames: components,
		actions.OptionGracePeriod:    int64(10),
	})
	if err != nil {
		log.Infof("there was a problem deleting %v: %v", components, err)
	}
	namespace := ksApp.KsApp.ObjectMeta.Namespace
	log.Infof("deleting namespace: %v", namespace)
	ns, nsMissingErr := cli.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{})
	if nsMissingErr == nil {
		nsErr := cli.CoreV1().Namespaces().Delete(ns.Name, metav1.NewDeleteOptions(int64(100)))
		if nsErr != nil {
			return fmt.Errorf("couldn't delete namespace %v Error: %v", namespace, nsErr)
		}
	}

	name := "meta-controller-cluster-role-binding"
	crb, crbErr := cli.RbacV1().ClusterRoleBindings().Get(name, metav1.GetOptions{})
	if crbErr == nil {
		crbDeleteErr := cli.RbacV1().ClusterRoleBindings().Delete(crb.Name, metav1.NewDeleteOptions(int64(5)))
		if crbDeleteErr != nil {
			return fmt.Errorf("couldn't delete clusterrolebinding %v Error: %v", name, crbDeleteErr)
		}
	}
	return nil
}

func (ksApp *KsApp) Generate(resources kftypes.ResourceEnum, options map[string]interface{}) error {
	log.Infof("Ksonnet.Generate Name %v AppDir %v Platform %v", ksApp.KsApp.Name,
		ksApp.KsApp.Spec.AppDir, ksApp.KsApp.Spec.Platform)
	initErr := ksApp.initKs()
	if initErr != nil {
		return fmt.Errorf("couldn't initialize KfApi: %v", initErr)
	}
	pkgs := ksApp.KsApp.Spec.Packages
	if pkgs == nil || len(pkgs) == 0 {
		ksApp.KsApp.Spec.Packages = kftypes.DefaultPackages
	}
	comps := ksApp.KsApp.Spec.Components
	if comps == nil || len(comps) == 0 {
		ksApp.KsApp.Spec.Components = kftypes.DefaultComponents
	}
	parameters := ksApp.KsApp.Spec.Parameters
	if parameters == nil || len(parameters) == 0 {
		ksApp.KsApp.Spec.Parameters = kftypes.DefaultParameters
	}
	ksRegistry := kstypes.DefaultRegistry
	ksRegistry.Version = ksApp.KsApp.Spec.Version
	ksRegistry.RegUri = ksApp.KsApp.Spec.Repo
	registryAddErr := ksApp.registryAdd(ksRegistry)
	if registryAddErr != nil {
		return fmt.Errorf("couldn't add registry %v. Error: %v", ksRegistry.Name, registryAddErr)
	}
	packageArray := ksApp.KsApp.Spec.Packages
	for _, pkgName := range packageArray {
		pkg := kstypes.KsPackage{
			Name:     pkgName,
			Registry: "kubeflow",
		}
		packageAddErr := ksApp.pkgInstall(pkg)
		if packageAddErr != nil {
			return fmt.Errorf("couldn't add package %v. Error: %v", pkg.Name, packageAddErr)
		}
	}
	componentArray := ksApp.KsApp.Spec.Components
	for _, compName := range componentArray {
		comp := kstypes.KsComponent{
			Name:      compName,
			Prototype: compName,
		}
		parameterMap := ksApp.KsApp.Spec.Parameters
		parameterArgs := []string{}
		parameters := parameterMap[compName]
		if parameters != nil {
			for _, parameter := range parameters {
				name := "--" + parameter.Name
				parameterArgs = append(parameterArgs, name)
				value := parameter.Value
				parameterArgs = append(parameterArgs, value)
			}
		}
		if compName == "application" {
			parameterArgs = append(parameterArgs, "--components")
			prunedArray := kftypes.RemoveItems(componentArray, "application", "metacontroller")
			quotedArray := kftypes.QuoteItems(prunedArray)
			arrayString := "[" + strings.Join(quotedArray, ",") + "]"
			parameterArgs = append(parameterArgs, arrayString)
		}
		componentAddErr := ksApp.componentAdd(comp, parameterArgs)
		if componentAddErr != nil {
			return fmt.Errorf("couldn't add comp %v. Error: %v", comp.Name, componentAddErr)
		}
	}
	return nil
}

func (ksApp *KsApp) Init(resources kftypes.ResourceEnum, options map[string]interface{}) error {
	ksApp.KsApp.Spec.Platform = options[string(kftypes.PLATFORM)].(string)
	err := os.Mkdir(ksApp.KsApp.Spec.AppDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("couldn't create directory %v, most likely it already exists", ksApp.KsApp.Spec.AppDir)
	}
	cfgFilePath := filepath.Join(ksApp.KsApp.Spec.AppDir, kftypes.KfConfigFile)
	_, appDirErr := afero.NewOsFs().Stat(cfgFilePath)
	if appDirErr == nil {
		return fmt.Errorf("config file %v already exists in %v", kftypes.KfConfigFile, ksApp.KsApp.Spec.AppDir)
	}
	cacheDir := path.Join(ksApp.KsApp.Spec.AppDir, kftypes.DefaultCacheDir)
	cacheDirErr := os.Mkdir(cacheDir, os.ModePerm)
	if cacheDirErr != nil {
		return fmt.Errorf("couldn't create directory %v Error %v", cacheDir, cacheDirErr)
	}
	tarballUrl := kftypes.DefaultGitRepo + "/" + ksApp.KsApp.Spec.Version + "?archive=tar.gz"
	tarballUrlErr := gogetter.GetAny(cacheDir, tarballUrl)
	if tarballUrlErr != nil {
		return fmt.Errorf("couldn't download kubeflow repo %v Error %v", tarballUrl, tarballUrlErr)
	}
	files, filesErr := ioutil.ReadDir(cacheDir)
	if filesErr != nil {
		return fmt.Errorf("couldn't read %v Error %v", cacheDir, filesErr)
	}
	subdir := files[0].Name()
	extractedPath := filepath.Join(cacheDir, subdir)
	newPath := filepath.Join(cacheDir, ksApp.KsApp.Spec.Version)
	renameErr := os.Rename(extractedPath, newPath)
	if renameErr != nil {
		return fmt.Errorf("couldn't rename %v to %v Error %v", extractedPath, newPath, renameErr)
	}
	ksApp.KsApp.Spec.Repo = path.Join(newPath, "kubeflow")
	createConfigErr := ksApp.writeConfigFile()
	if createConfigErr != nil {
		return fmt.Errorf("cannot create config file app.yaml in %v", ksApp.KsApp.Spec.AppDir)
	}
	return nil
}

func (ksApp *KsApp) initKs() error {
	newRoot := path.Join(ksApp.KsApp.Spec.AppDir, ksApp.KsName)
	ksApp.KsEnvName = kstypes.KsEnvName
	host, k8sSpec, err := kftypes.ServerVersion()
	if err != nil {
		return fmt.Errorf("couldn't get server version: %v", err)
	}
	options := map[string]interface{}{
		actions.OptionFs:                    afero.NewOsFs(),
		actions.OptionName:                  ksApp.KsName,
		actions.OptionEnvName:               ksApp.KsEnvName,
		actions.OptionNewRoot:               newRoot,
		actions.OptionServer:                host,
		actions.OptionSpecFlag:              k8sSpec,
		actions.OptionNamespace:             ksApp.KsApp.Namespace,
		actions.OptionSkipDefaultRegistries: true,
	}
	err = actions.RunInit(options)
	if err != nil {
		return fmt.Errorf("there was a problem initializing the app: %v", err)
	}
	log.Infof("Successfully initialized the app %v.", ksApp.KsApp.Name)

	return nil
}

func (ksApp *KsApp) envSet(envName string, host string) error {
	ksApp.KsEnvName = envName
	err := actions.RunEnvSet(map[string]interface{}{
		actions.OptionAppRoot: ksApp.ksRoot(),
		actions.OptionEnvName: ksApp.KsEnvName,
		actions.OptionServer:  host,
	})
	if err != nil {
		return fmt.Errorf("There was a problem setting ksonnet env: %v", err)
	}
	return nil
}

func (ksApp *KsApp) ksRoot() string {
	root := path.Join(ksApp.KsApp.Spec.AppDir, ksApp.KsName)
	return root
}

func (ksApp *KsApp) libraries() (map[string]*kstypes.KsLibrary, error) {
	libs, err := ksApp.KApp.Libraries()
	if err != nil {
		return nil, fmt.Errorf("there was a problem getting the libraries %v. Error: %v", ksApp.KsApp.Name, err)
	}

	libraries := make(map[string]*kstypes.KsLibrary)
	for k, v := range libs {
		libraries[k] = &kstypes.KsLibrary{
			Name:     v.Name,
			Registry: v.Registry,
			Version:  v.Version,
		}
	}
	return libraries, nil
}

func (ksApp *KsApp) registries() (map[string]*kstypes.Registry, error) {
	regs, err := ksApp.KApp.Registries()
	if err != nil {
		return nil, fmt.Errorf("There was a problem getting the registries %v. Error: %v", ksApp.KsApp.Name, err)
	}
	registries := make(map[string]*kstypes.Registry)
	for k, v := range regs {
		registries[k] = &kstypes.Registry{
			Name:     v.Name,
			Protocol: v.Protocol,
			URI:      v.URI,
		}
	}

	return registries, nil
}

func (ksApp *KsApp) paramSet(component string, name string, value string) error {
	err := actions.RunParamSet(map[string]interface{}{
		actions.OptionAppRoot: ksApp.ksRoot(),
		actions.OptionName:    component,
		actions.OptionPath:    name,
		actions.OptionValue:   value,
	})
	if err != nil {
		return fmt.Errorf("Error when setting Parameters %v for Component %v: %v", name, component, err)
	}
	return nil
}

func (ksApp *KsApp) pkgInstall(pkg kstypes.KsPackage) error {
	root := ksApp.ksRoot()
	err := actions.RunPkgInstall(map[string]interface{}{
		actions.OptionAppRoot: root,
		actions.OptionPkgName: pkg.Registry + "/" + pkg.Name,
		actions.OptionName:    pkg.Name,
		actions.OptionForce:   false,
	})
	if err != nil {
		return fmt.Errorf("there was a problem installing package %v: %v", pkg.Name, err)
	}
	return nil
}

func (ksApp *KsApp) prototypeUse(m map[string]interface{}) error {
	return nil
}

func (ksApp *KsApp) registryAdd(registry *kstypes.RegistryConfig) error {
	log.Infof("App %v add registry %v URI %v", ksApp.KsApp.Name, registry.Name, registry.RegUri)
	root := ksApp.ksRoot()
	options := map[string]interface{}{
		actions.OptionAppRoot:  root,
		actions.OptionName:     registry.Name,
		actions.OptionURI:      registry.RegUri,
		actions.OptionPath:     registry.Path,
		actions.OptionVersion:  registry.Version,
		actions.OptionOverride: false,
	}
	err := actions.RunRegistryAdd(options)
	if err != nil {
		return fmt.Errorf("there was a problem adding registry %v: %v", registry.Name, err)
	}
	return nil
}

func (ksApp *KsApp) Show(resources kftypes.ResourceEnum, options map[string]interface{}) error {
	capture := kftypes.Capture()
	err := actions.RunShow(map[string]interface{}{
		actions.OptionApp: ksApp.KApp,
		actions.OptionComponentNames: []string{},
		actions.OptionEnvName:        kstypes.KsEnvName,
		actions.OptionFormat:         "yaml",
	})
	if err != nil {
		return fmt.Errorf("there was a problem calling show: %v", err)
	}
	yamlDir := filepath.Join(ksApp.KsApp.Spec.AppDir, "yamls")
	err = os.Mkdir(yamlDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("couldn't create directory %v, most likely it already exists", yamlDir)
	}
	output, outputErr := capture()
	if outputErr != nil {
		return fmt.Errorf("there was a problem calling capture: %v", outputErr)
	}
	yamlFile := filepath.Join(yamlDir, "default.yaml")
	yamlFileErr := ioutil.WriteFile(yamlFile, []byte(output), 0644)
	if yamlFileErr != nil {
		return fmt.Errorf("could not write to %v Error %v", yamlFile, yamlFileErr)
	}
	return nil
}

func (ksApp *KsApp) writeConfigFile() error {
	buf, bufErr := yaml.Marshal(ksApp.KsApp)
	if bufErr != nil {
		return bufErr
	}
	cfgFilePath := filepath.Join(ksApp.KsApp.Spec.AppDir, kftypes.KfConfigFile)
	cfgFilePathErr := ioutil.WriteFile(cfgFilePath, buf, 0644)
	if cfgFilePathErr != nil {
		return cfgFilePathErr
	}
	return nil
}
