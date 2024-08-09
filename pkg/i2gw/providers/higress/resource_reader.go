package higress

import (
	"context"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	"k8s.io/apimachinery/pkg/util/sets"
)

// converter implements the i2gw.CustomResourceReader interface.
type resourceReader struct {
	conf *i2gw.ProviderConf
}

// newResourceReader returns a resourceReader instance.
func newResourceReader(conf *i2gw.ProviderConf) *resourceReader {
	return &resourceReader{
		conf: conf,
	}
}

// readResourcesFromCluster reads resources from the cluster and returns a storage object.
// It uses the provided context and the client from the configuration to read ingresses from the cluster.
// The ingresses are then stored in the storage object.
// Returns the storage object and any error encountered during the process.
func (r *resourceReader) readResourcesFromCluster(ctx context.Context) (*storage, error) {
	storage := newResourcesStorage()

	ingresses, err := common.ReadIngressesFromCluster(ctx, r.conf.Client, sets.New(HigressClass))
	if err != nil {
		return nil, err
	}
	storage.Ingresses.FromMap(ingresses)
	return storage, nil
}

// readResourcesFromFile reads the resources from a file and returns a storage object containing the ingresses.
// It takes a filename as input and returns the storage object and an error if any.
func (r *resourceReader) readResourcesFromFile(filename string) (*storage, error) {
	storage := newResourcesStorage()

	ingresses, err := common.ReadIngressesFromFile(filename, r.conf.Namespace, sets.New(HigressClass))
	if err != nil {
		return nil, err
	}
	storage.Ingresses.FromMap(ingresses)
	return storage, nil
}
