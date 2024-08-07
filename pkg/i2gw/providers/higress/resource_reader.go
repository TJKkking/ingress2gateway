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

func (r *resourceReader) readResourcesFromCluster(ctx context.Context) (*storage, error) {
	storage := newResourcesStorage()

	ingresses, err := common.ReadIngressesFromCluster(ctx, r.conf.Client, sets.New(HigressClass))
	if err != nil {
		return nil, err
	}
	storage.Ingresses.FromMap(ingresses)
	return storage, nil
}

func (r *resourceReader) readResourcesFromFile(filename string) (*storage, error) {
	// TODO: implement 貌似不生效
	storage := newResourcesStorage()

	ingresses, err := common.ReadIngressesFromFile(filename, r.conf.Namespace, sets.New(HigressClass))
	if err != nil {
		return nil, err
	}
	storage.Ingresses.FromMap(ingresses)
	return storage, nil
}
