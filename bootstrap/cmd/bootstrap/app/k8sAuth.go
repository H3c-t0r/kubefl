package app

import (
	"encoding/base64"

	"cloud.google.com/go/container/apiv1"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	containerpb "google.golang.org/genproto/googleapis/container/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	log "github.com/sirupsen/logrus"
)

func buildClusterConfig(ctx context.Context, token string, project string, zone string,
	clusterId string) (*rest.Config, error) {
	ts := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: token,
	})
	c, err := container.NewClusterManagerClient(ctx, option.WithTokenSource(ts))
	if err != nil {
		return nil, err
	}
	req := &containerpb.GetClusterRequest{
		ProjectId: project,
		Zone:      zone,
		ClusterId: clusterId,
	}
	resp, err := c.GetCluster(ctx, req)
	if err != nil {
		return nil, err
	}
	caDec, _ := base64.StdEncoding.DecodeString(resp.MasterAuth.ClusterCaCertificate)
	return &rest.Config{
		Host:        "https://" + resp.Endpoint,
		BearerToken: token,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: []byte(string(caDec)),
		},
	}, nil
}

func getK8sClientSet(ctx context.Context, token string, project string, zone string,
	cluster string) (*clientset.Clientset, error) {
	k8sConfig, err := buildClusterConfig(ctx, token, project, zone, cluster)
	if err != nil {
		log.Errorf("Failed getting GKE cluster config: %v", err)
		return nil, err
	}
	k8sClientset, err := clientset.NewForConfig(k8sConfig)
	if err != nil {
		return nil, err
	}
	return k8sClientset, nil
}

func createK8sRoleBing(config *rest.Config, roleBinding *v1.ClusterRoleBinding) error {
	kubeClient, err := clientset.NewForConfig(config)
	if err != nil {
		return err
	}
	_, err = kubeClient.RbacV1().ClusterRoleBindings().Create(roleBinding)
	if err != nil {
		return err
	}
	return nil
}
