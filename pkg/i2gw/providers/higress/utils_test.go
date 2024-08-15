package higress

import (
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestToBackendRef(t *testing.T) {
	tests := []struct {
		name           string
		ingressBackend networkingv1.IngressBackend
		expectedError  bool
		expectedRef    *gatewayv1.BackendRef
	}{
		{
			name: "Valid service with numeric port",
			ingressBackend: networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{
					Name: "test-service",
					Port: networkingv1.ServiceBackendPort{Number: 80},
				},
			},
			expectedError: false,
			expectedRef: &gatewayv1.BackendRef{
				BackendObjectReference: gatewayv1.BackendObjectReference{
					Name: gatewayv1.ObjectName("test-service"),
					Port: ptrTo(gatewayv1.PortNumber(80)),
				},
			},
		},
		{
			name: "Valid resource",
			ingressBackend: networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{
					Name: "test-resource",
					Port: networkingv1.ServiceBackendPort{Number: 80},
				},
			},
			expectedError: false,
			expectedRef: &gatewayv1.BackendRef{
				BackendObjectReference: gatewayv1.BackendObjectReference{
					Name: gatewayv1.ObjectName("test-resource"),
					Port: ptrTo(gatewayv1.PortNumber(80)),
				},
			},
		},
		{
			name: "Service with named port",
			ingressBackend: networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{
					Name: "test-service",
					Port: networkingv1.ServiceBackendPort{Name: "http"},
				},
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := ToBackendRef(tt.ingressBackend, field.NewPath("spec"))
			if tt.expectedError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if ref == nil {
					t.Errorf("expected non-nil BackendRef")
				} else if !apiequality.Semantic.DeepEqual(ref, tt.expectedRef) {
					t.Errorf("expected %+v, got %+v", *tt.expectedRef, *ref)
				}
			}
		})
	}
}

func TestTrimQuotes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Double quotes",
			input:    `"quoted"`,
			expected: "quoted",
		},
		{
			name:     "Single quotes",
			input:    `'quoted'`,
			expected: "quoted",
		},
		{
			name:     "Mixed quotes",
			input:    `"quoted'`,
			expected: `"quoted'`,
		},
		{
			name:     "No quotes",
			input:    "no quotes",
			expected: "no quotes",
		},
		{
			name:     "Quotes only at the beginning",
			input:    `"unmatched`,
			expected: `"unmatched`,
		},
		{
			name:     "Quotes only at the end",
			input:    `unmatched"`,
			expected: `unmatched"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := trimQuotes(tt.input)
			if result != tt.expected {
				t.Errorf("trimQuotes(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSplitBySeparator(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		separator string
		expected  []string
	}{
		{
			name:      "Comma separated values",
			content:   "apple, banana, cherry",
			separator: ",",
			expected:  []string{"apple", "banana", "cherry"},
		},
		{
			name:      "Space separated values",
			content:   "apple banana cherry",
			separator: " ",
			expected:  []string{"apple", "banana", "cherry"},
		},
		{
			name:      "No separator present",
			content:   "applebanana",
			separator: ",",
			expected:  []string{"applebanana"},
		},
		{
			name:      "Empty content",
			content:   "",
			separator: ",",
			expected:  []string{},
		},
		{
			name:      "Separator only",
			content:   ",,,",
			separator: ",",
			expected:  []string{},
		},
		{
			name:      "Multiple separators",
			content:   "apple,,banana,,,cherry",
			separator: ",",
			expected:  []string{"apple", "banana", "cherry"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitBySeparator(tt.content, tt.separator)
			if !apiequality.Semantic.DeepEqual(result, tt.expected) {
				t.Errorf("splitBySeparator(%q, %q) = %v, want %v", tt.content, tt.separator, result, tt.expected)
			}
		})
	}
}
