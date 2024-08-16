package higress

import (
	"fmt"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	// request
	RequestHeaderAdd    = "request-header-control-add"
	RequestHeaderUpdate = "request-header-control-update"
	RequestHeaderRemove = "request-header-control-remove"
)

type requestHeaderModConfig struct {
	add    map[string]string
	update map[string]string
	remove []string
}

func requestHeaderModFeature(ingresses []networkingv1.Ingress, gatewayResources *i2gw.GatewayResources) field.ErrorList {
	ruleGroups := common.GetRuleGroups(ingresses)
	var errors field.ErrorList

	for _, rg := range ruleGroups {
		ingressPathsByMatchKey, errs := getPathsByMatchGroups(rg, AnnotationRequestHeaderMod)
		if len(errs) > 0 {
			return errs
		}

		for _, paths := range ingressPathsByMatchKey {
			path := paths[0]
			key := types.NamespacedName{Namespace: path.ingress.Namespace, Name: common.RouteName(rg.Name, rg.Host)}
			httpRoute, ok := gatewayResources.HTTPRoutes[key]
			if !ok {
				errors = append(errors, field.InternalError(field.NewPath("HTTPRoutes"), fmt.Errorf("http route %s not found", key)))
				continue
			}

			applyHTTPRouteWithRequestHeaderMod(&httpRoute, paths)
			gatewayResources.HTTPRoutes[key] = httpRoute
		}
	}

	return errors
}

func applyHTTPRouteWithRequestHeaderMod(httpRoute *gatewayv1.HTTPRoute, paths []ingressPath) field.ErrorList {
	var headerPath []ingressPath
	var errors field.ErrorList
	for _, path := range paths {
		if path.extra != nil && path.extra.requestHeaderMod != nil && path.extra.requestHeaderMod.configExsits() {
			headerPath = append(headerPath, path)
		}
	}
	for i, path := range headerPath {
		var gwHTTPRouteFilters []gatewayv1.HTTPRouteFilter
		var headerFilter gatewayv1.HTTPHeaderFilter
		headerMod := path.extra.requestHeaderMod
		if headerMod.add != nil {
			var addHeaders []gatewayv1.HTTPHeader
			for key, value := range headerMod.add {
				addHeaders = append(addHeaders, gatewayv1.HTTPHeader{
					Name:  gatewayv1.HTTPHeaderName(key),
					Value: value,
				})
			}
			headerFilter.Add = addHeaders
		}
		if headerMod.update != nil {
			var updateHeaders []gatewayv1.HTTPHeader
			for key, value := range headerMod.update {
				updateHeaders = append(updateHeaders, gatewayv1.HTTPHeader{
					Name:  gatewayv1.HTTPHeaderName(key),
					Value: value,
				})
			}
			headerFilter.Set = updateHeaders
		}
		if headerMod.remove != nil {
			var removeHeaders []string
			removeHeaders = append(removeHeaders, headerMod.remove...)
			headerFilter.Remove = removeHeaders
		}

		gwHTTPRouteFilters = append(gwHTTPRouteFilters, gatewayv1.HTTPRouteFilter{
			Type:                  gatewayv1.HTTPRouteFilterRequestHeaderModifier,
			RequestHeaderModifier: &headerFilter,
		})
		backendRef, err := common.ToBackendRef(path.path.Backend, field.NewPath("paths", "backends").Index(i))
		if err != nil {
			errors = append(errors, err)
			continue
		}
		errs := applyByRequestHeaderMod(httpRoute, &path, backendRef, gwHTTPRouteFilters)
		if errs != nil {
			errors = append(errors, errs)
		}
	}
	return errors
}

func applyByRequestHeaderMod(httpRoute *gatewayv1.HTTPRoute, path *ingressPath, backendRef *gatewayv1.BackendRef, gwHTTPRouteFilters []gatewayv1.HTTPRouteFilter) *field.Error {
	if rule := singleBackendRuleExists(httpRoute, path); rule != nil {
		rule.Filters = append(rule.Filters, gwHTTPRouteFilters...)
		return nil
	} else {
		match := gatewayv1.HTTPRouteMatch{
			Path: &gatewayv1.HTTPPathMatch{
				Type:  ConvertPathType(path.path.PathType),
				Value: ptr.To(path.path.Path),
			},
		}
		deleteBackend(httpRoute, path)
		httpRoute.Spec.Rules = append(httpRoute.Spec.Rules, *createHTTPRouteRule(createHTTPRouteRuleParam{
			filters:     gwHTTPRouteFilters,
			backendRefs: []gatewayv1.HTTPBackendRef{{BackendRef: *backendRef}},
			matchs:      []gatewayv1.HTTPRouteMatch{match},
		}))
	}

	return nil
}

func (h *requestHeaderModConfig) Parse(ingress *networkingv1.Ingress) field.ErrorList {
	var errors field.ErrorList
	if hAdd := findAnnotationValue(ingress.Annotations, RequestHeaderAdd); hAdd != "" {
		ha, err := convertAddOrUpdate(hAdd)
		if err != nil {
			errors = append(errors, err...)
		} else {
			h.add = ha
		}
	}
	if hUpdate := findAnnotationValue(ingress.Annotations, RequestHeaderUpdate); hUpdate != "" {
		hu, err := convertAddOrUpdate(hUpdate)
		if err != nil {
			errors = append(errors, err...)
		} else {
			h.update = hu
		}
	}
	if hRemove := findAnnotationValue(ingress.Annotations, RequestHeaderRemove); hRemove != "" {
		h.remove = splitBySeparator(hRemove, ",")
	}

	if len(errors) > 0 {
		return errors
	}
	return nil
}

func (h *requestHeaderModConfig) configExsits() bool {
	return h.add != nil || h.update != nil || h.remove != nil
}
