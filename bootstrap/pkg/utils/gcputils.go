/*
Copyright The Kubeflow Authors.

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

package utils

import (
	"cloud.google.com/go/container/apiv1"
	"encoding/base64"
	"fmt"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	containerpb "google.golang.org/genproto/googleapis/container/v1"
	"k8s.io/client-go/rest"
)

// Use default token source and retrieve cluster information with given project/location/cluster
// information.
func GetClusterInfo(ctx context.Context, project string, loc string, cluster string, ts oauth2.TokenSource) (*containerpb.Cluster, error) {
	c, err := container.NewClusterManagerClient(ctx, option.WithTokenSource(ts))
	if err != nil {
		return nil, err
	}
	getClusterReq := &containerpb.GetClusterRequest{
		ProjectId: project,
		Zone:      loc,
		ClusterId: cluster,
	}
	return c.GetCluster(ctx, getClusterReq)
}

// BuildConfigFromClusterInfo returns k8s config using gcloud Application Default Credentials
// typically $HOME/.config/gcloud/application_default_credentials.json
func BuildConfigFromClusterInfo(ctx context.Context, cluster *containerpb.Cluster, ts oauth2.TokenSource) (*rest.Config, error) {
	t, err := ts.Token()
	if err != nil {
		return nil, fmt.Errorf("Token retrieval error: %v", err)
	}
	caDec, _ := base64.StdEncoding.DecodeString(cluster.MasterAuth.ClusterCaCertificate)
	config := &rest.Config{
		Host:        "https://" + cluster.Endpoint,
		BearerToken: t.AccessToken,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: []byte(string(caDec)),
		},
	}
	return config, nil
}
