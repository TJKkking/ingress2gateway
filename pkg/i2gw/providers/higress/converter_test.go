package higress

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestHigressConverter(t *testing.T) {
	iPrefix := networkingv1.PathTypePrefix
	//iExact := networkingv1.PathTypeExact
	// isPathType := networkingv1.PathTypeImplementationSpecific
	gPathPrefix := gatewayv1.PathMatchPathPrefix
	//gExact := gatewayv1.PathMatchExact

	// Test cases
	testCases := []struct {
		name                     string
		ingresses                OrderedIngressMap
		expectedGatewayResources i2gw.GatewayResources
		expectedErrors           field.ErrorList
	}{
		{
			name: "canary by weight",
			ingresses: OrderedIngressMap{
				ingressNames: []types.NamespacedName{{Namespace: "default", Name: "production"}, {Namespace: "default", Name: "canary"}},
				ingressObjects: map[types.NamespacedName]*networkingv1.Ingress{
					{Namespace: "default", Name: "production"}: {
						ObjectMeta: metav1.ObjectMeta{Name: "production", Namespace: "default"},
						Spec: networkingv1.IngressSpec{
							IngressClassName: common.PtrTo(HigressClass),
							Rules: []networkingv1.IngressRule{{
								Host: "echo.prod.mydomain.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{{
											Path:     "/",
											PathType: &iPrefix,
											Backend: networkingv1.IngressBackend{
												Resource: &corev1.TypedLocalObjectReference{
													Name:     "production",
													Kind:     "StorageBucket",
													APIGroup: common.PtrTo("vendor.example.com"),
												},
											},
										}},
									},
								},
							}},
						},
					},
					{Namespace: "default", Name: "canary"}: {
						ObjectMeta: metav1.ObjectMeta{
							Name:      "canary",
							Namespace: "default",
							Annotations: map[string]string{
								"nginx.ingress.kubernetes.io/canary":        "true",
								"nginx.ingress.kubernetes.io/canary-weight": "20",
							},
						},
						Spec: networkingv1.IngressSpec{
							IngressClassName: common.PtrTo(HigressClass),
							Rules: []networkingv1.IngressRule{{
								Host: "echo.prod.mydomain.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{{
											Path:     "/",
											PathType: &iPrefix,
											Backend: networkingv1.IngressBackend{
												Resource: &corev1.TypedLocalObjectReference{
													Name:     "canary",
													Kind:     "StorageBucket",
													APIGroup: common.PtrTo("vendor.example.com"),
												},
											},
										}},
									},
								},
							}},
						},
					},
				},
			},
			expectedGatewayResources: i2gw.GatewayResources{
				Gateways: map[types.NamespacedName]gatewayv1.Gateway{
					{Namespace: "default", Name: HigressClass}: {
						ObjectMeta: metav1.ObjectMeta{Name: HigressClass, Namespace: "default"},
						Spec: gatewayv1.GatewaySpec{
							GatewayClassName: HigressClass,
							Listeners: []gatewayv1.Listener{{
								Name:     "echo-prod-mydomain-com-http",
								Port:     80,
								Protocol: gatewayv1.HTTPProtocolType,
								Hostname: common.PtrTo(gatewayv1.Hostname("echo.prod.mydomain.com")),
							}},
						},
					},
				},
				HTTPRoutes: map[types.NamespacedName]gatewayv1.HTTPRoute{
					{Namespace: "default", Name: "production-echo-prod-mydomain-com"}: {
						ObjectMeta: metav1.ObjectMeta{Name: "production-echo-prod-mydomain-com", Namespace: "default"},
						Spec: gatewayv1.HTTPRouteSpec{
							CommonRouteSpec: gatewayv1.CommonRouteSpec{
								ParentRefs: []gatewayv1.ParentReference{{
									Name: HigressClass,
								}},
							},
							Hostnames: []gatewayv1.Hostname{"echo.prod.mydomain.com"},
							Rules: []gatewayv1.HTTPRouteRule{{
								Matches: []gatewayv1.HTTPRouteMatch{{
									Path: &gatewayv1.HTTPPathMatch{
										Type:  &gPathPrefix,
										Value: common.PtrTo("/"),
									},
								}},
								BackendRefs: []gatewayv1.HTTPBackendRef{
									{
										BackendRef: gatewayv1.BackendRef{
											BackendObjectReference: gatewayv1.BackendObjectReference{
												Name:  "production",
												Group: common.PtrTo(gatewayv1.Group("vendor.example.com")),
												Kind:  common.PtrTo(gatewayv1.Kind("StorageBucket")),
											},
											Weight: common.PtrTo(int32(80)),
										},
									},
									{
										BackendRef: gatewayv1.BackendRef{
											BackendObjectReference: gatewayv1.BackendObjectReference{
												Name:  "canary",
												Group: common.PtrTo(gatewayv1.Group("vendor.example.com")),
												Kind:  common.PtrTo(gatewayv1.Kind("StorageBucket")),
											},
											Weight: common.PtrTo(int32(20)),
										},
									},
								},
							}},
						},
					},
				},
			},
			expectedErrors: field.ErrorList{},
		},
		{
			name: "rewrite-target",
			ingresses: OrderedIngressMap{
				ingressNames: []types.NamespacedName{{Namespace: "default", Name: "higress-header-a"}},
				ingressObjects: map[types.NamespacedName]*networkingv1.Ingress{
					{Namespace: "default", Name: "higress-header-a"}: {
						ObjectMeta: metav1.ObjectMeta{
							Name:      "higress-header-a",
							Namespace: "default",
							Annotations: map[string]string{
								"nginx.ingress.kubernetes.io/rewrite-target": "/home",
								"nginx.ingress.kubernetes.io/upstream-vhost": "test-app.com",
							},
						},
						Spec: networkingv1.IngressSpec{
							IngressClassName: common.PtrTo("higress"),
							Rules: []networkingv1.IngressRule{{
								Host: "canary-app.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{{
											Path:     "/v1",
											PathType: common.PtrTo(networkingv1.PathTypeExact),
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "canary-test-v1",
													Port: networkingv1.ServiceBackendPort{
														Number: 5000,
													},
												},
											},
										}},
									},
								},
							}},
						},
					},
				},
			},
			expectedGatewayResources: i2gw.GatewayResources{
				Gateways: map[types.NamespacedName]gatewayv1.Gateway{
					{Namespace: "default", Name: "higress"}: {
						ObjectMeta: metav1.ObjectMeta{
							Name:      "higress",
							Namespace: "default",
						},
						Spec: gatewayv1.GatewaySpec{
							GatewayClassName: "higress",
							Listeners: []gatewayv1.Listener{{
								Hostname: common.PtrTo(gatewayv1.Hostname("canary-app.com")),
								Name:     "canary-app-com-http",
								Port:     80,
								Protocol: gatewayv1.HTTPProtocolType,
							}},
						},
					},
				},
				HTTPRoutes: map[types.NamespacedName]gatewayv1.HTTPRoute{
					{Namespace: "default", Name: "higress-header-a-canary-app-com"}: {
						ObjectMeta: metav1.ObjectMeta{
							Name:      "higress-header-a-canary-app-com",
							Namespace: "default",
						},
						Spec: gatewayv1.HTTPRouteSpec{
							CommonRouteSpec: gatewayv1.CommonRouteSpec{
								ParentRefs: []gatewayv1.ParentReference{{
									Name: "higress",
								}},
							},
							Hostnames: []gatewayv1.Hostname{"canary-app.com"},
							Rules: []gatewayv1.HTTPRouteRule{{
								BackendRefs: []gatewayv1.HTTPBackendRef{{
									BackendRef: gatewayv1.BackendRef{
										BackendObjectReference: gatewayv1.BackendObjectReference{
											Name: "canary-test-v1",
											Port: common.PtrTo(gatewayv1.PortNumber(5000)),
										},
									},
								}},
								Filters: []gatewayv1.HTTPRouteFilter{{
									Type: gatewayv1.HTTPRouteFilterURLRewrite,
									URLRewrite: &gatewayv1.HTTPURLRewriteFilter{
										Hostname: common.PtrTo(gatewayv1.PreciseHostname("test-app.com")),
										Path: &gatewayv1.HTTPPathModifier{
											ReplacePrefixMatch: common.PtrTo("/home"),
											Type:               gatewayv1.HTTPPathModifierType("ReplacePrefixMatch"),
										},
									},
								}},
								Matches: []gatewayv1.HTTPRouteMatch{{
									Path: &gatewayv1.HTTPPathMatch{
										Type:  common.PtrTo(gatewayv1.PathMatchExact),
										Value: common.PtrTo("/v1"),
									},
								}},
							}},
						},
					},
				},
			},
			expectedErrors: field.ErrorList{},
		},
		{
			name: "canary by header",
			ingresses: OrderedIngressMap{
				ingressNames: []types.NamespacedName{{Namespace: "default", Name: "higress-canary-test-b"}},
				ingressObjects: map[types.NamespacedName]*networkingv1.Ingress{
					{Namespace: "default", Name: "higress-canary-test-b"}: {
						ObjectMeta: metav1.ObjectMeta{
							Name:      "higress-canary-test-b",
							Namespace: "default",
							Annotations: map[string]string{
								"nginx.ingress.kubernetes.io/canary":                        "true",
								"nginx.ingress.kubernetes.io/canary-by-header":              "X-Canary",
								"nginx.ingress.kubernetes.io/canary-by-header-value":        "always",
								"nginx.ingress.kubernetes.io/request-header-control-add":    "X-Header-Add true",
								"nginx.ingress.kubernetes.io/request-header-control-update": "X-Header-Update true",
								"nginx.ingress.kubernetes.io/request-header-control-remove": "X-Header-Delete",
							},
						},
						Spec: networkingv1.IngressSpec{
							IngressClassName: common.PtrTo("higress"),
							Rules: []networkingv1.IngressRule{{
								Host: "canary-app.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{{
											Path:     "/2",
											PathType: common.PtrTo(networkingv1.PathTypePrefix),
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "canary-test-v2",
													Port: networkingv1.ServiceBackendPort{
														Number: 5000,
													},
												},
											},
										}},
									},
								},
							}},
						},
					},
				},
			},
			expectedGatewayResources: i2gw.GatewayResources{
				Gateways: map[types.NamespacedName]gatewayv1.Gateway{
					{Namespace: "default", Name: "higress"}: {
						ObjectMeta: metav1.ObjectMeta{
							Name:      "higress",
							Namespace: "default",
						},
						Spec: gatewayv1.GatewaySpec{
							GatewayClassName: "higress",
							Listeners: []gatewayv1.Listener{{
								Hostname: common.PtrTo(gatewayv1.Hostname("canary-app.com")),
								Name:     "canary-app-com-http",
								Port:     80,
								Protocol: gatewayv1.HTTPProtocolType,
							}},
						},
					},
				},
				HTTPRoutes: map[types.NamespacedName]gatewayv1.HTTPRoute{
					{Namespace: "default", Name: "higress-canary-test-b-canary-app-com"}: {
						ObjectMeta: metav1.ObjectMeta{
							Name:      "higress-canary-test-b-canary-app-com",
							Namespace: "default",
						},
						Spec: gatewayv1.HTTPRouteSpec{
							CommonRouteSpec: gatewayv1.CommonRouteSpec{
								ParentRefs: []gatewayv1.ParentReference{{
									Name: "higress",
								}},
							},
							Hostnames: []gatewayv1.Hostname{"canary-app.com"},
							Rules: []gatewayv1.HTTPRouteRule{{
								BackendRefs: []gatewayv1.HTTPBackendRef{{
									BackendRef: gatewayv1.BackendRef{
										BackendObjectReference: gatewayv1.BackendObjectReference{
											Name: "canary-test-v2",
											Port: common.PtrTo(gatewayv1.PortNumber(5000)),
										},
									},
								}},
								Filters: []gatewayv1.HTTPRouteFilter{{
									Type: gatewayv1.HTTPRouteFilterRequestHeaderModifier,
									RequestHeaderModifier: &gatewayv1.HTTPHeaderFilter{
										Set:    []gatewayv1.HTTPHeader{{Name: "X-Header-Update", Value: "true"}},
										Add:    []gatewayv1.HTTPHeader{{Name: "X-Header-Add", Value: "true"}},
										Remove: []string{"X-Header-Delete"},
									},
								}},
								Matches: []gatewayv1.HTTPRouteMatch{{
									Path: &gatewayv1.HTTPPathMatch{
										Type:  common.PtrTo(gatewayv1.PathMatchPathPrefix),
										Value: common.PtrTo("/2"),
									},
									Headers: []gatewayv1.HTTPHeaderMatch{{
										Type:  common.PtrTo(gatewayv1.HeaderMatchExact),
										Name:  "X-Canary",
										Value: "always",
									}},
								}},
							}},
						},
					},
				},
			},
			expectedErrors: field.ErrorList{},
		},
		{
			name: "timeout and mirroring",
			ingresses: OrderedIngressMap{
				ingressNames: []types.NamespacedName{
					{Namespace: "default", Name: "higress-header-a"},
					{Namespace: "default", Name: "higress-header-b"},
				},
				ingressObjects: map[types.NamespacedName]*networkingv1.Ingress{
					{Namespace: "default", Name: "higress-header-a"}: {
						ObjectMeta: metav1.ObjectMeta{
							Name:      "higress-header-a",
							Namespace: "default",
							Annotations: map[string]string{
								"higress.io/timeout": "5",
							},
						},
						Spec: networkingv1.IngressSpec{
							IngressClassName: common.PtrTo("higress"),
							Rules: []networkingv1.IngressRule{{
								Host: "canary-app.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{{
											Path:     "/",
											PathType: common.PtrTo(networkingv1.PathTypePrefix),
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "canary-test-v1",
													Port: networkingv1.ServiceBackendPort{
														Number: 5000,
													},
												},
											},
										}},
									},
								},
							}},
						},
					},
					{Namespace: "default", Name: "higress-header-b"}: {
						ObjectMeta: metav1.ObjectMeta{
							Name:      "higress-header-b",
							Namespace: "default",
							Annotations: map[string]string{
								"higress.io/timeout": "10",
								"nginx.ingress.kubernetes.io/mirror-target-service": "default/echo-server:8080",
							},
						},
						Spec: networkingv1.IngressSpec{
							IngressClassName: common.PtrTo("higress"),
							Rules: []networkingv1.IngressRule{{
								Host: "canary-app.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{{
											Path:     "/nm",
											PathType: common.PtrTo(networkingv1.PathTypePrefix),
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "canary-test-v2",
													Port: networkingv1.ServiceBackendPort{
														Number: 5000,
													},
												},
											},
										}},
									},
								},
							}},
						},
					},
				},
			},
			expectedGatewayResources: i2gw.GatewayResources{
				Gateways: map[types.NamespacedName]gatewayv1.Gateway{
					{Namespace: "default", Name: "higress"}: {
						ObjectMeta: metav1.ObjectMeta{
							Name:      "higress",
							Namespace: "default",
						},
						Spec: gatewayv1.GatewaySpec{
							GatewayClassName: "higress",
							Listeners: []gatewayv1.Listener{{
								Hostname: common.PtrTo(gatewayv1.Hostname("canary-app.com")),
								Name:     "canary-app-com-http",
								Port:     80,
								Protocol: gatewayv1.HTTPProtocolType,
							}},
						},
					},
				},
				HTTPRoutes: map[types.NamespacedName]gatewayv1.HTTPRoute{
					{Namespace: "default", Name: "higress-header-a-canary-app-com"}: {
						ObjectMeta: metav1.ObjectMeta{
							Name:      "higress-header-a-canary-app-com",
							Namespace: "default",
						},
						Spec: gatewayv1.HTTPRouteSpec{
							CommonRouteSpec: gatewayv1.CommonRouteSpec{
								ParentRefs: []gatewayv1.ParentReference{{
									Name: "higress",
								}},
							},
							Hostnames: []gatewayv1.Hostname{"canary-app.com"},
							Rules: []gatewayv1.HTTPRouteRule{
								{
									BackendRefs: []gatewayv1.HTTPBackendRef{{
										BackendRef: gatewayv1.BackendRef{
											BackendObjectReference: gatewayv1.BackendObjectReference{
												Name: "canary-test-v1",
												Port: common.PtrTo(gatewayv1.PortNumber(5000)),
											},
										},
									}},
									Matches: []gatewayv1.HTTPRouteMatch{{
										Path: &gatewayv1.HTTPPathMatch{
											Type:  common.PtrTo(gatewayv1.PathMatchPathPrefix),
											Value: common.PtrTo("/"),
										},
									}},
									Timeouts: &gatewayv1.HTTPRouteTimeouts{
										Request: common.PtrTo(gatewayv1.Duration("5s")),
									},
								},
								{
									BackendRefs: []gatewayv1.HTTPBackendRef{{
										BackendRef: gatewayv1.BackendRef{
											BackendObjectReference: gatewayv1.BackendObjectReference{
												Name: "canary-test-v2",
												Port: common.PtrTo(gatewayv1.PortNumber(5000)),
											},
										},
									}},
									Filters: []gatewayv1.HTTPRouteFilter{{
										Type: gatewayv1.HTTPRouteFilterRequestMirror,
										RequestMirror: &gatewayv1.HTTPRequestMirrorFilter{
											BackendRef: gatewayv1.BackendObjectReference{
												Name:      "echo-server",
												Namespace: common.PtrTo(gatewayv1.Namespace("default")),
												Port:      common.PtrTo(gatewayv1.PortNumber(8080)),
											},
										},
									}},
									Matches: []gatewayv1.HTTPRouteMatch{{
										Path: &gatewayv1.HTTPPathMatch{
											Type:  common.PtrTo(gatewayv1.PathMatchPathPrefix),
											Value: common.PtrTo("/nm"),
										},
									}},
									Timeouts: &gatewayv1.HTTPRouteTimeouts{
										Request: common.PtrTo(gatewayv1.Duration("10s")),
									},
								},
							},
						},
					},
				},
			},
			expectedErrors: field.ErrorList{},
		},
		{
			name: "SSL redirection and app-root redirection",
			ingresses: OrderedIngressMap{
				ingressNames: []types.NamespacedName{
					{Namespace: "default", Name: "higress-ssl-root"},
				},
				ingressObjects: map[types.NamespacedName]*networkingv1.Ingress{
					{Namespace: "default", Name: "higress-ssl-root"}: {
						ObjectMeta: metav1.ObjectMeta{
							Name:      "higress-ssl-root",
							Namespace: "default",
							Annotations: map[string]string{
								"nginx.ingress.kubernetes.io/ssl-redirect": "true",
								"nginx.ingress.kubernetes.io/app-root":     "/home",
							},
						},
						Spec: networkingv1.IngressSpec{
							IngressClassName: common.PtrTo("higress"),
							Rules: []networkingv1.IngressRule{{
								Host: "test-app.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path:     "/2",
												PathType: common.PtrTo(networkingv1.PathTypePrefix),
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "svc-test-v2",
														Port: networkingv1.ServiceBackendPort{
															Number: 5000,
														},
													},
												},
											},
											{
												Path:     "/3",
												PathType: common.PtrTo(networkingv1.PathTypeExact),
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "svc-test-v3",
														Port: networkingv1.ServiceBackendPort{
															Number: 5000,
														},
													},
												},
											},
										},
									},
								},
							}},
						},
					},
				},
			},
			expectedGatewayResources: i2gw.GatewayResources{
				Gateways: map[types.NamespacedName]gatewayv1.Gateway{
					{Namespace: "default", Name: "higress"}: {
						ObjectMeta: metav1.ObjectMeta{
							Name:      "higress",
							Namespace: "default",
						},
						Spec: gatewayv1.GatewaySpec{
							GatewayClassName: "higress",
							Listeners: []gatewayv1.Listener{{
								Hostname: common.PtrTo(gatewayv1.Hostname("test-app.com")),
								Name:     "test-app-com-http",
								Port:     80,
								Protocol: gatewayv1.HTTPProtocolType,
							}},
						},
					},
				},
				HTTPRoutes: map[types.NamespacedName]gatewayv1.HTTPRoute{
					{Namespace: "default", Name: "higress-ssl-root-test-app-com"}: {
						ObjectMeta: metav1.ObjectMeta{
							Name:      "higress-ssl-root-test-app-com",
							Namespace: "default",
						},
						Spec: gatewayv1.HTTPRouteSpec{
							CommonRouteSpec: gatewayv1.CommonRouteSpec{
								ParentRefs: []gatewayv1.ParentReference{{
									Name: "higress",
								}},
							},
							Hostnames: []gatewayv1.Hostname{"test-app.com"},
							Rules: []gatewayv1.HTTPRouteRule{
								{
									Filters: []gatewayv1.HTTPRouteFilter{{
										Type: gatewayv1.HTTPRouteFilterRequestRedirect,
										RequestRedirect: &gatewayv1.HTTPRequestRedirectFilter{
											Scheme:     common.PtrTo("https"),
											StatusCode: common.PtrTo(308),
										},
									}},
									Matches: []gatewayv1.HTTPRouteMatch{
										{
											Path: &gatewayv1.HTTPPathMatch{
												Type:  common.PtrTo(gatewayv1.PathMatchPathPrefix),
												Value: common.PtrTo("/2"),
											},
										},
										{
											Path: &gatewayv1.HTTPPathMatch{
												Type:  common.PtrTo(gatewayv1.PathMatchExact),
												Value: common.PtrTo("/3"),
											},
										},
									},
								},
								{
									Filters: []gatewayv1.HTTPRouteFilter{{
										Type: gatewayv1.HTTPRouteFilterRequestRedirect,
										RequestRedirect: &gatewayv1.HTTPRequestRedirectFilter{
											Path: &gatewayv1.HTTPPathModifier{
												ReplaceFullPath: common.PtrTo("/home"),
												Type:            "ReplaceFullPath",
											},
											StatusCode: common.PtrTo(301),
										},
									}},
									Matches: []gatewayv1.HTTPRouteMatch{
										{
											Path: &gatewayv1.HTTPPathMatch{
												Type:  common.PtrTo(gatewayv1.PathMatchExact),
												Value: common.PtrTo("/"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedErrors: field.ErrorList{},
		},
		{
			name: "permanent redirect with custom status code",
			ingresses: OrderedIngressMap{
				ingressNames: []types.NamespacedName{
					{Namespace: "default", Name: "higress-redirect-b"},
				},
				ingressObjects: map[types.NamespacedName]*networkingv1.Ingress{
					{Namespace: "default", Name: "higress-redirect-b"}: {
						ObjectMeta: metav1.ObjectMeta{
							Name:      "higress-redirect-b",
							Namespace: "default",
							Annotations: map[string]string{
								"nginx.ingress.kubernetes.io/permanent-redirect-code": "306",
								"nginx.ingress.kubernetes.io/permanent-redirect":      "http://canary-app.com/export",
							},
						},
						Spec: networkingv1.IngressSpec{
							IngressClassName: common.PtrTo("higress"),
							Rules: []networkingv1.IngressRule{{
								Host: "test-app.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path:     "/test3",
												PathType: common.PtrTo(networkingv1.PathTypeExact),
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "redirect-test-v2",
														Port: networkingv1.ServiceBackendPort{
															Number: 5000,
														},
													},
												},
											},
										},
									},
								},
							}},
						},
					},
				},
			},
			expectedGatewayResources: i2gw.GatewayResources{
				Gateways: map[types.NamespacedName]gatewayv1.Gateway{
					{Namespace: "default", Name: "higress"}: {
						ObjectMeta: metav1.ObjectMeta{
							Name:      "higress",
							Namespace: "default",
						},
						Spec: gatewayv1.GatewaySpec{
							GatewayClassName: "higress",
							Listeners: []gatewayv1.Listener{{
								Hostname: common.PtrTo(gatewayv1.Hostname("test-app.com")),
								Name:     "test-app-com-http",
								Port:     80,
								Protocol: gatewayv1.HTTPProtocolType,
							}},
						},
					},
				},
				HTTPRoutes: map[types.NamespacedName]gatewayv1.HTTPRoute{
					{Namespace: "default", Name: "higress-redirect-b-test-app-com"}: {
						ObjectMeta: metav1.ObjectMeta{
							Name:      "higress-redirect-b-test-app-com",
							Namespace: "default",
						},
						Spec: gatewayv1.HTTPRouteSpec{
							CommonRouteSpec: gatewayv1.CommonRouteSpec{
								ParentRefs: []gatewayv1.ParentReference{{
									Name: "higress",
								}},
							},
							Hostnames: []gatewayv1.Hostname{"test-app.com"},
							Rules: []gatewayv1.HTTPRouteRule{
								{
									Filters: []gatewayv1.HTTPRouteFilter{{
										Type: gatewayv1.HTTPRouteFilterRequestRedirect,
										RequestRedirect: &gatewayv1.HTTPRequestRedirectFilter{
											Hostname: common.PtrTo(gatewayv1.PreciseHostname("canary-app.com")),
											Path: &gatewayv1.HTTPPathModifier{
												ReplaceFullPath: common.PtrTo("/export"),
												Type:            "ReplaceFullPath",
											},
											Scheme:     common.PtrTo("http"),
											StatusCode: common.PtrTo(306),
										},
									}},
									Matches: []gatewayv1.HTTPRouteMatch{{
										Path: &gatewayv1.HTTPPathMatch{
											Type:  common.PtrTo(gatewayv1.PathMatchExact),
											Value: common.PtrTo("/test3"),
										},
									}},
								},
							},
						},
					},
				},
			},
			expectedErrors: field.ErrorList{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			provider := NewProvider(&i2gw.ProviderConf{})

			higressProvider := provider.(*Provider)
			higressProvider.storage.Ingresses = tc.ingresses

			gatewayResources, errs := provider.ToGatewayAPI()

			if len(errs) != len(tc.expectedErrors) {
				t.Errorf("Expected %d errors, got %d: %+v", len(tc.expectedErrors), len(errs), errs)
			} else {
				for i, e := range errs {
					if errors.Is(e, tc.expectedErrors[i]) {
						t.Errorf("Unexpected error message at %d index. Got %s, want: %s", i, e, tc.expectedErrors[i])
					}
				}
			}

			if len(gatewayResources.HTTPRoutes) != len(tc.expectedGatewayResources.HTTPRoutes) {
				t.Errorf("Expected %d HTTPRoutes, got %d: %+v",
					len(tc.expectedGatewayResources.HTTPRoutes), len(gatewayResources.HTTPRoutes), gatewayResources.HTTPRoutes)
			} else {
				for i, got := range gatewayResources.HTTPRoutes {
					key := types.NamespacedName{Namespace: got.Namespace, Name: got.Name}
					want := tc.expectedGatewayResources.HTTPRoutes[key]
					want.SetGroupVersionKind(common.HTTPRouteGVK)
					if !apiequality.Semantic.DeepEqual(got, want) {
						t.Errorf("Expected HTTPRoute %s to be %+v\n Got: %+v\n Diff: %s", i, want, got, cmp.Diff(want, got))
					}
				}
			}

			if len(gatewayResources.Gateways) != len(tc.expectedGatewayResources.Gateways) {
				t.Errorf("Expected %d Gateways, got %d: %+v",
					len(tc.expectedGatewayResources.Gateways), len(gatewayResources.Gateways), gatewayResources.Gateways)
			} else {
				for i, got := range gatewayResources.Gateways {
					key := types.NamespacedName{Namespace: got.Namespace, Name: got.Name}
					want := tc.expectedGatewayResources.Gateways[key]
					want.SetGroupVersionKind(common.GatewayGVK)
					if !apiequality.Semantic.DeepEqual(got, want) {
						t.Errorf("Expected Gateway %s to be %+v\n Got: %+v\n Diff: %s", i, want, got, cmp.Diff(want, got))
					}
				}
			}

		})
	}
}
