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
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	PathTypeExact  = gatewayv1.PathMatchExact
	PathTypePrefix = gatewayv1.PathMatchPathPrefix
)

// converter implements the ToGatewayAPI function of i2gw.ResourceConverter interface.
type converter struct {
	featureParsers []i2gw.FeatureParser
	FeatureHandler []FeatureHandler
}

// newConverter returns an higress converter instance.
// Note: The order in which the Paser/Handler is executed may result in different outputs, change it with caution.
func newConverter() *converter {
	return &converter{
		featureParsers: []i2gw.FeatureParser{
			canaryFeature,
			headerModFeature,
		},
		FeatureHandler: []FeatureHandler{
			applyHTTPRouteWithCanary,
			applyHTTPRouteWithHeaderMod,
			applyHTTPRouteWithRewrite,
			applyHTTPRouteWithMirror,
			applyHTTPRouteWithTimeout,
			applyHTTPRouteWithRedirect,
		},
	}
}

// Converter for ingress-nginx to gateway resources.
func (c *converter) convertOlds(storage *storage) (i2gw.GatewayResources, field.ErrorList) {

	// TODO(liorliberman) temporary until we decide to change ToGateway and featureParsers to get a map of [types.NamespacedName]*networkingv1.Ingress instead of a list
	ingressList := storage.Ingresses.List()

	// Convert plain ingress resources to gateway resources, ignoring all
	// provider-specific features.
	gatewayResources, errs := common.ToGateway(ingressList, i2gw.ProviderImplementationSpecificOptions{})
	if len(errs) > 0 {
		return i2gw.GatewayResources{}, errs
	}

	for _, parseFeatureFunc := range c.featureParsers {
		// Apply the feature parsing function to the gateway resources, one by one.
		parseErrs := parseFeatureFunc(ingressList, &gatewayResources)
		// Append the parsing errors to the error list.
		errs = append(errs, parseErrs...)
	}

	return gatewayResources, errs
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
