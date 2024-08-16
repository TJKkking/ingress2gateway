package higress

import (
	"fmt"
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
	RewriteTarget = "rewrite-target"
	UpstreamVhost = "upstream-vhost"
)

type rewriteConfig struct {
	hostname string
	path     string
}

func rewriteFeature(ingresses []networkingv1.Ingress, gatewayResources *i2gw.GatewayResources) field.ErrorList {
	ruleGroups := common.GetRuleGroups(ingresses)
	var errors field.ErrorList

	for _, rg := range ruleGroups {
		ingressPathsByMatchKey, errs := getPathsByMatchGroups(rg, AnnotationRewrite)
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

			applyHTTPRouteWithRewrite(&httpRoute, paths)
			gatewayResources.HTTPRoutes[key] = httpRoute
		}
	}

	return errors
}

func applyHTTPRouteWithRewrite(httpRoute *gatewayv1.HTTPRoute, paths []ingressPath) field.ErrorList {
	var errors field.ErrorList
	var rewritePaths []ingressPath

	for _, path := range paths {
		if path.extra != nil && path.extra.rewrite != nil && path.extra.rewrite.configExsits() {
			// TODO: keep capture groups or not? no.
			if err := isPathValid(path.path.Path); err != nil {
				errors = append(errors, err)
				continue
			}
			rewritePaths = append(rewritePaths, path)
		}
	}

	for i, path := range rewritePaths {
		rewrite := path.extra.rewrite
		var URLRewrite gatewayv1.HTTPURLRewriteFilter

		if rewrite.hostname != "" {
			URLRewrite.Hostname = toHostname(rewrite.hostname)
		}
		if rewrite.path != "" {
			path := toPathModifier(rewrite.path)
			URLRewrite.Path = path
		}

		backendRef, err := common.ToBackendRef(path.path.Backend, field.NewPath("paths", "backends").Index(i))
		if err != nil {
			errors = append(errors, err)
			continue
		}
		applyByRewrite(httpRoute, &path, backendRef, &URLRewrite)
	}

	return errors
}

func toHostname(hostname string) *gatewayv1.PreciseHostname {
	return ptr.To(gatewayv1.PreciseHostname(strings.TrimSpace(hostname)))
}

func toPathModifier(path string) *gatewayv1.HTTPPathModifier {
	return &gatewayv1.HTTPPathModifier{
		Type:               gatewayv1.PrefixMatchHTTPPathModifier,
		ReplacePrefixMatch: &path,
	}
}

func applyByRewrite(httpRoute *gatewayv1.HTTPRoute, path *ingressPath, backendRef *gatewayv1.BackendRef, URLRewrite *gatewayv1.HTTPURLRewriteFilter) *field.Error {
	if rule := singleBackendRuleExists(httpRoute, path); rule != nil {
		rule.Filters = append(rule.Filters, gatewayv1.HTTPRouteFilter{
			Type:       gatewayv1.HTTPRouteFilterURLRewrite,
			URLRewrite: URLRewrite,
		})
		return nil
	} else {
		deleteBackend(httpRoute, path)
		httpRoute.Spec.Rules = append(httpRoute.Spec.Rules, *createHTTPRouteRule(createHTTPRouteRuleParam{
			matchs: []gatewayv1.HTTPRouteMatch{createHTTPRouteMatch(path)},
			filters: []gatewayv1.HTTPRouteFilter{
				{
					Type:       gatewayv1.HTTPRouteFilterURLRewrite,
					URLRewrite: URLRewrite,
				},
			},
			backendRefs: []gatewayv1.HTTPBackendRef{{BackendRef: *backendRef}},
		}))
	}

	return nil
}

func (r *rewriteConfig) Parse(ingress *networkingv1.Ingress) field.ErrorList {
	if hostname := findAnnotationValue(ingress.Annotations, UpstreamVhost); hostname != "" {
		r.hostname = strings.TrimSpace(hostname)
	}
	if path := findAnnotationValue(ingress.Annotations, RewriteTarget); path != "" {
		r.path = strings.TrimSpace(path)
	}
	return nil
}

func (r *rewriteConfig) configExsits() bool {
	return r.hostname != "" || r.path != ""
}
