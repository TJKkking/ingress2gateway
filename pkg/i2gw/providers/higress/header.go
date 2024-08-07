package higress

import (
	"fmt"
	"regexp"
	"strings"

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

// type annoHeader struct{}

type headerModConfig struct {
	add    map[string]string
	update map[string]string
	remove []string
}

func headerModFeature(ingresses []networkingv1.Ingress, gatewayResources *i2gw.GatewayResources) field.ErrorList {
	ruleGroups := common.GetRuleGroups(ingresses)
	var errors field.ErrorList

	for _, rg := range ruleGroups {
		ingressPathsByMatchKey, errs := getPathsByMatchGroups(rg)
		if len(errs) > 0 {
			return errs
		}

		for _, paths := range ingressPathsByMatchKey {
			path := paths[0]
			key := types.NamespacedName{Namespace: path.ingress.Namespace, Name: common.RouteName(rg.Name, rg.Host)}
			httpRoute, ok := gatewayResources.HTTPRoutes[key]
			if !ok {
				fmt.Println("httpRoute not found")
				continue
			}

			applyHTTPRouteWithHeaderMod(&httpRoute, paths)
			gatewayResources.HTTPRoutes[key] = httpRoute
		}
	}

	return errors
}

func applyHTTPRouteWithHeaderMod(httpRoute *gatewayv1.HTTPRoute, paths []ingressPath) field.ErrorList {
	var headerPath []ingressPath
	var errors field.ErrorList

	for _, path := range paths {
		if path.extra != nil && path.extra.headerMod != nil && path.extra.headerMod.configExsits() {
			headerPath = append(headerPath, path)
		}
	}

	for i, path := range headerPath {
		var gwHTTPRouteFilters []gatewayv1.HTTPRouteFilter
		var headerFilter gatewayv1.HTTPHeaderFilter
		headerMod := path.extra.headerMod

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
		errs := applyByHeaderMod(httpRoute, &path, backendRef, gwHTTPRouteFilters)
		if errs != nil {
			errors = append(errors, errs)
		}
	}

	return errors
}

func applyByHeaderMod(httpRoute *gatewayv1.HTTPRoute, path *ingressPath, backendRef *gatewayv1.BackendRef, gwHTTPRouteFilters []gatewayv1.HTTPRouteFilter) *field.Error {
	// fmt.Println("bakendRef Name is: ", backendRef.Name)
	if rule := singleBackendRuleExists(httpRoute, path); rule != nil {
		rule.Filters = append(rule.Filters, gwHTTPRouteFilters...)
		return nil
	} else {
		// fmt.Println("New Rule")
		match := gatewayv1.HTTPRouteMatch{
			Path: &gatewayv1.HTTPPathMatch{
				Type:  ConvertPathType(path.path.PathType),
				Value: ptr.To(path.path.Path),
			},
		}
		deleteBackendNew(httpRoute, path)
		httpRoute.Spec.Rules = append(httpRoute.Spec.Rules, *createHTTPRouteRule(createHTTPRouteRuleParam{
			filters:     gwHTTPRouteFilters,
			backendRefs: []gatewayv1.HTTPBackendRef{{BackendRef: *backendRef}},
			matchs:      []gatewayv1.HTTPRouteMatch{match},
		}))
	}

	return nil
}

func (h *headerModConfig) Parse(ingress *networkingv1.Ingress) field.ErrorList {
	if hAdd := findAnnotationValue(ingress.Annotations, RequestHeaderAdd); hAdd != "" {
		h.add = convertAddOrUpdate(hAdd)
	}
	if hUpdate := findAnnotationValue(ingress.Annotations, RequestHeaderUpdate); hUpdate != "" {
		h.update = convertAddOrUpdate(hUpdate)
	}
	if hRemove := findAnnotationValue(ingress.Annotations, RequestHeaderRemove); hRemove != "" {
		h.remove = splitBySeparator(hRemove, ",")
	}

	return nil
}

func (h *headerModConfig) configExsits() bool {
	return h.add != nil || h.update != nil || h.remove != nil
}

var pattern = regexp.MustCompile(`\s+`)

func convertAddOrUpdate(headers string) map[string]string {
	result := map[string]string{}
	parts := strings.Split(headers, "\n")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		keyValue := pattern.Split(part, 2)
		if len(keyValue) != 2 {
			// errors.New("invalid header format")
			fmt.Println("invalid header format")
			continue
		}
		key := trimQuotes(strings.TrimSpace(keyValue[0]))
		value := trimQuotes(strings.TrimSpace(keyValue[1]))
		result[key] = value
	}
	return result
}
