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

package gcpapp

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/cenkalti/backoff"
	"github.com/ksonnet/ksonnet/pkg/actions"
	"github.com/ksonnet/ksonnet/pkg/app"
	"github.com/ksonnet/ksonnet/pkg/client"
	"github.com/ksonnet/ksonnet/pkg/component"
	kftypes "github.com/kubeflow/kubeflow/bootstrap/pkg/apis/apps/v1alpha1"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/spf13/viper"
	"io"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"text/template"
	"time"
)

// GcpApp implements the KfApp Interface
type GcpApp struct {
	AppName string
	// appDir is the directory where apps should be stored.
	AppDir string
	// ksonnet root name
	KsName string
	// ksonnet env name
	KsEnvName string
	CfgFile   *viper.Viper
	Fs        afero.Fs
	KApp      app.App
	GcpApp    kftypes.KsApp
}

func GetKfApp(options map[string]interface{}) kftypes.KfApp {
	_gcpapp := &GcpApp{
		AppName:   "",
		AppDir:    "",
		KsName:    kftypes.KsName,
		KsEnvName: kftypes.KsEnvName,
		Fs:        nil,
		CfgFile:   nil,
		KApp:      nil,
		GcpApp: kftypes.KsApp{
			TypeMeta: metav1.TypeMeta{
				Kind:       "KfApp",
				APIVersion: "apps.kubeflow.org/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "",
			},
			Spec: kftypes.KsAppSpec{
				Platform:   "gcp",
				Version:    "",
				Components: []string{"all"},
				Packages:   []string{"all"},
				App: kftypes.AppConfig{
					Registries: []*kftypes.RegistryConfig{
						{
							Name: "kubeflow",
							Repo: "https://github.com/kubeflow/kubeflow.git",
							Path: "kubeflow",
						},
					},
					Packages:   []kftypes.KsPackage{},
					Components: []kftypes.KsComponent{},
					Parameters: []kftypes.KsParameter{
						{
							Component: "spartakus",
							Name:      "usageId",
							Value:     fmt.Sprintf("%08d", 10000000+rand.Intn(90000000)),
						},
						{
							Component: "spartakus",
							Name:      "reportUsage",
							Value:     "true",
						},
					},
				},
			},
		},
	}
	_gcpapp.GcpApp.Name = options["AppName"].(string)
	for k, v := range options {
		x := reflect.ValueOf(_gcpapp).Elem().FieldByName(k)
		x.Set(reflect.ValueOf(v))
	}
	return _gcpapp
}

var GcpAppTemplate = string(`
apiVersion: {{.APIVersion}}
kind: {{.Kind}}
metadata: 
  name: {{.ObjectMeta.Name}}
  namespace: {{.ObjectMeta.Namespace}}
spec:
  platform: {{.Spec.Platform}}
  components: {{.Spec.Components}}
  app:
    registries:
{{range $registry := .Spec.App.Registries }}
      - name: {{$registry.Name}}
        repo: {{$registry.Repo}}
        version: {{$registry.Version}}
        path: {{$registry.Path}}
        RegUri: {{$registry.RegUri}}
{{end}}
    packages:
{{range $package := .Spec.App.Packages }}
      - name: {{$package.Name}}
        registry: {{$package.Registry}}
{{end}}
    components:
{{range $component := .Spec.App.Components }}
      - name: {{$component.Name}}
        prototype: {{$component.Prototype}}
{{end}}
    parameters:
{{range $parameter := .Spec.App.Parameters }}
      - component: {{$parameter.Component}}
        name: {{$parameter.Name}}
        value: {{$parameter.Value}}
{{end}}
`)

func KubeConfigPath() string {
	kubeconfigEnv := os.Getenv("KUBECONFIG")
	if kubeconfigEnv == "" {
		home := os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home == "" {
			for _, h := range []string{"HOME", "USERPROFILE"} {
				if home = os.Getenv(h); home != "" {
					break
				}
			}
		}
		kubeconfigPath := filepath.Join(home, ".kube", "config")
		return kubeconfigPath
	}
	return kubeconfigEnv
}

// BuildOutOfClusterConfig returns k8s config
func BuildOutOfClusterConfig() (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.ExplicitPath = KubeConfigPath()
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return nil, err
	}
	return config, nil
}

func ServerVersion() (host string, version string, err error) {
	restApi, err := BuildOutOfClusterConfig()
	if err != nil {
		return "", "", fmt.Errorf("couldn't build out-of-cluster config. Error: %v", err)
	}
	clnt, clntErr := kubernetes.NewForConfig(restApi)
	if clntErr != nil {
		return "", "", fmt.Errorf("couldn't get clientset. Error: %v", err)
	}
	serverVersion, serverVersionErr := clnt.ServerVersion()
	if serverVersionErr != nil {
		return "", "", fmt.Errorf("couldn't get server version info. Error: %v", serverVersionErr)
	}
	re := regexp.MustCompile("^v[0-9]+.[0-9]+.[0-9]+")
	version = re.FindString(serverVersion.String())
	return restApi.Host, "version:" + version, nil
}

func GetClientConfig() (*clientcmdapi.Config, error) {
	kubeconfig := KubeConfigPath()
	config, configErr := clientcmd.LoadFromFile(kubeconfig)
	if configErr != nil {
		return nil, fmt.Errorf("could not load config Error: %v", configErr)

	}
	return config, nil
}

// GetClientOutOfCluster returns a k8s clientset to the request from outside of cluster
func GetClientOutOfCluster() (kubernetes.Interface, error) {
	config, err := BuildOutOfClusterConfig()
	if err != nil {
		log.Fatalf("Can not get kubernetes config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Can not get kubernetes client: %v", err)
	}

	return clientset, nil
}

// capture replaces os.Stdout with a writer that buffers any data written
// to os.Stdout. Call the returned function to cleanup and get the data
// as a string.
func capture() func() (string, error) {
	r, w, err := os.Pipe()
	if err != nil {
		panic(err)
	}

	done := make(chan error, 1)

	save := os.Stdout
	os.Stdout = w

	var buf strings.Builder

	go func() {
		_, err := io.Copy(&buf, r)
		_ = r.Close()
		done <- err
	}()

	return func() (string, error) {
		os.Stdout = save
		_ = w.Close()
		err := <-done
		return buf.String(), err
	}
}

func NewKfAppWithNameAndConfig(appName string, CfgFile *viper.Viper) (kftypes.KfApp, error) {
	appDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("could not get current directory %v", err)
	}
	fs := afero.NewOsFs()
	api := &GcpApp{
		AppName:   appName,
		AppDir:    appDir,
		KsName:    kftypes.KsName,
		KsEnvName: kftypes.KsEnvName,
		Fs:        fs,
		CfgFile:   CfgFile,
		KApp:      nil,
		GcpApp: kftypes.KsApp{
			TypeMeta: metav1.TypeMeta{
				Kind:       "KsApp",
				APIVersion: "apps.kubeflow.org/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: appName,
			},
			Spec: kftypes.KsAppSpec{},
		},
	}
	return api, nil
}

func NewKfAppWithConfig(cfg *viper.Viper) (kftypes.KfApp, error) {
	appDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("could not get current directory %v", err)
	}
	appName := filepath.Base(appDir)
	log.Infof("appName %v appDir %v", appName, appDir)
	cfg.AddConfigPath(appDir)
	cfgErr := cfg.ReadInConfig()
	if cfgErr != nil {
		return nil, fmt.Errorf("could not read config file %v Error %v", kftypes.KfConfigFile, cfgErr)
	}
	cfgfile := cfg.ConfigFileUsed()
	if cfgfile == "" {
		return nil, fmt.Errorf("config file does not exist")
	}
	return NewKfApp(appName, appDir, cfg)
}

func NewKfApp(appName string, appDir string, CfgFile *viper.Viper) (kftypes.KfApp, error) {
	fs := afero.NewOsFs()
	ksDir := path.Join(appDir, kftypes.KsName)
	kApp, kAppErr := app.Load(fs, nil, ksDir)
	if kAppErr != nil {
		return nil, fmt.Errorf("there was a problem loading app %v. Error: %v", appName, kAppErr)
	}
	api := &GcpApp{
		AppName:   appName,
		AppDir:    appDir,
		KsName:    kftypes.KsName,
		KsEnvName: kftypes.KsEnvName,
		Fs:        fs,
		CfgFile:   CfgFile,
		KApp:      kApp,
		GcpApp: kftypes.KsApp{
			TypeMeta: metav1.TypeMeta{
				Kind:       "KsApp",
				APIVersion: "apps.kubeflow.org/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: appName,
			},
			Spec: kftypes.KsAppSpec{},
		},
	}
	if api.CfgFile != nil {
		applicationSpecErr := api.CfgFile.Sub("spec").Unmarshal(&api.GcpApp.Spec)
		if applicationSpecErr != nil {
			return nil, fmt.Errorf("couldn't unmarshall yaml. Error: %v", applicationSpecErr)
		}
		for _, registry := range api.GcpApp.Spec.App.Registries {
			if registry.Name != "" {
				if registry.Name == "kubeflow" {
					kubeflowRepo := os.Getenv("KUBEFLOW_REPO")
					if kubeflowRepo != "" {
						registry.RegUri = path.Join(kubeflowRepo, "kubeflow")
					}
					kubeflowVersion := os.Getenv("KUBEFLOW_VERSION")
					if kubeflowVersion != "" {
						registry.Version = kubeflowVersion
					}
				}
			}
		}
		componentsEnvVar := os.Getenv("KUBEFLOW_COMPONENTS")
		if componentsEnvVar != "" {
			components := strings.Split(componentsEnvVar, ",")
			api.GcpApp.Spec.App.Components = make([]kftypes.KsComponent, len(components))
			for _, comp := range components {
				ksComponent := kftypes.KsComponent{
					Name:      comp,
					Prototype: comp,
				}
				api.GcpApp.Spec.App.Components = append(api.GcpApp.Spec.App.Components, ksComponent)
			}
		}
	}
	return api, nil
}

func writeConfigFile(cfg *viper.Viper, appName string, appDir string) error {
	tmpl, tmplErr := template.New(kftypes.KfConfigFile).Parse(GcpAppTemplate)
	if tmplErr != nil {
		return tmplErr
	}
	namespace := os.Getenv("K8S_NAMESPACE")
	if namespace == "" {
		namespace = kftypes.DefaultNamespace
	}
	kubeflowRepo := os.Getenv("DEFAULT_KUBEFLOW_REPO")
	if kubeflowRepo == "" {
		kubeflowRepo = kftypes.DefaultKfRepo
	}
	re := regexp.MustCompile(`(^\$GOPATH)(.*$)`)
	goPathVar := os.Getenv("GOPATH")
	if goPathVar != "" {
		kubeflowRepo = re.ReplaceAllString(kubeflowRepo, goPathVar+`$2`)
	}
	log.Infof("kubeflowRepo %v", kubeflowRepo)
	gcpApp := kftypes.KsApp{
		TypeMeta: metav1.TypeMeta{
			Kind:       "KsApp",
			APIVersion: "apps.kubeflow.org/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      appName,
			Namespace: namespace,
		},
		Spec: kftypes.KsAppSpec{
			Platform:   "none",
			Components: []string{"all"},
			App: kftypes.AppConfig{
				Registries: []*kftypes.RegistryConfig{
					{
						Name:    "kubeflow",
						Repo:    "https://github.com/kubeflow/kubeflow.git",
						Version: "0.4",
						Path:    "kubeflow",
						RegUri:  kubeflowRepo,
					},
				},
				Packages: []kftypes.KsPackage{
					{
						Name:     "argo",
						Registry: "kubeflow",
					},
					{
						Name:     "pipeline",
						Registry: "kubeflow",
					},
					{
						Name:     "common",
						Registry: "kubeflow",
					},
					{
						Name:     "examples",
						Registry: "kubeflow",
					},
					{
						Name:     "jupyter",
						Registry: "kubeflow",
					},
					{
						Name:     "katib",
						Registry: "kubeflow",
					},
					{
						Name:     "mpi-job",
						Registry: "kubeflow",
					},
					{
						Name:     "pytorch-job",
						Registry: "kubeflow",
					},
					{
						Name:     "seldon",
						Registry: "kubeflow",
					},
					{
						Name:     "tf-serving",
						Registry: "kubeflow",
					},
					{
						Name:     "openvino",
						Registry: "kubeflow",
					},
					{
						Name:     "tensorboard",
						Registry: "kubeflow",
					},
					{
						Name:     "tf-training",
						Registry: "kubeflow",
					},
					{
						Name:     "metacontroller",
						Registry: "kubeflow",
					},
					{
						Name:     "profiles",
						Registry: "kubeflow",
					},
					{
						Name:     "application",
						Registry: "kubeflow",
					},
					{
						Name:     "modeldb",
						Registry: "kubeflow",
					},
				},
				Components: []kftypes.KsComponent{
					{
						Name:      "pytorch-operator",
						Prototype: "pytorch-operator",
					},
					{
						Name:      "ambassador",
						Prototype: "ambassador",
					},
					{
						Name:      "openvino",
						Prototype: "openvino",
					},
					{
						Name:      "jupyter",
						Prototype: "jupyter",
					},
					{
						Name:      "centraldashboard",
						Prototype: "centraldashboard",
					},
					{
						Name:      "tf-job-operator",
						Prototype: "tf-job-operator",
					},
					{
						Name:      "tensorboard",
						Prototype: "tensorboard",
					},
					{
						Name:      "metacontroller",
						Prototype: "metacontroller",
					},
					{
						Name:      "profiles",
						Prototype: "profiles",
					},
					{
						Name:      "notebooks",
						Prototype: "notebooks",
					},
					{
						Name:      "argo",
						Prototype: "argo",
					},
					{
						Name:      "pipeline",
						Prototype: "pipeline",
					},
					{
						Name:      "katib",
						Prototype: "katib",
					},
					{
						Name:      "spartakus",
						Prototype: "spartakus",
					},
					{
						Name:      "application",
						Prototype: "application",
					},
				},
				Parameters: []kftypes.KsParameter{
					{
						Component: "spartakus",
						Name:      "usageId",
						Value:     fmt.Sprintf("%08d", 10000000+rand.Intn(90000000)),
					},
					{
						Component: "spartakus",
						Name:      "reportUsage",
						Value:     "true",
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	execErr := tmpl.Execute(&buf, gcpApp)
	if execErr != nil {
		return execErr
	}
	errDefaultConfig := cfg.ReadConfig(bytes.NewBuffer(buf.Bytes()))
	if errDefaultConfig != nil {
		return errDefaultConfig
	}
	CfgFile := filepath.Join(appDir, kftypes.KfConfigFile)
	CfgFileErr := cfg.WriteConfigAs(CfgFile)
	if CfgFileErr != nil {
		return CfgFileErr
	}
	return nil
}

func (gcpApp *GcpApp) Apply() error {
	host, _, err := ServerVersion()
	if err != nil {
		return fmt.Errorf("couldn't get server version: %v", err)
	}
	cli, cliErr := GetClientOutOfCluster()
	if cliErr != nil {
		return fmt.Errorf("couldn't create client Error: %v", cliErr)
	}
	envSetErr := gcpApp.EnvSet(kftypes.KsEnvName, host)
	if envSetErr != nil {
		return fmt.Errorf("couldn't create ksonnet env %v Error: %v", kftypes.KsEnvName, envSetErr)
	}
	//ks param set application name ${DEPLOYMENT_NAME}
	name := gcpApp.GcpApp.Name
	paramSetErr := gcpApp.ParamSet("application", "name", name)
	if paramSetErr != nil {
		return fmt.Errorf("couldn't set application component's name to %v Error: %v", name, paramSetErr)
	}
	namespace := os.Getenv("K8S_NAMESPACE")
	if namespace == "" {
		namespace = kftypes.DefaultNamespace
	}
	_, nsMissingErr := cli.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{})
	if nsMissingErr != nil {
		nsSpec := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
		_, nsErr := cli.CoreV1().Namespaces().Create(nsSpec)
		if nsErr != nil {
			return fmt.Errorf("couldn't create namespace %v Error: %v", namespace, nsErr)
		}
	}
	clientConfig, clientConfigErr := GetClientConfig()
	if clientConfigErr != nil {
		return fmt.Errorf("couldn't load client config Error: %v", clientConfigErr)
	}
	components := []string{"metacontroller"}
	applyErr := gcpApp.ApplyComponent(components, clientConfig)
	if applyErr != nil {
		return fmt.Errorf("couldn't apply metacontroller Error: %v", applyErr)
	}
	components = []string{"application"}
	applyErr = gcpApp.ApplyComponent(components, clientConfig)
	if applyErr != nil {
		return fmt.Errorf("couldn't apply application Error: %v", applyErr)
	}
	return nil
}

func (gcpApp *GcpApp) ApplyComponent(components []string, cfg *clientcmdapi.Config) error {
	applyOptions := map[string]interface{}{
		actions.OptionApp: gcpApp.KApp,
		actions.OptionClientConfig: &client.Config{
			Overrides: &clientcmd.ConfigOverrides{},
			Config:    clientcmd.NewDefaultClientConfig(*cfg, &clientcmd.ConfigOverrides{}),
		},
		actions.OptionComponentNames: components,
		actions.OptionCreate:         true,
		actions.OptionDryRun:         false,
		actions.OptionEnvName:        "default",
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
		log.Errorf("Components apply failed; Error: %v", err)
	} else {
		log.Infof("All components apply succeeded")
	}
	return err

}

func (gcpApp *GcpApp) ComponentAdd(component kftypes.KsComponent, args []string) error {
	componentPath := filepath.Join(gcpApp.KsRoot(), "components", component.Name+".jsonnet")
	componentArgs := make([]string, 0)
	componentArgs = append(componentArgs, component.Prototype)
	componentArgs = append(componentArgs, component.Name)
	if args != nil && len(args) > 0 {
		componentArgs = append(componentArgs, args[0:]...)
	}
	if exists, _ := afero.Exists(gcpApp.Fs, componentPath); !exists {
		log.Infof("Creating Component: %v ...", component.Name)
		err := actions.RunPrototypeUse(map[string]interface{}{
			actions.OptionAppRoot:   gcpApp.KsRoot(),
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

func (gcpApp *GcpApp) Components() (map[string]*kftypes.KsComponent, error) {
	moduleName := "/"

	topModule := component.NewModule(gcpApp.KApp, moduleName)
	components, err := topModule.Components()
	if err != nil {
		return nil, fmt.Errorf("there was a problem getting the Components %v. Error: %v", gcpApp.AppName, err)
	}
	comps := make(map[string]*kftypes.KsComponent)
	for _, comp := range components {
		name := comp.Name(false)
		comps[name] = &kftypes.KsComponent{
			Name:      name,
			Prototype: name,
		}
	}
	return comps, nil
}

func (gcpApp *GcpApp) Delete() error {
	return nil
}

func (gcpApp *GcpApp) Generate() error {
	host, k8sSpec, err := ServerVersion()
	if err != nil {
		return fmt.Errorf("couldn't get server version: %v", err)
	}
	namespace := os.Getenv("K8S_NAMESPACE")
	writeConfigErr := writeConfigFile(gcpApp.CfgFile, gcpApp.AppName, gcpApp.AppDir)
	if writeConfigErr != nil {
		return fmt.Errorf("couldn't write config file app.yaml in %v Error %v", gcpApp.AppDir, writeConfigErr)
	}
	initErr := gcpApp.InitKs("default", k8sSpec, host, namespace)
	if initErr != nil {
		return fmt.Errorf("couldn't initialize KfApi: %v", initErr)
	}
	for _, registry := range gcpApp.GcpApp.Spec.App.Registries {
		registryAddErr := gcpApp.RegistryAdd(registry)
		if registryAddErr != nil {
			return fmt.Errorf("couldn't add registry %v. Error: %v", registry.Name, registryAddErr)
		}
	}
	for _, pkg := range gcpApp.GcpApp.Spec.App.Packages {
		packageAddErr := gcpApp.PkgInstall(pkg)
		if packageAddErr != nil {
			return fmt.Errorf("couldn't add package %v. Error: %v", pkg.Name, packageAddErr)
		}
	}
	for _, comp := range gcpApp.GcpApp.Spec.App.Components {
		componentAddErr := gcpApp.ComponentAdd(comp, []string{})
		if componentAddErr != nil {
			return fmt.Errorf("couldn't add comp %v. Error: %v", comp.Name, componentAddErr)
		}
	}
	for _, parameter := range gcpApp.GcpApp.Spec.App.Parameters {
		parameterSetErr := gcpApp.ParamSet(parameter.Component, parameter.Name, parameter.Value)
		if parameterSetErr != nil {
			return fmt.Errorf("couldn't set %v for comp %v. Error: %v",
				parameter.Name, parameter.Component, parameterSetErr)
		}
	}
	return nil
}

func (gcpApp *GcpApp) Init() error {
	//TODO must be checked eg `kfctl init kf_app` results in
	// metadata.name: Invalid value:
	// "kf_app-controller": a DNS-1123 subdomain must consist of lower case alphanumeric characters, '-' or '.',
	// and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is
	// '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')
	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	appDir := path.Join(dir, gcpApp.AppName)
	log.Infof("appDir %v", appDir)
	err = os.Mkdir(appDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("cannot create directory %v", appDir)
	}
	fs := afero.NewOsFs()
	CfgFilePath := filepath.Join(appDir, kftypes.KfConfigFile)
	_, appDirErr := fs.Stat(CfgFilePath)
	if appDirErr == nil {
		return fmt.Errorf("config file %v already exists in %v", kftypes.KfConfigFile, appDir)
	}
	createConfigErr := writeConfigFile(gcpApp.CfgFile, gcpApp.AppName, appDir)
	if createConfigErr != nil {
		return fmt.Errorf("cannot create config file app.yaml in %v", appDir)
	}
	return nil
}

func (gcpApp *GcpApp) InitKs(envName string, k8sSpecFlag string, host string, namespace string) error {
	newRoot := path.Join(gcpApp.AppDir, gcpApp.KsName)
	gcpApp.KsEnvName = envName
	options := map[string]interface{}{
		actions.OptionFs:                    gcpApp.Fs,
		actions.OptionName:                  gcpApp.KsName,
		actions.OptionEnvName:               gcpApp.KsEnvName,
		actions.OptionNewRoot:               newRoot,
		actions.OptionServer:                host,
		actions.OptionSpecFlag:              k8sSpecFlag,
		actions.OptionNamespace:             namespace,
		actions.OptionSkipDefaultRegistries: true,
	}
	err := actions.RunInit(options)
	if err != nil {
		return fmt.Errorf("there was a problem initializing the app: %v", err)
	}
	log.Infof("Successfully initialized the app %v.", gcpApp.AppName)

	return nil
}

func (gcpApp *GcpApp) EnvSet(envName string, host string) error {
	gcpApp.KsEnvName = envName
	err := actions.RunEnvSet(map[string]interface{}{
		actions.OptionAppRoot: gcpApp.KsRoot(),
		actions.OptionEnvName: gcpApp.KsEnvName,
		actions.OptionServer:  host,
	})
	if err != nil {
		return fmt.Errorf("There was a problem setting ksonnet env: %v", err)
	}
	return nil
}

func (gcpApp *GcpApp) KsRoot() string {
	root := path.Join(gcpApp.AppDir, gcpApp.KsName)
	return root
}

func (gcpApp *GcpApp) Libraries() (map[string]*kftypes.KsLibrary, error) {
	libs, err := gcpApp.KApp.Libraries()
	if err != nil {
		return nil, fmt.Errorf("there was a problem getting the libraries %v. Error: %v", gcpApp.AppName, err)
	}

	libraries := make(map[string]*kftypes.KsLibrary)
	for k, v := range libs {
		libraries[k] = &kftypes.KsLibrary{
			Name:     v.Name,
			Registry: v.Registry,
			Version:  v.Version,
		}
	}
	return libraries, nil
}

func (gcpApp *GcpApp) Registries() (map[string]*kftypes.Registry, error) {
	regs, err := gcpApp.KApp.Registries()
	if err != nil {
		return nil, fmt.Errorf("There was a problem getting the Registries %v. Error: %v", gcpApp.AppName, err)
	}
	registries := make(map[string]*kftypes.Registry)
	for k, v := range regs {
		registries[k] = &kftypes.Registry{
			Name:     v.Name,
			Protocol: v.Protocol,
			URI:      v.URI,
		}
	}

	return registries, nil
}

func (gcpApp *GcpApp) Root() string {
	return gcpApp.AppDir
}

func (gcpApp *GcpApp) ParamSet(component string, name string, value string) error {
	err := actions.RunParamSet(map[string]interface{}{
		actions.OptionAppRoot: gcpApp.KsRoot(),
		actions.OptionName:    component,
		actions.OptionPath:    name,
		actions.OptionValue:   value,
	})
	if err != nil {
		return fmt.Errorf("Error when setting Parameters %v for Component %v: %v", name, component, err)
	}
	return nil
}

func (gcpApp *GcpApp) PkgInstall(pkg kftypes.KsPackage) error {
	root := gcpApp.KsRoot()
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

func (gcpApp *GcpApp) PrototypeUse(m map[string]interface{}) error {
	return nil
}

func (gcpApp *GcpApp) RegistryAdd(registry *kftypes.RegistryConfig) error {
	log.Infof("App %v add registry %v URI %v", gcpApp.AppName, registry.Name, registry.RegUri)
	root := gcpApp.KsRoot()
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

func (gcpApp *GcpApp) Show(components []string) error {
	done := capture()
	err := actions.RunShow(map[string]interface{}{
		actions.OptionApp:            gcpApp.KApp,
		actions.OptionComponentNames: components,
		actions.OptionEnvName:        gcpApp.KsEnvName,
		actions.OptionFormat:         "yaml",
	})
	if err != nil {
		return fmt.Errorf("there was a problem showing components %v: %v", components, err)
	}
	capturedOutput, capturedOutputErr := done()
	if capturedOutputErr != nil {
		return fmt.Errorf("there was a problem capturing the output: %v", capturedOutput)
	}
	outputFileName := filepath.Join(gcpApp.KsRoot(), gcpApp.KsEnvName+".yaml")
	outputFile, outputFileErr := os.Create(outputFileName)
	if outputFileErr != nil {
		return fmt.Errorf("there was a problem creating output file %v: %v", outputFileName, outputFileErr)
	}
	defer outputFile.Close()
	writer := bufio.NewWriter(outputFile)
	_, writerErr := writer.WriteString(capturedOutput)
	if writerErr != nil {
		return fmt.Errorf("there was a problem writing to %v: %v", outputFileName, writerErr)
	}
	flushErr := writer.Flush()
	if flushErr != nil {
		return fmt.Errorf("there was a problem flushing file %v: %v", outputFileName, flushErr)
	}

	return nil
}
