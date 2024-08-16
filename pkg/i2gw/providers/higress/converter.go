/*
Copyright 2023 The Kubernetes Authors.

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

package higress

import (
	"strconv"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	PathTypeExact  = gatewayv1.PathMatchExact
	PathTypePrefix = gatewayv1.PathMatchPathPrefix
)

// converter implements the ToGatewayAPI function of i2gw.ResourceConverter interface.
type converter struct {
	FeatureHandler []FeatureHandler
}

// newConverter returns an higress converter instance.
// Note: The order in which the Paser/Handler is executed may result in different outputs, change it with caution.
func newConverter() *converter {
	return &converter{
		FeatureHandler: []FeatureHandler{
			applyHTTPRouteWithCanary,
			applyHTTPRouteWithRequestHeaderMod,
			applyHTTPRouteWithResponseHeaderMod,
			applyHTTPRouteWithRewrite,
			applyHTTPRouteWithMirror,
			applyHTTPRouteWithTimeout,
			applyHTTPRouteWithRedirect,
		},
	}
}

// FeatureHandler is a function type that handles features for HTTP routes.
type FeatureHandler func(httpRoute *gatewayv1.HTTPRoute, paths []ingressPath) field.ErrorList

// Converter for higress to gateway resources.
func (c *converter) convert(storage *storage) (i2gw.GatewayResources, field.ErrorList) {
	ingressList := storage.Ingresses.List()

	gatewayResources, errs := common.ToGateway(ingressList, i2gw.ProviderImplementationSpecificOptions{})

	if len(errs) > 0 {
		return i2gw.GatewayResources{}, errs
	}

	// Set the default namespace and gateway class name for the gateway resources.
	for idx := range gatewayResources.Gateways {
		temp := gatewayResources.Gateways[idx]
		temp.ObjectMeta.Namespace = GatewayNamespace
		// temp.ObjectMeta.Name = GatewayName
		temp.Spec.GatewayClassName = gatewayv1.ObjectName(GatewayClassName)
		temp.Spec.Addresses = []gatewayv1.GatewayAddress{
			{
				Type:  ptr.To(gatewayv1.AddressType("Hostname")),
				Value: HigressGatewaySvcName,
			},
		}

		// Higress can only handle gateway from "higress-system" namespaces. So we set the allowed routes to "All".
		for i := range temp.Spec.Listeners {
			// name add the index
			lname := string(temp.Spec.Listeners[i].Name) + "-" + strconv.Itoa(i)
			temp.Spec.Listeners[i].Name = gatewayv1.SectionName(lname)
			temp.Spec.Listeners[i].AllowedRoutes = &gatewayv1.AllowedRoutes{
				Namespaces: &gatewayv1.RouteNamespaces{
					From: ptr.To(gatewayv1.FromNamespaces("All")),
				},
			}
		}
		gatewayResources.Gateways[idx] = temp
	}

	// Set the default ParentRefs namespace for the HTTPRoutes.
	for idx := range gatewayResources.HTTPRoutes {
		temp := gatewayResources.HTTPRoutes[idx]
		for i := range temp.Spec.ParentRefs {
			temp.Spec.ParentRefs[i].Namespace = ptr.To(gatewayv1.Namespace(GatewayNamespace))
			// temp.Spec.ParentRefs[i].Name = gatewayv1.ObjectName(GatewayName)
		}
		gatewayResources.HTTPRoutes[idx] = temp
	}

	ruleGroups := common.GetRuleGroups(ingressList)
	for _, rg := range ruleGroups {
		ingressPathsByMatchKey, errs := getPathsByMatchGroups(rg)
		if len(errs) > 0 {
			return i2gw.GatewayResources{}, errs
		}

		for _, paths := range ingressPathsByMatchKey {
			path := paths[0]
			key := types.NamespacedName{Namespace: path.ingress.Namespace, Name: common.RouteName(rg.Name, rg.Host)}
			httpRoute, ok := gatewayResources.HTTPRoutes[key]
			if !ok {
				continue
			}

			for _, handler := range c.FeatureHandler {
				parseErrs := handler(&httpRoute, paths)
				errs = append(errs, parseErrs...)
			}
			gatewayResources.HTTPRoutes[key] = httpRoute
		}
	}

	return gatewayResources, errs
}
