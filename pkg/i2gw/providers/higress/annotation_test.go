package higress

import (
	"reflect"
	"sort"
	"testing"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common" // 提供common.IngressRuleGroup
	networkingv1 "k8s.io/api/networking/v1"                                // 提供网络配置相关的API对象，如Ingress
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	// 提供元数据的API对象，如ObjectMeta
	"k8s.io/apimachinery/pkg/util/validation/field" // 提供字段验证功能
)

func Test_getPathsByMatchGroups(t *testing.T) {
	// Mock data for testing
	mockIngressRuleGroup := common.IngressRuleGroup{
		Rules: []common.Rule{
			{
				Ingress: networkingv1.Ingress{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"higress.io/rewrite-target": "/newpath",
						},
					},
				},
				IngressRule: networkingv1.IngressRule{
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path: "/oldpath",
									PathType: func() *networkingv1.PathType {
										pt := networkingv1.PathTypeExact
										return &pt
									}(),
								},
							},
						},
					},
				},
			},
		},
	}

	expectedPathsByKey := map[pathMatchKey][]ingressPath{
		"Exact/oldpath": {
			{
				ruleType: "http",
				path: networkingv1.HTTPIngressPath{
					Path:     "/oldpath",
					PathType: ptr.To(networkingv1.PathTypeExact),
				},
				extra: &extra{
					rewrite: &rewriteConfig{path: "/newpath"},
				},
			},
		},
	}

	expectedErrs := field.ErrorList{}

	// Test function
	t.Run("Valid annotations", func(t *testing.T) {
		pathsByKey, errs := getPathsByMatchGroups(mockIngressRuleGroup)

		if !isEqualMapStringIngressPaths(pathsByKey, expectedPathsByKey) {
			t.Errorf("Expected pathsByKey to be %v, got %v", expectedPathsByKey, pathsByKey)
		}

		if len(errs) != len(expectedErrs) {
			t.Errorf("Expected no errors, got %d errors", len(errs))
		}
	})
}

func Test_getPathMatchKey(t *testing.T) {
	tests := []struct {
		name     string
		ip       ingressPath
		expected pathMatchKey
	}{
		{
			name: "Exact path type",
			ip: ingressPath{
				path: networkingv1.HTTPIngressPath{
					Path: "/exact",
					PathType: func() *networkingv1.PathType {
						pt := networkingv1.PathTypeExact
						return &pt
					}(),
				},
			},
			expected: pathMatchKey("Exact/exact"),
		},
		{
			name: "Prefix path type",
			ip: ingressPath{
				path: networkingv1.HTTPIngressPath{
					Path: "/prefix",
					PathType: func() *networkingv1.PathType {
						pt := networkingv1.PathTypePrefix
						return &pt
					}(),
				},
			},
			expected: pathMatchKey("Prefix/prefix"),
		},
		{
			name: "ImplementationSpecific path type",
			ip: ingressPath{
				path: networkingv1.HTTPIngressPath{
					Path: "/implspecific",
					PathType: func() *networkingv1.PathType {
						pt := networkingv1.PathTypeImplementationSpecific
						return &pt
					}(),
				},
			},
			expected: pathMatchKey("ImplementationSpecific/implspecific"),
		},
		{
			name: "Empty path type",
			ip: ingressPath{
				path: networkingv1.HTTPIngressPath{
					Path: "/emptypathtype",
				},
			},
			expected: pathMatchKey("/emptypathtype"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getPathMatchKey(tt.ip); got != tt.expected {
				t.Errorf("getPathMatchKey() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func Test_findAnnotationValue(t *testing.T) {
	testCases := []struct {
		name          string
		annotations   map[string]string
		key           string
		expectedValue string
	}{
		{
			name: "Key found with Nginx prefix",
			annotations: map[string]string{
				"higress.io/rewrite-target": "value1",
			},
			key:           "rewrite-target",
			expectedValue: "value1",
		},
		{
			name: "Key found with Higress prefix",
			annotations: map[string]string{
				"higress.io/rewrite-target": "value2",
			},
			key:           "rewrite-target",
			expectedValue: "value2",
		},
		{
			name: "Key not found",
			annotations: map[string]string{
				"nginx.org/another-key": "value",
			},
			key:           "non-existent-key",
			expectedValue: "",
		},
		{
			name:          "Empty annotations map",
			annotations:   map[string]string{},
			key:           "any-key",
			expectedValue: "",
		},
		{
			name: "Key exists with both Nginx and Higress prefixes",
			annotations: map[string]string{
				"nginx.org/rewrite-target":  "nginxValue",
				"higress.io/rewrite-target": "higressValue",
			},
			key:           "rewrite-target",
			expectedValue: "higressValue",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := findAnnotationValue(tc.annotations, tc.key)
			if result != tc.expectedValue {
				t.Errorf("Expected %s, got %s", tc.expectedValue, result)
			}
		})
	}
}

func Test_buildNginxAnnotationKey(t *testing.T) {
	// Define test cases
	testCases := []struct {
		name     string
		key      string
		expected string
	}{
		{
			name:     "simple key",
			key:      "rewrite-target",
			expected: DefaultAnnotationsPrefix + "/rewrite-target",
		},
		{
			name:     "key with spaces",
			key:      "rewrite target",
			expected: DefaultAnnotationsPrefix + "/rewrite target",
		},
		{
			name:     "empty key",
			key:      "",
			expected: DefaultAnnotationsPrefix + "/",
		},
	}

	// Iterate through test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Call the function with the test case input
			result := buildNginxAnnotationKey(tc.key)

			// Check if the result matches the expected value
			if result != tc.expected {
				t.Errorf("For key '%s', expected '%s' but got '%s'", tc.key, tc.expected, result)
			}
		})
	}
}

func Test_buildHigressAnnotationKey(t *testing.T) {
	// Define test cases
	testCases := []struct {
		name     string
		key      string
		expected string
	}{
		{
			name:     "simple key",
			key:      "max-connections",
			expected: HigressAnnotationsPrefix + "/max-connections",
		},
		{
			name:     "key with spaces",
			key:      "max connections",
			expected: HigressAnnotationsPrefix + "/max connections",
		},
		{
			name:     "empty key",
			key:      "",
			expected: HigressAnnotationsPrefix + "/",
		},
		{
			name:     "key with special characters",
			key:      "rate-limit@5r/s",
			expected: HigressAnnotationsPrefix + "/rate-limit@5r/s",
		},
	}
	// Iterate through test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Call the function with the test case input
			result := buildHigressAnnotationKey(tc.key)
			// Check if the result matches the expected value
			if result != tc.expected {
				t.Errorf("For key '%s', expected '%s' but got '%s'", tc.key, tc.expected, result)
			}
		})
	}
}

func isEqualMapStringIngressPaths(a, b map[pathMatchKey][]ingressPath) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || !isEqualIngressPaths(v, bv) {
			return false
		}
	}
	return true
}

func isEqualIngressPaths(a, b []ingressPath) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !isEqualIngressPath(a[i], b[i]) {
			return false
		}
	}
	return true
}

func isEqualIngressPath(a, b ingressPath) bool {
	if a.ruleType != b.ruleType {
		return false
	}
	if !isEqualExtra(a.extra, b.extra) { // Assuming `isEqualExtra` is implemented
		return false
	}
	// Add more comparisons for other fields as necessary
	return true
}

func isEqualExtra(a, b *extra) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	// Check each configuration; assuming each has a corresponding isEqual function
	return isEqualCanaryConfig(a.canary, b.canary) &&
		isEqualHeaderModConfig(a.headerMod, b.headerMod) &&
		isEqualRewriteConfig(a.rewrite, b.rewrite) &&
		isEqualRedirectConfig(a.redirect, b.redirect) &&
		isEqualMirrorConfig(a.mirror, b.mirror) &&
		isEqualTimeoutConfig(a.timeout, b.timeout)
}

func isEqualCanaryConfig(a, b *canaryConfig) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.enable == b.enable &&
		a.headerKey == b.headerKey &&
		a.headerValue == b.headerValue &&
		a.headerRegexMatch == b.headerRegexMatch &&
		a.cookieMatch == b.cookieMatch &&
		a.weight == b.weight &&
		a.weightTotal == b.weightTotal
}

func isEqualHeaderModConfig(a, b *headerModConfig) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return isEqualMapStringString(a.add, b.add) &&
		isEqualMapStringString(a.update, b.update) &&
		isEqualSliceString(a.remove, b.remove)
}

func isEqualMapStringString(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

func isEqualSliceString(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	sort.Strings(a)
	sort.Strings(b)
	return reflect.DeepEqual(a, b)
}

func isEqualRewriteConfig(a, b *rewriteConfig) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.hostname == b.hostname &&
		a.path == b.path
}

func isEqualMirrorConfig(a, b *mirrorConfig) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.namespace == b.namespace &&
		a.targetService == b.targetService &&
		a.port == b.port
}

func isEqualTimeoutConfig(a, b *timeoutConfig) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.timeout == b.timeout
}

func isEqualRedirectConfig(a, b *redirectConfig) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.sslRedirect == b.sslRedirect &&
		a.redirectURL == b.redirectURL &&
		a.redirectCode == b.redirectCode &&
		a.rootRedirect == b.rootRedirect
}
