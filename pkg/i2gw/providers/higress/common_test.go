package higress

import (
	"reflect"
	"testing"

	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestCreateHTTPRouteRule(t *testing.T) {
	tests := []struct {
		name   string
		param  createHTTPRouteRuleParam
		expect *gatewayv1.HTTPRouteRule
	}{
		{
			name: "All fields provided",
			param: createHTTPRouteRuleParam{
				matchs:      []gatewayv1.HTTPRouteMatch{{}},
				filters:     []gatewayv1.HTTPRouteFilter{{}},
				backendRefs: []gatewayv1.HTTPBackendRef{{}},
				timeouts:    &gatewayv1.HTTPRouteTimeouts{},
			},
			expect: &gatewayv1.HTTPRouteRule{
				Matches:     []gatewayv1.HTTPRouteMatch{{}},
				Filters:     []gatewayv1.HTTPRouteFilter{{}},
				BackendRefs: []gatewayv1.HTTPBackendRef{{}},
				Timeouts:    &gatewayv1.HTTPRouteTimeouts{},
			},
		},
		{
			name: "Nil slices",
			param: createHTTPRouteRuleParam{
				matchs:      nil,
				filters:     nil,
				backendRefs: nil,
				timeouts:    nil,
			},
			expect: &gatewayv1.HTTPRouteRule{
				Matches:     []gatewayv1.HTTPRouteMatch{},
				Filters:     []gatewayv1.HTTPRouteFilter{},
				BackendRefs: []gatewayv1.HTTPBackendRef{},
				Timeouts:    nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := createHTTPRouteRule(tt.param)
			if !reflect.DeepEqual(result, tt.expect) {
				t.Errorf("expected %+v, got %+v", tt.expect, result)
			}
		})
	}
}
