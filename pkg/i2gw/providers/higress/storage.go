package higress

import (
	"sort"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
)

type OrderedIngressMap struct {
	ingressNames   []types.NamespacedName
	ingressObjects map[types.NamespacedName]*networkingv1.Ingress
}
type storage struct {
	Ingresses OrderedIngressMap
}

func newResourcesStorage() *storage {
	return &storage{
		Ingresses: OrderedIngressMap{
			ingressNames:   []types.NamespacedName{},
			ingressObjects: map[types.NamespacedName]*networkingv1.Ingress{},
		},
	}
}

func (oim *OrderedIngressMap) List() []networkingv1.Ingress {
	ingressList := []networkingv1.Ingress{}
	for _, ing := range oim.ingressNames {
		ingressList = append(ingressList, *oim.ingressObjects[ing])
	}
	return ingressList
}

func (oim *OrderedIngressMap) FromMap(ingresses map[types.NamespacedName]*networkingv1.Ingress) {
	ingNames := []types.NamespacedName{}
	for ing := range ingresses {
		ingNames = append(ingNames, ing)
	}
	sort.Slice(ingNames, func(i, j int) bool {
		return ingNames[i].Name < ingNames[j].Name
	})
	oim.ingressNames = ingNames
	oim.ingressObjects = ingresses
}
