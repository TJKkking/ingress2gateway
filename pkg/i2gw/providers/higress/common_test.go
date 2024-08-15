package higress

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	networkingv1 "k8s.io/api/networking/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestConvertPathType(t *testing.T) {
	tests := []struct {
		name   string
		input  *networkingv1.PathType
		expect *gatewayv1.PathMatchType
	}{
		{
			name:   "Nil input",
			input:  nil,
			expect: nil,
		},
		{
			name:   "PathTypeExact",
			input:  ptrTo(networkingv1.PathTypeExact),
			expect: ptrTo(gatewayv1.PathMatchExact),
		},
		{
			name:   "PathTypePrefix",
			input:  ptrTo(networkingv1.PathTypePrefix),
			expect: ptrTo(gatewayv1.PathMatchPathPrefix),
		},
		{
			name:   "Invalid PathType",
			input:  ptrTo(networkingv1.PathType("Invalid")),
			expect: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertPathType(tt.input)
			if (result == nil && tt.expect != nil) || (result != nil && tt.expect == nil) || (result != nil && tt.expect != nil && *result != *tt.expect) {
				t.Errorf("ConvertPathType(%v) = %v, want %v", tt.input, result, tt.expect)
			}
		})
	}
}

func TestGetPathMatchType(t *testing.T) {
	tests := []struct {
		name   string
		input  networkingv1.PathType
		expect gatewayv1.PathMatchType
	}{
		{
			name:   "PathTypeExact",
			input:  networkingv1.PathTypeExact,
			expect: gatewayv1.PathMatchExact,
		},
		{
			name:   "PathTypePrefix",
			input:  networkingv1.PathTypePrefix,
			expect: gatewayv1.PathMatchPathPrefix,
		},
		{
			name:   "PathTypeImplementationSpecific",
			input:  networkingv1.PathTypeImplementationSpecific,
			expect: gatewayv1.PathMatchRegularExpression,
		},
		{
			name:   "Invalid PathType",
			input:  networkingv1.PathType("Invalid"),
			expect: gatewayv1.PathMatchType(""),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getPathMatchType(tt.input)
			if result != tt.expect {
				t.Errorf("getPathMatchType(%v) = %v, want %v", tt.input, result, tt.expect)
			}
		})
	}
}

func TestFindRuleByPath(t *testing.T) {
	tests := []struct {
		name      string
		httpRoute *gatewayv1.HTTPRoute
		path      ingressPath
		expect    *gatewayv1.HTTPRouteRule
	}{
		{
			name:      "No rules",
			httpRoute: &gatewayv1.HTTPRoute{Spec: gatewayv1.HTTPRouteSpec{Rules: []gatewayv1.HTTPRouteRule{}}},
			path:      ingressPath{},
			expect:    nil,
		},
		{
			name: "No matching rules",
			httpRoute: &gatewayv1.HTTPRoute{Spec: gatewayv1.HTTPRouteSpec{Rules: []gatewayv1.HTTPRouteRule{
				{Matches: []gatewayv1.HTTPRouteMatch{{Path: &gatewayv1.HTTPPathMatch{Type: ptrTo(gatewayv1.PathMatchExact), Value: ptrTo("/no-match")}}}},
			}}},
			path: ingressPath{path: networkingv1.HTTPIngressPath{
				Path:     "/test",
				PathType: ptrTo(networkingv1.PathTypeExact),
			}},
			expect: nil,
		},
		{
			name: "Matching rule found",
			httpRoute: &gatewayv1.HTTPRoute{Spec: gatewayv1.HTTPRouteSpec{Rules: []gatewayv1.HTTPRouteRule{{
				Matches:     []gatewayv1.HTTPRouteMatch{{Path: &gatewayv1.HTTPPathMatch{Type: ptrTo(gatewayv1.PathMatchExact), Value: ptrTo("/test")}}},
				BackendRefs: []gatewayv1.HTTPBackendRef{{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{Name: "test-service", Port: ptrTo(gatewayv1.PortNumber(80))}}}},
			},
			}}},
			path: ingressPath{path: networkingv1.HTTPIngressPath{
				Path:     "/test",
				PathType: ptrTo(networkingv1.PathTypeExact),
				Backend: networkingv1.IngressBackend{
					Service: &networkingv1.IngressServiceBackend{
						Name: "test-service",
						Port: networkingv1.ServiceBackendPort{Number: 80},
					},
				},
			}},
			expect: &gatewayv1.HTTPRouteRule{
				Matches:     []gatewayv1.HTTPRouteMatch{{Path: &gatewayv1.HTTPPathMatch{Type: ptrTo(gatewayv1.PathMatchExact), Value: ptrTo("/test")}}},
				BackendRefs: []gatewayv1.HTTPBackendRef{{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{Name: "test-service", Port: ptrTo(gatewayv1.PortNumber(80))}}}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findRuleByPath(tt.httpRoute, tt.path)
			if !apiequality.Semantic.DeepEqual(result, tt.expect) {
				t.Errorf("findRuleByPath() got = %v, want %v, diff: %s", result, tt.expect, cmp.Diff(tt.expect, result))
			}
		})
	}
}

func TestMatchRule(t *testing.T) {
	tests := []struct {
		name     string
		rule     gatewayv1.HTTPRouteRule
		path     *ingressPath
		expected bool
	}{
		{
			name: "Exact match with matching path",
			rule: createTestRule([]gatewayv1.HTTPRouteMatch{{
				Path: &gatewayv1.HTTPPathMatch{
					Type:  ptrTo(gatewayv1.PathMatchExact),
					Value: ptrTo("/exact"),
				},
			},
			}, []gatewayv1.HTTPBackendRef{{
				BackendRef: gatewayv1.BackendRef{
					BackendObjectReference: gatewayv1.BackendObjectReference{Name: "test-service", Port: ptrTo(gatewayv1.PortNumber(80))},
				},
			},
			}),
			path: &ingressPath{path: networkingv1.HTTPIngressPath{
				Path:     "/exact",
				PathType: ptrTo(networkingv1.PathTypeExact),
				Backend: networkingv1.IngressBackend{
					Service: &networkingv1.IngressServiceBackend{
						Name: "test-service",
						Port: networkingv1.ServiceBackendPort{Number: 80},
					},
				},
			}},
			expected: true,
		},
		{
			name: "Non-match due to backendRefs not matching",
			rule: createTestRule(nil, []gatewayv1.HTTPBackendRef{
				{
					BackendRef: gatewayv1.BackendRef{
						// Name: "service2",
						// Port: ptrTo(gatewayv1.PortNumber(80)),
						BackendObjectReference: gatewayv1.BackendObjectReference{Name: "service2", Port: ptrTo(gatewayv1.PortNumber(80))},
					},
				},
			}),
			path:     createTestPathWithBackend("/path", networkingv1.PathTypeExact, "service1", 80),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchRule(tt.rule, tt.path)
			if result != tt.expected {
				t.Errorf("%s: expected %t, got %t", tt.name, tt.expected, result)
			}
		})
	}
}

func createTestPathWithBackend(path string, pathType networkingv1.PathType, serviceName string, servicePort int32) *ingressPath {
	return &ingressPath{
		path: networkingv1.HTTPIngressPath{
			Path:     path,
			PathType: &pathType,
			Backend: networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{
					Name: serviceName,
					Port: networkingv1.ServiceBackendPort{
						Number: servicePort,
					},
				},
			},
		},
	}
}

func createTestRule(matches []gatewayv1.HTTPRouteMatch, backendRefs []gatewayv1.HTTPBackendRef) gatewayv1.HTTPRouteRule {
	return gatewayv1.HTTPRouteRule{
		Matches:     matches,
		BackendRefs: backendRefs,
	}
}

func ptrTo[T any](v T) *T {
	return &v
}

func TestIsPathMatch(t *testing.T) {
	tests := []struct {
		name     string
		match    gatewayv1.HTTPRouteMatch
		path     ingressPath
		expected bool
	}{
		{
			name: "Exact match",
			match: gatewayv1.HTTPRouteMatch{
				Path: &gatewayv1.HTTPPathMatch{
					Type:  ptrTo(gatewayv1.PathMatchExact),
					Value: ptrTo("/exact"),
				},
			},
			path: ingressPath{
				path: networkingv1.HTTPIngressPath{
					Path:     "/exact",
					PathType: ptrTo(networkingv1.PathTypeExact),
				},
			},
			expected: true,
		},
		{
			name: "Prefix match",
			match: gatewayv1.HTTPRouteMatch{
				Path: &gatewayv1.HTTPPathMatch{
					Type:  ptrTo(gatewayv1.PathMatchPathPrefix),
					Value: ptrTo("/prefix"),
				},
			},
			path: ingressPath{
				path: networkingv1.HTTPIngressPath{
					Path:     "/prefix",
					PathType: ptrTo(networkingv1.PathTypePrefix),
				},
			},
			expected: true,
		},
		{
			name: "Mismatched path values",
			match: gatewayv1.HTTPRouteMatch{
				Path: &gatewayv1.HTTPPathMatch{
					Type:  ptrTo(gatewayv1.PathMatchExact),
					Value: ptrTo("/exact"),
				},
			},
			path: ingressPath{
				path: networkingv1.HTTPIngressPath{
					Path:     "/different",
					PathType: ptrTo(networkingv1.PathTypeExact),
				},
			},
			expected: false,
		},
		{
			name: "Mismatched path types",
			match: gatewayv1.HTTPRouteMatch{
				Path: &gatewayv1.HTTPPathMatch{
					Type:  ptrTo(gatewayv1.PathMatchExact),
					Value: ptrTo("/exact"),
				},
			},
			path: ingressPath{
				path: networkingv1.HTTPIngressPath{
					Path:     "/exact",
					PathType: ptrTo(networkingv1.PathTypePrefix),
				},
			},
			expected: false,
		},
		{
			name: "Nil match path",
			match: gatewayv1.HTTPRouteMatch{
				Path: nil,
			},
			path: ingressPath{
				path: networkingv1.HTTPIngressPath{
					Path:     "/nil",
					PathType: ptrTo(networkingv1.PathTypeExact),
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPathMatch(&tt.match, &tt.path)
			if result != tt.expected {
				t.Errorf("%s: expected %t, got %t", tt.name, tt.expected, result)
			}
		})
	}
}

func TestDeleteBackend(t *testing.T) {
	tests := []struct {
		name      string
		httpRoute gatewayv1.HTTPRoute
		path      ingressPath
		expected  gatewayv1.HTTPRoute
	}{
		{
			name: "Delete matching backend",
			httpRoute: gatewayv1.HTTPRoute{
				Spec: gatewayv1.HTTPRouteSpec{
					Rules: []gatewayv1.HTTPRouteRule{
						{
							Matches: []gatewayv1.HTTPRouteMatch{
								{Path: &gatewayv1.HTTPPathMatch{Type: ptrTo(gatewayv1.PathMatchExact), Value: ptrTo("/test")}},
							},
							BackendRefs: []gatewayv1.HTTPBackendRef{
								// {Name: "test-service", Port: 80},
								// {Name: "other-service", Port: 80},
								{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{Name: "test-service", Port: ptrTo(gatewayv1.PortNumber(80))}}},
								{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{Name: "other-service", Port: ptrTo(gatewayv1.PortNumber(80))}}},
							},
						},
					},
				},
			},
			path: ingressPath{
				// serviceName: "test-service",
				// servicePort: 80,
				path: networkingv1.HTTPIngressPath{
					Path:     "/test",
					PathType: ptrTo(networkingv1.PathTypeExact),
					Backend: networkingv1.IngressBackend{
						Service: &networkingv1.IngressServiceBackend{
							Name: "test-service",
							Port: networkingv1.ServiceBackendPort{Number: 80},
						},
					},
				},
			},
			expected: gatewayv1.HTTPRoute{
				Spec: gatewayv1.HTTPRouteSpec{
					Rules: []gatewayv1.HTTPRouteRule{
						{
							Matches: []gatewayv1.HTTPRouteMatch{
								{Path: &gatewayv1.HTTPPathMatch{Type: ptrTo(gatewayv1.PathMatchExact), Value: ptrTo("/test")}},
							},
							BackendRefs: []gatewayv1.HTTPBackendRef{
								// {Name: "other-service", Port: 80},
								{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{Name: "other-service", Port: ptrTo(gatewayv1.PortNumber(80))}}},
							},
						},
					},
				},
			},
		},
		{
			name: "Attempt to delete non-existing backend",
			httpRoute: gatewayv1.HTTPRoute{
				Spec: gatewayv1.HTTPRouteSpec{
					Rules: []gatewayv1.HTTPRouteRule{
						{
							Matches: []gatewayv1.HTTPRouteMatch{
								{Path: &gatewayv1.HTTPPathMatch{Type: ptrTo(gatewayv1.PathMatchExact), Value: ptrTo("/test")}},
							},
							BackendRefs: []gatewayv1.HTTPBackendRef{
								{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{Name: "other-service", Port: ptrTo(gatewayv1.PortNumber(80))}}},
							},
						},
					},
				},
			},
			path: ingressPath{
				path: networkingv1.HTTPIngressPath{
					Path:     "/test",
					PathType: ptrTo(networkingv1.PathTypeExact),
					Backend: networkingv1.IngressBackend{
						Service: &networkingv1.IngressServiceBackend{
							Name: "non-existing-service",
							Port: networkingv1.ServiceBackendPort{Number: 80},
						},
					},
				},
			},
			expected: gatewayv1.HTTPRoute{
				Spec: gatewayv1.HTTPRouteSpec{
					Rules: []gatewayv1.HTTPRouteRule{
						{
							Matches: []gatewayv1.HTTPRouteMatch{
								{Path: &gatewayv1.HTTPPathMatch{Type: ptrTo(gatewayv1.PathMatchExact), Value: ptrTo("/test")}},
							},
							BackendRefs: []gatewayv1.HTTPBackendRef{
								{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{Name: "other-service", Port: ptrTo(gatewayv1.PortNumber(80))}}},
							},
						},
					},
				},
			},
		},
		{
			name: "Delete backend leading to rule removal",
			httpRoute: gatewayv1.HTTPRoute{
				Spec: gatewayv1.HTTPRouteSpec{
					Rules: []gatewayv1.HTTPRouteRule{
						{
							Matches: []gatewayv1.HTTPRouteMatch{
								{Path: &gatewayv1.HTTPPathMatch{Type: ptrTo(gatewayv1.PathMatchExact), Value: ptrTo("/test")}},
							},
							BackendRefs: []gatewayv1.HTTPBackendRef{
								{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{Name: "test-service", Port: ptrTo(gatewayv1.PortNumber(80))}}},
							},
						},
					},
				},
			},
			path: ingressPath{
				path: networkingv1.HTTPIngressPath{
					Path:     "/test",
					PathType: ptrTo(networkingv1.PathTypeExact),
					Backend: networkingv1.IngressBackend{
						Service: &networkingv1.IngressServiceBackend{
							Name: "test-service",
							Port: networkingv1.ServiceBackendPort{Number: 80},
						},
					},
				},
			},
			expected: gatewayv1.HTTPRoute{
				Spec: gatewayv1.HTTPRouteSpec{
					Rules: []gatewayv1.HTTPRouteRule{},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deleteBackend(&tt.httpRoute, &tt.path)
			if len(tt.httpRoute.Spec.Rules) != len(tt.expected.Spec.Rules) {
				t.Errorf("deleteBackend() want %d rules, got = %d rules: %v", len(tt.httpRoute.Spec.Rules), len(tt.expected.Spec.Rules), tt.httpRoute.Spec.Rules)
			} else {
				for i := range tt.httpRoute.Spec.Rules {
					if !apiequality.Semantic.DeepEqual(tt.httpRoute.Spec.Rules[i], tt.expected.Spec.Rules[i]) {
						t.Errorf("deleteBackend() got = %v, want %v, Diff: %s", tt.httpRoute, tt.expected, cmp.Diff(tt.expected, tt.httpRoute))
					}
				}
			}
		})
	}
}

func TestMatchBackendRefs(t *testing.T) {
	tests := []struct {
		name        string
		backendRefs []gatewayv1.HTTPBackendRef
		path        *ingressPath
		expect      bool
	}{
		{
			name: "Match by name and port",
			backendRefs: []gatewayv1.HTTPBackendRef{
				{
					BackendRef: gatewayv1.BackendRef{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: "test-service",
							Port: ptrTo(gatewayv1.PortNumber(80)),
						},
					},
				},
			},
			path: &ingressPath{
				path: networkingv1.HTTPIngressPath{
					Backend: networkingv1.IngressBackend{
						Service: &networkingv1.IngressServiceBackend{
							Name: "test-service",
							Port: networkingv1.ServiceBackendPort{Number: 80},
						},
					},
				},
			},
			expect: true,
		},
		{
			name: "Mismatch by name",
			backendRefs: []gatewayv1.HTTPBackendRef{
				{
					BackendRef: gatewayv1.BackendRef{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: "wrong-service",
							Port: ptrTo(gatewayv1.PortNumber(80)),
						},
					},
				},
			},
			path: &ingressPath{
				path: networkingv1.HTTPIngressPath{
					Backend: networkingv1.IngressBackend{
						Service: &networkingv1.IngressServiceBackend{
							Name: "test-service",
							Port: networkingv1.ServiceBackendPort{Number: 80},
						},
					},
				},
			},
			expect: false,
		},
		{
			name: "Mismatch by port",
			backendRefs: []gatewayv1.HTTPBackendRef{
				{
					BackendRef: gatewayv1.BackendRef{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: "test-service",
							Port: ptrTo(gatewayv1.PortNumber(8080)),
						},
					},
				},
			},
			path: &ingressPath{
				path: networkingv1.HTTPIngressPath{
					Backend: networkingv1.IngressBackend{
						Service: &networkingv1.IngressServiceBackend{
							Name: "test-service",
							Port: networkingv1.ServiceBackendPort{Number: 80},
						},
					},
				},
			},
			expect: false,
		},
		{
			name: "Multiple backendRefs, one matches",
			backendRefs: []gatewayv1.HTTPBackendRef{
				{
					BackendRef: gatewayv1.BackendRef{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: "wrong-service",
							Port: ptrTo(gatewayv1.PortNumber(80)),
						},
					},
				},
				{
					BackendRef: gatewayv1.BackendRef{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: "test-service",
							Port: ptrTo(gatewayv1.PortNumber(80)),
						},
					},
				},
			},
			path: &ingressPath{
				path: networkingv1.HTTPIngressPath{
					Backend: networkingv1.IngressBackend{
						Service: &networkingv1.IngressServiceBackend{
							Name: "test-service",
							Port: networkingv1.ServiceBackendPort{Number: 80},
						},
					},
				},
			},
			expect: true,
		},
		{
			name:        "Empty backendRefs",
			backendRefs: []gatewayv1.HTTPBackendRef{},
			path: &ingressPath{
				path: networkingv1.HTTPIngressPath{
					Backend: networkingv1.IngressBackend{
						Service: &networkingv1.IngressServiceBackend{
							Name: "test-service",
							Port: networkingv1.ServiceBackendPort{Number: 80},
						},
					},
				},
			},
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchBackendRefs(tt.backendRefs, tt.path)
			if result != tt.expect {
				t.Errorf("Expected %v, got %v for test %s", tt.expect, result, tt.name)
			}
		})
	}
}

func TestIsValidURL(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
	}{
		{
			name:        "Valid HTTP URL",
			input:       "http://example.com",
			expectError: false,
		},
		{
			name:        "Valid HTTPS URL",
			input:       "https://example.com",
			expectError: false,
		},
		{
			name:        "Invalid protocol",
			input:       "ftp://example.com",
			expectError: true,
		},
		{
			name:        "Invalid URL format",
			input:       "htp://example.com",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := isValidURL(tt.input)
			if tt.expectError && err == nil {
				t.Errorf("expected an error but got none for input %s", tt.input)
			}
			if !tt.expectError && err != nil {
				t.Errorf("did not expect an error but got %v for input %s", err, tt.input)
			}
		})
	}
}

func TestIsPathValid(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "Valid path without group capture",
			path:    "/api/v1/resource",
			wantErr: false,
		},
		{
			name:    "Valid path with special characters",
			path:    "/api/v1/resource?query=value&another=value",
			wantErr: false,
		},
		{
			name:    "Path with group capture",
			path:    "/api/v1/(resource)/details",
			wantErr: true,
		},
		{
			name:    "Path with incomplete group capture",
			path:    "/api/v1/resource)/details",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := isPathValid(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("isPathValid() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

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
			if !apiequality.Semantic.DeepEqual(tt.expect, result) {
				t.Errorf("expected %+v, got %+v", tt.expect, result)
			}
		})
	}
}

func TestGroupCaptureUsed(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "No parentheses",
			path:    "/api/v1/resources",
			wantErr: false,
		},
		{
			name:    "Opening parenthesis",
			path:    "/api/(v1)/resources",
			wantErr: true,
		},
		{
			name:    "Closing parenthesis",
			path:    "/api/v1)/resources",
			wantErr: true,
		},
		{
			name:    "Both parentheses",
			path:    "/api/(v1/resources)",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := groupCaptureUsed(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("groupCaptureUsed(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestIsMatchPath(t *testing.T) {
	tests := []struct {
		name   string
		match  gatewayv1.HTTPRouteMatch
		path   *ingressPath
		expect bool
	}{
		{
			name: "Exact match",
			match: gatewayv1.HTTPRouteMatch{
				Path: &gatewayv1.HTTPPathMatch{
					Type:  ptrTo(gatewayv1.PathMatchExact),
					Value: ptr.To("/exact"),
				},
			},
			path: &ingressPath{
				path: networkingv1.HTTPIngressPath{
					Path:     "/exact",
					PathType: ptrTo(networkingv1.PathTypeExact),
				},
			},
			expect: true,
		},
		{
			name: "Path value mismatch",
			match: gatewayv1.HTTPRouteMatch{
				Path: &gatewayv1.HTTPPathMatch{
					Type:  ptrTo(gatewayv1.PathMatchExact),
					Value: ptr.To("/mismatch"),
				},
			},
			path: &ingressPath{
				path: networkingv1.HTTPIngressPath{
					Path:     "/exact",
					PathType: ptrTo(networkingv1.PathTypeExact),
				},
			},
			expect: false,
		},
		{
			name: "Path type mismatch",
			match: gatewayv1.HTTPRouteMatch{
				Path: &gatewayv1.HTTPPathMatch{
					Type:  ptrTo(gatewayv1.PathMatchPathPrefix),
					Value: ptr.To("/exact"),
				},
			},
			path: &ingressPath{
				path: networkingv1.HTTPIngressPath{
					Path:     "/exact",
					PathType: ptrTo(networkingv1.PathTypeExact),
				},
			},
			expect: false,
		},
		{
			name:  "Match path is nil",
			match: gatewayv1.HTTPRouteMatch{},
			path: &ingressPath{
				path: networkingv1.HTTPIngressPath{
					Path:     "/exact",
					PathType: ptrTo(networkingv1.PathTypeExact),
				},
			},
			expect: false,
		},
		{
			name: "Match path value is nil",
			match: gatewayv1.HTTPRouteMatch{
				Path: &gatewayv1.HTTPPathMatch{
					Type: ptrTo(gatewayv1.PathMatchExact),
				},
			},
			path: &ingressPath{
				path: networkingv1.HTTPIngressPath{
					Path:     "/exact",
					PathType: ptrTo(networkingv1.PathTypeExact),
				},
			},
			expect: false,
		},
		{
			name: "Match path type is nil",
			match: gatewayv1.HTTPRouteMatch{
				Path: &gatewayv1.HTTPPathMatch{
					Value: ptr.To("/exact"),
				},
			},
			path: &ingressPath{
				path: networkingv1.HTTPIngressPath{
					Path:     "/exact",
					PathType: ptrTo(networkingv1.PathTypeExact),
				},
			},
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isMatchPath(tt.match, tt.path)
			if result != tt.expect {
				t.Errorf("expected %v, got %v", tt.expect, result)
			}
		})
	}
}
