// package fake provides a fake implementation of the GCP Plugin
package fake

import (
	kftypes "github.com/kubeflow/kubeflow/bootstrap/pkg/apis/apps"
	"golang.org/x/oauth2"
)

type FakeGcp struct {
	ts oauth2.TokenSource
}

func (g *FakeGcp) PostApply(resources kftypes.ResourceEnum) error {
	return nil
}

func (g *FakeGcp) Apply(resources kftypes.ResourceEnum) error {
	return nil
}

func (g *FakeGcp) Delete(resources kftypes.ResourceEnum) error {
	return nil
}

func (g *FakeGcp) Generate(resources kftypes.ResourceEnum) error {
	return nil
}

func (g *FakeGcp) Init(resources kftypes.ResourceEnum) error {
	return nil
}

func (g *FakeGcp) SetTokenSource(s oauth2.TokenSource) error {
	g.ts = s
	return nil
}
