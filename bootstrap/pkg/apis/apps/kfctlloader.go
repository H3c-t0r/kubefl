package apps

import (
	"fmt"
	"github.com/ghodss/yaml"
	configsv3 "github.com/kubeflow/kubeflow/bootstrap/v3/config"
	kfapis "github.com/kubeflow/kubeflow/bootstrap/v3/pkg/apis"
	kfconfig "github.com/kubeflow/kubeflow/bootstrap/v3/pkg/apis/apps/kfctlconfig"
	kfdefv1alpha1 "github.com/kubeflow/kubeflow/bootstrap/v3/pkg/apis/apps/kfdef/v1alpha1"
	kfdefv1beta1 "github.com/kubeflow/kubeflow/bootstrap/v3/pkg/apis/apps/kfdef/v1beta1"
)

func pluginNameToKind(pluginName string) kfconfig.PluginKindType {
	mapper := map[string]kfconfig.PluginKindType{
		AWS:              kfconfig.AWS_PLUGIN_KIND,
		GCP:              kfconfig.GCP_PLUGIN_KIND,
		MINIKUBE:         kfconfig.MINIKUBE_PLUGIN_KIND,
		EXISTING_ARRIKTO: kfconfig.EXISTING_ARRIKTO_PLUGIN_KIND,
	}
	kind, ok := mapper[pluginName]
	if ok {
		return kind
	} else {
		return kfconfig.PluginKindType("KfUnknownPlugin")
	}
}

func kfdefToConfigV1alpha1(appdir string, kfdefBytes []byte) (*kfconfig.KfctlConfig, error) {
	kfdef := &kfdefv1alpha1.KfDef{}
	if err := yaml.Unmarshal(kfdefBytes, kfdef); err != nil {
		return nil, &kfapis.KfError{
			Code:    int(kfapis.INTERNAL_ERROR),
			Message: fmt.Sprintf("could not unmarshal config file onto KfDef struct: %v", err),
		}
	}

	config := &kfconfig.KfctlConfig{
		AppDir:        kfdef.Spec.AppDir,
		UseBasicAuth:  kfdef.Spec.UseBasicAuth,
		SourceVersion: "v1alpha1",
	}
	if config.AppDir == "" {
		config.AppDir = appdir
	}
	config.Name = kfdef.Name
	config.Namespace = kfdef.Namespace
	config.APIVersion = kfdef.APIVersion
	config.Kind = "KfctlConfig"
	for _, app := range kfdef.Spec.Applications {
		application := kfconfig.Application{
			Name: app.Name,
		}
		if app.KustomizeConfig != nil {
			kconfig := &kfconfig.KustomizeConfig{
				Overlays: app.KustomizeConfig.Overlays,
			}
			if app.KustomizeConfig.RepoRef != nil {
				kref := &kfconfig.RepoRef{
					Name: app.KustomizeConfig.RepoRef.Name,
					Path: app.KustomizeConfig.RepoRef.Path,
				}
				kconfig.RepoRef = kref
			}
			for _, param := range app.KustomizeConfig.Parameters {
				p := kfconfig.NameValue{
					Name:  param.Name,
					Value: param.Value,
				}
				kconfig.Parameters = append(kconfig.Parameters, p)
			}
			application.KustomizeConfig = kconfig
		}
		config.Applications = append(config.Applications, application)
	}

	for _, plugin := range kfdef.Spec.Plugins {
		p := kfconfig.Plugin{
			Name:      plugin.Name,
			Namespace: kfdef.Namespace,
			Kind:      pluginNameToKind(plugin.Name),
			Spec:      plugin.Spec,
		}
		config.Plugins = append(config.Plugins, p)
	}

	for _, secret := range kfdef.Spec.Secrets {
		s := kfconfig.Secret{
			Name: secret.Name,
		}
		if secret.SecretSource == nil {
			config.Secrets = append(config.Secrets, s)
			continue
		}
		src := &kfconfig.SecretSource{}
		if secret.SecretSource.LiteralSource != nil {
			src.LiteralSource = &kfconfig.LiteralSource{
				Value: secret.SecretSource.LiteralSource.Value,
			}
		} else if secret.SecretSource.EnvSource != nil {
			src.EnvSource = &kfconfig.EnvSource{
				Name: secret.SecretSource.EnvSource.Name,
			}
		}
		s.SecretSource = src
		config.Secrets = append(config.Secrets, s)
	}

	for _, repo := range kfdef.Spec.Repos {
		r := kfconfig.Repo{
			Name: repo.Name,
			URI:  repo.Uri,
		}
		config.Repos = append(config.Repos, r)
	}

	for _, cond := range kfdef.Status.Conditions {
		c := kfconfig.Condition{
			Type:               kfconfig.ConditionType(cond.Type),
			Status:             cond.Status,
			LastUpdateTime:     cond.LastUpdateTime,
			LastTransitionTime: cond.LastTransitionTime,
			Reason:             cond.Reason,
			Message:            cond.Message,
		}
		config.Status.Conditions = append(config.Status.Conditions, c)
	}
	for name, cache := range kfdef.Status.ReposCache {
		c := kfconfig.Cache{
			Name:      name,
			LocalPath: cache.LocalPath,
		}
		config.Status.Caches = append(config.Status.Caches, c)
	}

	return config, nil
}

func kfdefToConfigV1beta1(appdir string, kfdefBytes []byte) (*kfconfig.KfctlConfig, error) {
	kfdef := &kfdefv1beta1.KfDef{}
	if err := yaml.Unmarshal(kfdefBytes, kfdef); err != nil {
		return nil, &kfapis.KfError{
			Code:    int(kfapis.INTERNAL_ERROR),
			Message: fmt.Sprintf("could not unmarshal config file onto KfDef struct: %v", err),
		}
	}

	// Set UseBasicAuth later.
	config := &kfconfig.KfctlConfig{
		AppDir:        appdir,
		UseBasicAuth:  false,
		SourceVersion: "v1beta1",
	}
	config.Name = kfdef.Name
	config.Namespace = kfdef.Namespace
	config.APIVersion = kfdef.APIVersion
	config.Kind = "KfctlConfig"
	for _, app := range kfdef.Spec.Applications {
		application := kfconfig.Application{
			Name: app.Name,
		}
		if app.KustomizeConfig != nil {
			kconfig := &kfconfig.KustomizeConfig{
				Overlays: app.KustomizeConfig.Overlays,
			}
			if app.KustomizeConfig.RepoRef != nil {
				kref := &kfconfig.RepoRef{
					Name: app.KustomizeConfig.RepoRef.Name,
					Path: app.KustomizeConfig.RepoRef.Path,
				}
				kconfig.RepoRef = kref

				// Use application to infer whether UseBasicAuth is true.
				if kref.Path == "common/basic-auth" {
					config.UseBasicAuth = true
				}
			}
			for _, param := range app.KustomizeConfig.Parameters {
				p := kfconfig.NameValue{
					Name:  param.Name,
					Value: param.Value,
				}
				kconfig.Parameters = append(kconfig.Parameters, p)
			}
			application.KustomizeConfig = kconfig
		}
		config.Applications = append(config.Applications, application)
	}

	for _, plugin := range kfdef.Spec.Plugins {
		p := kfconfig.Plugin{
			Name:      plugin.Name,
			Namespace: kfdef.Namespace,
			Kind:      kfconfig.PluginKindType(plugin.Kind),
			Spec:      plugin.Spec,
		}
		config.Plugins = append(config.Plugins, p)
	}

	for _, secret := range kfdef.Spec.Secrets {
		s := kfconfig.Secret{
			Name: secret.Name,
		}
		if secret.SecretSource == nil {
			config.Secrets = append(config.Secrets, s)
			continue
		}
		src := &kfconfig.SecretSource{}
		if secret.SecretSource.LiteralSource != nil {
			src.LiteralSource = &kfconfig.LiteralSource{
				Value: secret.SecretSource.LiteralSource.Value,
			}
		} else if secret.SecretSource.EnvSource != nil {
			src.EnvSource = &kfconfig.EnvSource{
				Name: secret.SecretSource.EnvSource.Name,
			}
		}
		s.SecretSource = src
		config.Secrets = append(config.Secrets, s)
	}

	for _, repo := range kfdef.Spec.Repos {
		r := kfconfig.Repo{
			Name: repo.Name,
			URI:  repo.URI,
		}
		config.Repos = append(config.Repos, r)
	}

	for _, cond := range kfdef.Status.Conditions {
		c := kfconfig.Condition{
			Type:               kfconfig.ConditionType(cond.Type),
			Status:             cond.Status,
			LastUpdateTime:     cond.LastUpdateTime,
			LastTransitionTime: cond.LastTransitionTime,
			Reason:             cond.Reason,
			Message:            cond.Message,
		}
		config.Status.Conditions = append(config.Status.Conditions, c)
	}
	for _, cache := range kfdef.Status.ReposCache {
		c := kfconfig.Cache{
			Name:      cache.Name,
			LocalPath: cache.LocalPath,
		}
		config.Status.Caches = append(config.Status.Caches, c)
	}

	return config, nil
}

func LoadConfigFromURI(configFile string) (*kfconfig.KfctlConfig, error) {
	return nil, &kfapis.KfError{
		Code:    int(kfapis.INTERNAL_ERROR),
		Message: "Not implemented.",
	}
}

func configToKfDefSerializedV1alpha1(config kfconfig.KfctlConfig) ([]byte, error) {
	kfdef := &kfdefv1alpha1.KfDef{}
	kfdef.Name = config.Name
	kfdef.Namespace = config.Namespace
	kfdef.APIVersion = config.APIVersion
	kfdef.Kind = "KfDef"

	kfdef.Spec.AppDir = config.AppDir
	kfdef.Spec.UseBasicAuth = config.UseBasicAuth
	// Should be deprecated, hardcode it just to be safe.
	kfdef.Spec.EnableApplications = true
	kfdef.Spec.UseIstio = true
	kfdef.Spec.PackageManager = "kustomize"

	// Use generic type to prevent cyclic dependency.
	var gcpPluginSpec map[string]interface{}
	if err := config.GetPluginSpec(kfconfig.GCP_PLUGIN_KIND, &gcpPluginSpec); err == nil {
		if p, ok := gcpPluginSpec["project"]; ok {
			kfdef.Spec.Project = p.(string)
		}
		if e, ok := gcpPluginSpec["email"]; ok {
			kfdef.Spec.Email = e.(string)
		}
		if i, ok := gcpPluginSpec["ipName"]; ok {
			kfdef.Spec.IpName = i.(string)
		}
		if h, ok := gcpPluginSpec["hostname"]; ok {
			kfdef.Spec.Hostname = h.(string)
		}
		if z, ok := gcpPluginSpec["zone"]; ok {
			kfdef.Spec.Zone = z.(string)
		}
		if s, ok := gcpPluginSpec["skipInitProject"]; ok {
			kfdef.Spec.SkipInitProject = s.(bool)
		}
		if d, ok := gcpPluginSpec["deleteStorage"]; ok {
			kfdef.Spec.DeleteStorage = d.(bool)
		}
	}
	for _, app := range config.Applications {
		application := kfdefv1alpha1.Application{
			Name: app.Name,
		}
		if app.KustomizeConfig != nil {
			kconfig := &kfdefv1alpha1.KustomizeConfig{
				Overlays: app.KustomizeConfig.Overlays,
			}
			if app.KustomizeConfig.RepoRef != nil {
				kref := &kfdefv1alpha1.RepoRef{
					Name: app.KustomizeConfig.RepoRef.Name,
					Path: app.KustomizeConfig.RepoRef.Path,
				}
				kconfig.RepoRef = kref
			}
			for _, param := range app.KustomizeConfig.Parameters {
				p := configsv3.NameValue{
					Name:  param.Name,
					Value: param.Value,
				}
				kconfig.Parameters = append(kconfig.Parameters, p)
			}
			application.KustomizeConfig = kconfig
		}
		kfdef.Spec.Applications = append(kfdef.Spec.Applications, application)
	}

	for _, plugin := range config.Plugins {
		p := kfdefv1alpha1.Plugin{
			Name: plugin.Name,
			Spec: plugin.Spec,
		}
		kfdef.Spec.Plugins = append(kfdef.Spec.Plugins, p)
	}

	for _, secret := range config.Secrets {
		s := kfdefv1alpha1.Secret{
			Name: secret.Name,
		}
		if secret.SecretSource != nil {
			s.SecretSource = &kfdefv1alpha1.SecretSource{}
			if secret.SecretSource.LiteralSource != nil {
				s.SecretSource.LiteralSource = &kfdefv1alpha1.LiteralSource{
					Value: secret.SecretSource.LiteralSource.Value,
				}
			}
			if secret.SecretSource.EnvSource != nil {
				s.SecretSource.EnvSource = &kfdefv1alpha1.EnvSource{
					Name: secret.SecretSource.EnvSource.Name,
				}
			}
		}
		kfdef.Spec.Secrets = append(kfdef.Spec.Secrets, s)
	}

	for _, repo := range config.Repos {
		r := kfdefv1alpha1.Repo{
			Name: repo.Name,
			Uri:  repo.URI,
		}
		kfdef.Spec.Repos = append(kfdef.Spec.Repos, r)
	}

	for _, cond := range config.Status.Conditions {
		c := kfdefv1alpha1.KfDefCondition{
			Type:               kfdefv1alpha1.KfDefConditionType(cond.Type),
			Status:             cond.Status,
			LastUpdateTime:     cond.LastUpdateTime,
			LastTransitionTime: cond.LastTransitionTime,
			Reason:             cond.Reason,
			Message:            cond.Message,
		}
		kfdef.Status.Conditions = append(kfdef.Status.Conditions, c)
	}

	for _, cache := range config.Status.Caches {
		kfdef.Status.ReposCache[cache.Name] = kfdefv1alpha1.RepoCache{
			LocalPath: cache.LocalPath,
		}
	}

	return yaml.Marshal(kfdef)
}

func configToKfDefSerializedV1beta1(config kfconfig.KfctlConfig) ([]byte, error) {
	kfdef := &kfdefv1beta1.KfDef{}

	return yaml.Marshal(kfdef)
}

func WriteConfigToFile(config kfconfig.KfctlConfig) error {
	return &kfapis.KfError{
		Code:    int(kfapis.INTERNAL_ERROR),
		Message: "Not implemented.",
	}
}
