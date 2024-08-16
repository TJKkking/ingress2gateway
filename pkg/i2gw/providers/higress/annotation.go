package higress

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type AnnotationHandler interface {
	Parse(ingress networkingv1.Ingress) field.ErrorList
	configExsits() bool
}

// Define an enum or constants for annotation types
type AnnotationType string

const (
	AnnotationRequestHeaderMod  AnnotationType = "requestHeaderMod"
	AnnotationResponseHeaderMod AnnotationType = "responseHeaderMod"
	AnnotationRedirect          AnnotationType = "redirect"
	AnnotationMirror            AnnotationType = "mirror"
	AnnotationTimeout           AnnotationType = "timeout"
	AnnotationCanary            AnnotationType = "canary"
	AnnotationRewrite           AnnotationType = "rewrite"
)

func getPathsByMatchGroups(rg common.IngressRuleGroup, annotationType AnnotationType) (map[pathMatchKey][]ingressPath, field.ErrorList) {
	ingressPathsByMatchKey := map[pathMatchKey][]ingressPath{}
	var errs field.ErrorList

	for _, ir := range rg.Rules {
		ingress := ir.Ingress
		extraFeatures := &extra{}

		switch annotationType {
		case AnnotationRequestHeaderMod:
			requestHeaderMod := requestHeaderModConfig{}
			if err := requestHeaderMod.Parse(&ingress); err != nil {
				errs = append(errs, field.Invalid(field.NewPath("metadata", "annotations"), ingress.Annotations, err.ToAggregate().Error()))
			}
			if requestHeaderMod.configExsits() {
				extraFeatures.requestHeaderMod = &requestHeaderMod
			}
		case AnnotationResponseHeaderMod:
			responseHeaderMod := responseHeaderModConfig{}
			if err := responseHeaderMod.Parse(&ingress); err != nil {
				errs = append(errs, field.Invalid(field.NewPath("metadata", "annotations"), ingress.Annotations, err.ToAggregate().Error()))
			}
			if responseHeaderMod.configExsits() {
				extraFeatures.responseHeaderMod = &responseHeaderMod
			}
		case AnnotationRedirect:
			redirect := redirectConfig{}
			if err := redirect.Parse(&ingress); err != nil {
				errs = append(errs, field.Invalid(field.NewPath("metadata", "annotations"), ingress.Annotations, err.ToAggregate().Error()))
			}
			if redirect.configExsits() {
				extraFeatures.redirect = &redirect
			}
		case AnnotationMirror:
			mirror := mirrorConfig{}
			if err := mirror.Parse(&ingress); err != nil {
				errs = append(errs, field.Invalid(field.NewPath("metadata", "annotations"), ingress.Annotations, err.ToAggregate().Error()))
			}
			if mirror.configExsits() {
				extraFeatures.mirror = &mirror
			}
		case AnnotationTimeout:
			timeout := timeoutConfig{}
			if err := timeout.Parse(&ingress); err != nil {
				errs = append(errs, field.Invalid(field.NewPath("metadata", "annotations"), ingress.Annotations, err.ToAggregate().Error()))
			}
			if timeout.configExsits() {
				extraFeatures.timeout = &timeout
			}
		case AnnotationCanary:
			canaryAnnotations := canaryConfig{}
			if err := canaryAnnotations.Parse(&ingress); err != nil {
				errs = append(errs, field.Invalid(field.NewPath("metadata", "annotations"), ingress.Annotations, err.ToAggregate().Error()))
			}
			if canaryAnnotations.configExsits() {
				extraFeatures.canary = &canaryAnnotations
			}
		case AnnotationRewrite:
			rewrite := rewriteConfig{}
			if err := rewrite.Parse(&ingress); err != nil {
				errs = append(errs, field.Invalid(field.NewPath("metadata", "annotations"), ingress.Annotations, err.ToAggregate().Error()))
			}
			if rewrite.configExsits() {
				extraFeatures.rewrite = &rewrite
			}
		}

		// After parsing the specific annotation, process the ingress paths
		for _, path := range ir.IngressRule.HTTP.Paths {
			ip := ingressPath{ingress: ingress, ruleType: "http", path: path, extra: extraFeatures}
			pmKey := getPathMatchKey(ip)
			ingressPathsByMatchKey[pmKey] = append(ingressPathsByMatchKey[pmKey], ip)
		}
	}

	return ingressPathsByMatchKey, errs
}

func getPathMatchKey(ip ingressPath) pathMatchKey {
	var pathType string
	if ip.path.PathType != nil {
		pathType = string(*ip.path.PathType)
	}
	// var canaryHeaderKey string
	// if ip.extra != nil && ip.extra.canary != nil && ip.extra.canary.headerKey != "" {
	// 	canaryHeaderKey = ip.extra.canary.headerKey
	// }
	// 同一host下的path以pathType-path作为key聚合
	return pathMatchKey(fmt.Sprintf("%s%s", pathType, ip.path.Path))
}

func findAnnotationValue(annotations map[string]string, key string) string {
	if value, found := annotations[buildNginxAnnotationKey(key)]; found && value != "" {
		return value
	}
	if value, found := annotations[buildHigressAnnotationKey(key)]; found && value != "" {
		return value
	}
	return ""
}

func buildNginxAnnotationKey(key string) string {
	return DefaultAnnotationsPrefix + "/" + key
}

func buildHigressAnnotationKey(key string) string {
	return HigressAnnotationsPrefix + "/" + key
}

var pattern = regexp.MustCompile(`\s+`)

func convertAddOrUpdate(headers string) (map[string]string, field.ErrorList) {
	result := map[string]string{}
	parts := strings.Split(headers, "\n")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		keyValue := pattern.Split(part, 2)
		if len(keyValue) != 2 {
			return nil, field.ErrorList{field.Invalid(field.NewPath("request-header-control-add"), part, "invalid format")}
		}
		key := trimQuotes(strings.TrimSpace(keyValue[0]))
		value := trimQuotes(strings.TrimSpace(keyValue[1]))
		result[key] = value
	}
	return result, nil
}
