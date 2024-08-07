package higress

import (
	"fmt"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func ToBackendRef(ib networkingv1.IngressBackend, path *field.Path) (*gatewayv1.BackendRef, *field.Error) {
	if ib.Service != nil {
		if ib.Service.Port.Name != "" {
			fieldPath := path.Child("service", "port")
			return nil, field.Invalid(fieldPath, "name", fmt.Sprintf("named ports not supported: %s", ib.Service.Port.Name))
		}
		return &gatewayv1.BackendRef{
			BackendObjectReference: gatewayv1.BackendObjectReference{
				Name: gatewayv1.ObjectName(ib.Service.Name),
				Port: (*gatewayv1.PortNumber)(&ib.Service.Port.Number),
			},
		}, nil
	}
	return &gatewayv1.BackendRef{
		BackendObjectReference: gatewayv1.BackendObjectReference{
			Group: (*gatewayv1.Group)(ib.Resource.APIGroup),
			Kind:  (*gatewayv1.Kind)(&ib.Resource.Kind),
			Name:  gatewayv1.ObjectName(ib.Resource.Name),
		},
	}, nil
}

func trimQuotes(s string) string {
	if len(s) >= 2 {
		if s[0] == '"' && s[len(s)-1] == '"' {
			return s[1 : len(s)-1]
		}
		if s[0] == '\'' && s[len(s)-1] == '\'' {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func splitBySeparator(content, separator string) []string {
	var result []string
	parts := strings.Split(content, separator)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		result = append(result, part)
	}
	return result
}
