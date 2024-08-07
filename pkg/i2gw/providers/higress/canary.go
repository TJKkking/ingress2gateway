package higress

import (
	"fmt"
	"strconv"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	CanaryAnnotation    = "canary"
	CanaryByHeader      = "canary-by-header"
	CanaryByHeaderVal   = "canary-by-header-value"
	CanaryByHeaderRegex = "canary-by-header-regex"
	CanaryByCookie      = "canary-by-cookie"
	CanaryWeight        = "canary-weight"
	CanaryWeightTotal   = "canary-weight-total"
)

// 1. 从Ingress中提取出所有的IngressRuleGroup
// 2. 遍历IngressRuleGroup，对每个IngressRuleGroup进行处理，按照host进行分组
// 3. 对每个IngressRuleGroup，提取出所有的path，按照pathType-path进行分组
// 4. 对每个pathType-path数组，根据（header-cookie-weight）优先级进行处理，最终得到一个或多个Rule
func canaryFeature(ingresses []networkingv1.Ingress, gatewayResources *i2gw.GatewayResources) field.ErrorList {
	ruleGroups := common.GetRuleGroups(ingresses)

	for _, rg := range ruleGroups {
		// 按照pathType-path进行分组
		ingressPathsByMatchKey, errs := getPathsByMatchGroups(rg)
		if len(errs) > 0 {
			return errs
		}

		for _, paths := range ingressPathsByMatchKey {
			path := paths[0]
			key := types.NamespacedName{Namespace: path.ingress.Namespace, Name: common.RouteName(rg.Name, rg.Host)}
			httpRoute, ok := gatewayResources.HTTPRoutes[key]
			if !ok {
				fmt.Println("httpRoute not found")
				continue
			}

			applyHTTPRouteWithCanary(&httpRoute, paths)
			gatewayResources.HTTPRoutes[key] = httpRoute
		}
	}

	return nil
}

func applyHTTPRouteWithCanary(httpRoute *gatewayv1.HTTPRoute, paths []ingressPath) field.ErrorList {
	var headerPaths, weightPaths []ingressPath
	var backendRefsByWeight []gatewayv1.HTTPBackendRef
	var numBackends int32
	var errList field.ErrorList

	// split paths by canary config
	for i, path := range paths {
		if path.extra != nil && path.extra.canary != nil && path.extra.canary.configExsits() {
			if path.extra.canary.headerKey != "" {
				headerPaths = append(headerPaths, path)
			} else if path.extra.canary.weight > 0 {
				weightPaths = append(weightPaths, path)
			} else {
				// fmt.Println("canary config not valid")
				continue
			}
		} else {
			backendRef, err := common.ToBackendRef(path.path.Backend, field.NewPath("paths", "backends").Index(i))
			if err != nil {
				errList = append(errList, err)
				continue
			}
			backendRefsByWeight = append(backendRefsByWeight, gatewayv1.HTTPBackendRef{BackendRef: *backendRef})
			numBackends++
		}
	}

	// handle header canary
	for i, path := range headerPaths {
		backendRef, err := common.ToBackendRef(path.path.Backend, field.NewPath("paths", "backends").Index(i))
		if err != nil {
			errList = append(errList, err)
			continue
		}
		applyByCanaryHeader(httpRoute, &path, backendRef)

	}

	// handle weight canary
	var weightTotal = int32(100)
	var totalWeightSet int32
	for i, path := range weightPaths {
		config := path.extra.canary
		backendRef, err := common.ToBackendRef(path.path.Backend, field.NewPath("paths", "backends").Index(i))
		if err != nil {
			errList = append(errList, err)
			continue
		}

		weight := int32(config.weight)
		backendRef.Weight = ptr.To(weight)
		totalWeightSet += weight
		if path.extra.canary.weightTotal > 0 {
			weightTotal = int32(path.extra.canary.weightTotal)
		}
		backendRefsByWeight = append(backendRefsByWeight, gatewayv1.HTTPBackendRef{BackendRef: *backendRef})
	}
	if len(weightPaths) > 0 {
		err := adjustWeights(httpRoute, backendRefsByWeight, weightTotal, totalWeightSet, numBackends)
		if err != nil {
			errList = append(errList, err)
		}
	}

	return errList
}

func adjustWeights(httpRoute *gatewayv1.HTTPRoute, backendRefsByWeight []gatewayv1.HTTPBackendRef, weightTotal, totalWeightSet, numBackends int32) *field.Error {
	if len(backendRefsByWeight) > 0 {
		weightToSet := (weightTotal - totalWeightSet) / numBackends
		if weightToSet < 0 {
			weightToSet = 0
		}
		for i := range backendRefsByWeight {
			if backendRefsByWeight[i].Weight == nil {
				backendRefsByWeight[i].Weight = ptr.To(weightToSet)
			}
			if *backendRefsByWeight[i].Weight > weightTotal {
				backendRefsByWeight[i].Weight = ptr.To(weightTotal)
			}
			// fmt.Println("backendRefsByWeight[i].Weight: ", *backendRefsByWeight[i].Weight)
		}

		patchHTTPRouteWithWeight(httpRoute, backendRefsByWeight)
	}

	return nil
}

func applyByCanaryHeader(httpRoute *gatewayv1.HTTPRoute, path *ingressPath, backendRef *gatewayv1.BackendRef) field.ErrorList {
	var errors field.ErrorList
	var backendRefs []gatewayv1.HTTPBackendRef
	match := gatewayv1.HTTPRouteMatch{
		Path: &gatewayv1.HTTPPathMatch{
			Type:  ConvertPathType(path.path.PathType),
			Value: ptr.To(path.path.Path),
		},
	}

	if !path.extra.canary.cookieMatch && !path.extra.canary.headerRegexMatch {
		matchHeader := gatewayv1.HTTPHeaderMatch{
			Type:  getHeaderMatchTypeExact(),
			Name:  gatewayv1.HTTPHeaderName(path.extra.canary.headerKey),
			Value: path.extra.canary.headerValue,
		}
		match.Headers = append(match.Headers, matchHeader)
	} else if path.extra.canary.cookieMatch {
		// cookie value must be "always"
		// higress使用: "^(.*?;\\s*)?(" + canaryConfig.Cookie + "=always)(;.*)?$", 效果一样但性能更差
		cookieRegex := fmt.Sprintf("(?:^|;\\s*)%s=%s(?:$|;|\\s)", path.extra.canary.headerKey, "always")

		matchHeader := gatewayv1.HTTPHeaderMatch{
			Type:  getHeaderMatchTypeRegex(),
			Name:  gatewayv1.HTTPHeaderName("cookie"),
			Value: cookieRegex,
		}
		match.Headers = append(match.Headers, matchHeader)
	} else {
		matchHeader := gatewayv1.HTTPHeaderMatch{
			Type:  getHeaderMatchTypeRegex(),
			Name:  gatewayv1.HTTPHeaderName(path.extra.canary.headerKey),
			Value: path.extra.canary.headerValue,
		}
		match.Headers = append(match.Headers, matchHeader)
	}

	if rule := singleBackendRuleExists(httpRoute, path); rule != nil {
		rule.Matches = []gatewayv1.HTTPRouteMatch{match}
		return errors
	} else {
		deleteBackendNew(httpRoute, path)
		backendRefs = append(backendRefs, gatewayv1.HTTPBackendRef{BackendRef: *backendRef})
		httpRoute.Spec.Rules = append(httpRoute.Spec.Rules, gatewayv1.HTTPRouteRule{
			Matches:     []gatewayv1.HTTPRouteMatch{match},
			BackendRefs: backendRefs,
		})
	}

	return errors
}

func patchHTTPRouteWithWeight(httpRoute *gatewayv1.HTTPRoute, backendRefs []gatewayv1.HTTPBackendRef) {
	for _, backendRef := range backendRefs {
		ruleExists := false

		for _, rule := range httpRoute.Spec.Rules {
			for i := range rule.BackendRefs {
				if backendRef.Name == rule.BackendRefs[i].Name {
					rule.BackendRefs[i].Weight = backendRef.Weight
					ruleExists = true
					break
				}
			}
		}

		if !ruleExists {
			httpRoute.Spec.Rules = append(httpRoute.Spec.Rules, gatewayv1.HTTPRouteRule{
				BackendRefs: []gatewayv1.HTTPBackendRef{backendRef},
			})
		}
	}
}

type canaryConfig struct {
	enable           bool
	headerKey        string
	headerValue      string
	headerRegexMatch bool
	cookieMatch      bool
	weight           int
	weightTotal      int
}

func (c *canaryConfig) parse(ingress *networkingv1.Ingress) field.ErrorList {
	// 当多种方式同时配置时，灰度方式选择优先级为：基于Header  > 基于Cookie > 基于权重（从高到低）。
	// 所有配置的canary注解的Ingress生效的前提是存在host-pathType-path相同的基线配置（没有canary注解）
	var errs field.ErrorList
	var err error

	fieldPath := field.NewPath("metadata", "annotations")

	if canary := findAnnotationValue(ingress.Annotations, CanaryAnnotation); canary == "true" {
		c.enable = true

		if cHeader := findAnnotationValue(ingress.Annotations, CanaryByHeader); cHeader != "" {
			c.headerKey = cHeader
			c.headerValue = "always"

			if cValue := findAnnotationValue(ingress.Annotations, CanaryByHeaderVal); cValue != "" {
				c.headerValue = cValue

			} else if cRegex := findAnnotationValue(ingress.Annotations, CanaryByHeaderRegex); cRegex != "" {
				c.headerValue = cRegex
				c.headerRegexMatch = true
			}

		} else if cCookie := findAnnotationValue(ingress.Annotations, CanaryByCookie); cCookie != "" {
			c.headerKey = cCookie
			c.cookieMatch = true

		} else if cWeight := findAnnotationValue(ingress.Annotations, CanaryWeight); cWeight != "" {
			c.weight, err = strconv.Atoi(cWeight)
			if err != nil {
				errs = append(errs, field.TypeInvalid(fieldPath, CanaryWeight, err.Error()))
			}

			c.weightTotal = 100
			if cWeightTotal := findAnnotationValue(ingress.Annotations, CanaryWeightTotal); cWeightTotal != "" {
				c.weightTotal, err = strconv.Atoi(cWeightTotal)
				if err != nil {
					errs = append(errs, field.TypeInvalid(fieldPath, CanaryWeightTotal, err.Error()))
				}

			}
		}
	}

	return errs
}

func (c *canaryConfig) configExsits() bool {
	return c.enable
}
