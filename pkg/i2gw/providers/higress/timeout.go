package higress

import (
	"fmt"
	"strconv"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	HigressTimeout = "timeout"
)

func timeoutFeature(ingresses []networkingv1.Ingress, gatewayResources *i2gw.GatewayResources) field.ErrorList {
	ruleGroups := common.GetRuleGroups(ingresses)
	var errors field.ErrorList

	for _, rg := range ruleGroups {
		ingressPathsByMatchKey, errs := getPathsByMatchGroups(rg, AnnotationTimeout)
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

			applyHTTPRouteWithTimeout(&httpRoute, paths)
			gatewayResources.HTTPRoutes[key] = httpRoute
		}
	}

	return errors
}

func applyHTTPRouteWithTimeout(httpRoute *gatewayv1.HTTPRoute, paths []ingressPath) field.ErrorList {
	var errors field.ErrorList
	var timeoutPaths []ingressPath

	for _, path := range paths {
		if path.extra != nil && path.extra.timeout != nil && path.extra.timeout.configExsits() {
			timeoutPaths = append(timeoutPaths, path)
		}
	}

	for _, path := range timeoutPaths {
		timeout := path.extra.timeout
		var timeoutConf gatewayv1.HTTPRouteTimeouts

		t := toDuration(timeout.timeout)
		timeoutConf.Request = &t

		rule := findRuleByPath(httpRoute, path)
		if rule == nil {
			errors = append(errors, field.Invalid(field.NewPath("metadata", "annotations"), paths, "rule not found"))
			continue
		}
		rule.Timeouts = &timeoutConf
	}

	return errors
}

type timeoutConfig struct {
	timeout int
}

func (t *timeoutConfig) Parse(ingress *networkingv1.Ingress) field.ErrorList {
	var errors field.ErrorList
	if timeout := findAnnotationValue(ingress.Annotations, HigressTimeout); timeout != "" {
		timeoutInt, err := strconv.Atoi(timeout)
		if err != nil {
			errors = append(errors, field.Invalid(field.NewPath("metadata", "annotations"), timeout, "timeout must be an integer"))
		}
		t.timeout = timeoutInt
	}
	return errors
}

func (t *timeoutConfig) configExsits() bool {
	return t.timeout != 0
}

func toDuration(seconds int) gatewayv1.Duration {
	return gatewayv1.Duration(strconv.Itoa(seconds) + "s")
}
