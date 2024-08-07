package higress

import (
	"strconv"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	HigressTimeout = "timeout"
)

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
