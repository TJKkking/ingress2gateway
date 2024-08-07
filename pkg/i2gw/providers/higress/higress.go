package higress

import (
	"context"
	"fmt"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const (
	// The Name of the provider.
	Name = "higress"

	// HigressClass is the default ingress class used by the Higress controller.
	HigressClass = "higress"

	// DefaultAnnotationsPrefix defines the common prefix used in the nginx ingress controller
	DefaultAnnotationsPrefix = "nginx.ingress.kubernetes.io"

	// HigressAnnotationsPrefix defines the common prefix used in the higress ingress controller
	HigressAnnotationsPrefix = "higress.io"
)

func init() {
	i2gw.ProviderConstructorByName[Name] = NewProvider
}

// Provider implements the i2gw.Provider interface.
type Provider struct {
	storage        *storage
	resourceReader *resourceReader
	converter      *converter
}

// NewProvider constructs and returns the higress implementation of i2gw.Provider.
func NewProvider(conf *i2gw.ProviderConf) i2gw.Provider {
	return &Provider{
		storage:        newResourcesStorage(),
		resourceReader: newResourceReader(conf),
		converter:      newConverter(),
	}
}

// ToGatewayAPI converts stored Ingress-Nginx API entities to i2gw.GatewayResources
// including the higress specific features.
func (p *Provider) ToGatewayAPI() (i2gw.GatewayResources, field.ErrorList) {
	return p.converter.convert(p.storage)
}

func (p *Provider) ReadResourcesFromCluster(ctx context.Context) error {
	storage, err := p.resourceReader.readResourcesFromCluster(ctx)
	if err != nil {
		return fmt.Errorf("failed to read resources from cluster: %w", err)
	}

	p.storage = storage
	return nil
}

func (p *Provider) ReadResourcesFromFile(_ context.Context, filename string) error {
	storage, err := p.resourceReader.readResourcesFromFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read resources from file: %w", err)
	}

	p.storage = storage
	return nil
}
