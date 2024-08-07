package higress

import (
	"fmt"
	"net/url"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type ingressPath struct {
	ingress networkingv1.Ingress

	ruleType string
	path     networkingv1.HTTPIngressPath
	extra    *extra
}

type extra struct {
	canary    *canaryConfig
	headerMod *headerModConfig
	rewrite   *rewriteConfig
	redirect  *redirectConfig
	mirror    *mirrorConfig
	timeout   *timeoutConfig
}

type pathMatchKey string

func ConvertPathType(nwPathType *networkingv1.PathType) *gatewayv1.PathMatchType {
	if nwPathType == nil {
		return nil
	}

	var gwPathMatchType gatewayv1.PathMatchType
	switch *nwPathType {
	case networkingv1.PathTypeExact:
		gwPathMatchType = gatewayv1.PathMatchExact
	case networkingv1.PathTypePrefix:
		gwPathMatchType = gatewayv1.PathMatchPathPrefix
	default:
		return nil
	}

	return &gwPathMatchType
}

func getPathMatchType(pathType networkingv1.PathType) gatewayv1.PathMatchType {
	switch pathType {
	case networkingv1.PathTypeExact:
		return gatewayv1.PathMatchExact
	case networkingv1.PathTypePrefix:
		return gatewayv1.PathMatchPathPrefix
	case networkingv1.PathTypeImplementationSpecific:
		return gatewayv1.PathMatchRegularExpression
	default:
		// 返回一个默认值或处理未知的pathType
		return gatewayv1.PathMatchType("")
	}
}

func getHeaderMatchTypeExact() *gatewayv1.HeaderMatchType {
	exact := gatewayv1.HeaderMatchExact
	return &exact
}

func getHeaderMatchTypeRegex() *gatewayv1.HeaderMatchType {
	regex := gatewayv1.HeaderMatchRegularExpression
	return &regex
}

func toNamespacePointer(namespace string) *gatewayv1.Namespace {
	ns := gatewayv1.Namespace(namespace)
	return &ns
}

func toPortNumber(port int) *gatewayv1.PortNumber {
	p := gatewayv1.PortNumber(port)
	return &p

}

func singleBackendRuleExists(httpRoute *gatewayv1.HTTPRoute, path *ingressPath) *gatewayv1.HTTPRouteRule {
	for i, rule := range httpRoute.Spec.Rules {
		if len(rule.BackendRefs) == 1 {
			if matchRule(rule, path) {
				return &httpRoute.Spec.Rules[i]
			}
		}
	}
	return nil
}

func findRuleByPath(httpRoute *gatewayv1.HTTPRoute, path ingressPath) *gatewayv1.HTTPRouteRule {
	for i, rule := range httpRoute.Spec.Rules {
		if matchRule(rule, &path) {
			return &httpRoute.Spec.Rules[i]
		}
	}
	return nil
}

func matchRule(rule gatewayv1.HTTPRouteRule, path *ingressPath) bool {
	for _, match := range rule.Matches {
		if match.Path == nil || match.Path.Type == nil || match.Path.Value == nil {
			continue
		}
		if !isPathMatch(&match, path) {
			continue
		}
		if matchBackendRefs(rule.BackendRefs, path) {
			return true
		}
	}
	return false
}

func isPathMatch(match *gatewayv1.HTTPRouteMatch, path *ingressPath) bool {
	if *match.Path.Value != path.path.Path {
		return false
	}
	matchType := *match.Path.Type
	switch matchType {
	case gatewayv1.PathMatchExact:
		return *path.path.PathType == networkingv1.PathTypeExact
	case gatewayv1.PathMatchPathPrefix:
		return *path.path.PathType == networkingv1.PathTypePrefix
	default:
		return false
	}
}

func matchBackendRefs(backendRefs []gatewayv1.HTTPBackendRef, path *ingressPath) bool {
	for _, backend := range backendRefs {
		if backend.BackendRef.Name == gatewayv1.ObjectName(path.path.Backend.Service.Name) &&
			backend.BackendRef.Port != nil &&
			*backend.BackendRef.Port == gatewayv1.PortNumber(path.path.Backend.Service.Port.Number) {
			return true
		}
	}
	return false
}

func printRules(httpRoute *gatewayv1.HTTPRoute) {
	for k, rule := range httpRoute.Spec.Rules {
		fmt.Println("----------------")
		fmt.Println("Rule: ", k)
		if rule.Matches != nil {
			for i, match := range rule.Matches {
				fmt.Println("Match: ", i)
				if match.Path != nil {
					fmt.Println("Path: ", match.Path)
				}
				if match.Headers != nil {
					for j, header := range match.Headers {
						fmt.Println("Header: ", j)
						fmt.Println("Name: ", header.Name)
						fmt.Println("Value: ", header.Value)
					}
				}
			}

		}
		if rule.BackendRefs != nil {
			for i, backend := range rule.BackendRefs {
				fmt.Println("Backend: ", i)
				fmt.Println("Name: ", backend.Name)
				fmt.Println("Weight: ", backend.Weight)
			}
		}
	}
	fmt.Println("----------------")
}

type createHTTPRouteRuleParam struct {
	matchs      []gatewayv1.HTTPRouteMatch
	filters     []gatewayv1.HTTPRouteFilter
	backendRefs []gatewayv1.HTTPBackendRef
	timeouts    *gatewayv1.HTTPRouteTimeouts
}

func deleteBackendNew(httpRoute *gatewayv1.HTTPRoute, path *ingressPath) *field.Error {
	for i := 0; i < len(httpRoute.Spec.Rules); i++ {
		rule := &httpRoute.Spec.Rules[i]
		if !hasMatches(rule.Matches) {
			continue
		}

		for _, match := range rule.Matches {
			if !isMatchPath(match, path) || !hasBackendRefs(rule.BackendRefs) {
				continue
			}

			for j := 0; j < len(rule.BackendRefs); j++ {
				ref := &rule.BackendRefs[j]
				if isMatchingBackendRef(ref, path) {
					if isRemovableRule(rule) {
						httpRoute.Spec.Rules = append(httpRoute.Spec.Rules[:i], httpRoute.Spec.Rules[i+1:]...)
						// i-- // Adjust the index after removing an element
						return nil
					}
					rule.BackendRefs = append(rule.BackendRefs[:j], rule.BackendRefs[j+1:]...)
					return nil
				}
			}
		}
	}

	return nil
}

func hasMatches(matches []gatewayv1.HTTPRouteMatch) bool {
	// The Go programming language has a convenient property where calling len() on a nil slice returns 0.
	return len(matches) > 0
}

func isMatchPath(match gatewayv1.HTTPRouteMatch, path *ingressPath) bool {
	return match.Path != nil && *match.Path.Value == path.path.Path && *match.Path.Type == getPathMatchType(*path.path.PathType)
}

func hasBackendRefs(backendRefs []gatewayv1.HTTPBackendRef) bool {
	return len(backendRefs) > 0
}

func isMatchingBackendRef(ref *gatewayv1.HTTPBackendRef, path *ingressPath) bool {
	// networkingv1.IngressServiceBackend仅包含ServiceName和ServicePort，故不比较namespace
	// 参考Github.com/kubernetes-sigs/ingress2gateway/blob/main/pkg/i2gw/providers/common/converter.go#L393
	return ref.Name == gatewayv1.ObjectName(path.path.Backend.Service.Name) &&
		(ref.Port != nil && *ref.Port == (gatewayv1.PortNumber)(path.path.Backend.Service.Port.Number))
}

func isRemovableRule(rule *gatewayv1.HTTPRouteRule) bool {
	return len(rule.BackendRefs) == 1 && !hasFilters(rule.Filters) && len(rule.Matches) == 1
}

func hasFilters(filters []gatewayv1.HTTPRouteFilter) bool {
	return len(filters) > 0
}

func createHTTPRouteRule(param createHTTPRouteRuleParam) *gatewayv1.HTTPRouteRule {
	return &gatewayv1.HTTPRouteRule{
		Matches:     param.matchs,
		Filters:     param.filters,
		BackendRefs: param.backendRefs,
		Timeouts:    param.timeouts,
	}
}

func createHTTPRouteMatch(path *ingressPath) gatewayv1.HTTPRouteMatch {
	return gatewayv1.HTTPRouteMatch{
		Path: &gatewayv1.HTTPPathMatch{
			Type:  ConvertPathType(path.path.PathType),
			Value: ptr.To(path.path.Path),
		},
	}
}

func isValidURL(s string) error {
	u, err := url.Parse(s)
	if err != nil {
		return err
	}

	if !strings.HasPrefix(u.Scheme, "http") {
		return fmt.Errorf("only http and https are valid protocols (%v)", u.Scheme)
	}

	return nil
}

func isPathValid(path string) *field.Error {
	if err := groupCaptureUsed(path); err != nil {
		return err
	}
	return nil
}

func groupCaptureUsed(path string) *field.Error {
	if strings.Contains(path, "(") && strings.Contains(path, ")") {
		return field.Invalid(field.NewPath("metadata", "annotations"), path, "group capture not supported")
	}
	return nil
}
