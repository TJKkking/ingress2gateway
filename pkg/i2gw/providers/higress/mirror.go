package higress

import (
	"strconv"
	"strings"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	MirrorTargetService = "mirror-target-service"
)

type mirrorConfig struct {
	namespace     string
	targetService string
	port          int
}

func applyHTTPRouteWithMirror(httpRoute *gatewayv1.HTTPRoute, paths []ingressPath) field.ErrorList {
	var errors field.ErrorList
	var mirrorPaths []ingressPath

	for _, path := range paths {
		if path.extra != nil && path.extra.mirror != nil && path.extra.mirror.configExsits() {
			mirrorPaths = append(mirrorPaths, path)
		}
	}

	for i, path := range mirrorPaths {
		mirror := path.extra.mirror
		var mirrorFilter gatewayv1.HTTPRequestMirrorFilter
		var backend gatewayv1.BackendObjectReference

		if mirror.namespace != "" {
			backend.Namespace = toNamespacePointer(mirror.namespace)
		}
		if mirror.port != 0 {
			backend.Port = toPortNumber(mirror.port)
		}
		backend.Name = gatewayv1.ObjectName(mirror.targetService)
		mirrorFilter.BackendRef = backend
		backendRef, err := common.ToBackendRef(path.path.Backend, field.NewPath("paths", "backends").Index(i))
		if err != nil {
			errors = append(errors, err)
			continue
		}
		applyByMirror(httpRoute, &path, backendRef, &mirrorFilter)
	}

	return errors
}

func applyByMirror(httpRoute *gatewayv1.HTTPRoute, path *ingressPath, backendRef *gatewayv1.BackendRef, mirrorFilter *gatewayv1.HTTPRequestMirrorFilter) *field.Error {
	if rule := singleBackendRuleExists(httpRoute, path); rule != nil {
		rule.Filters = append(rule.Filters, gatewayv1.HTTPRouteFilter{
			Type:          "RequestMirror",
			RequestMirror: mirrorFilter,
		})
		return nil
	} else {
		match := gatewayv1.HTTPRouteMatch{
			Path: &gatewayv1.HTTPPathMatch{
				Type:  ConvertPathType(path.path.PathType),
				Value: ptr.To(path.path.Path),
			},
		}

		deleteBackendNew(httpRoute, path)
		httpRoute.Spec.Rules = append(httpRoute.Spec.Rules, gatewayv1.HTTPRouteRule{
			Matches:     []gatewayv1.HTTPRouteMatch{match},
			Filters:     []gatewayv1.HTTPRouteFilter{{Type: "RequestMirror", RequestMirror: mirrorFilter}},
			BackendRefs: []gatewayv1.HTTPBackendRef{{BackendRef: *backendRef}},
		})
	}

	return nil
}

func (m *mirrorConfig) Parse(ingress *networkingv1.Ingress) field.ErrorList {
	var errors field.ErrorList

	if targetService := findAnnotationValue(ingress.Annotations, MirrorTargetService); targetService != "" {
		// m.targetService = targetService
		info, err := parseServiceInfo(targetService)
		if err != nil {
			errors = append(errors, field.Invalid(field.NewPath("metadata", "annotations"), MirrorTargetService, err.Error()))
		} else {
			m.namespace = info.namespace
			m.targetService = info.targetService
			m.port = info.port
		}
	}

	return errors
}

func (m *mirrorConfig) configExsits() bool {
	return m.targetService != ""
}

func parseServiceInfo(service string) (mirrorConfig, error) {
	var m mirrorConfig
	parts := strings.Split(service, ":")
	namespaceName := strings.Split(parts[0], "/")
	if len(namespaceName) == 2 {
		m.namespace = namespaceName[0]
		m.targetService = namespaceName[1]
	} else if len(namespaceName) == 1 {
		m.targetService = namespaceName[0]
	} else {
		return m, field.Invalid(field.NewPath("metadata", "annotations"), service, "invalid service format")
	}
	var port int
	var err error
	if len(parts) == 2 {
		port, err = strconv.Atoi(parts[1])
		if err != nil {
			return m, field.Invalid(field.NewPath("metadata", "annotations"), service, err.Error())
		}
		m.port = port
	}

	return m, nil
}
