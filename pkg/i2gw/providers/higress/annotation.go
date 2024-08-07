package higress

import (
	"fmt"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type AnnotationHandler interface {
	Parse(ingress networkingv1.Ingress) field.ErrorList
}

// TODO: use interface to handle different annotations
// 1. Define all feature's annotation struct
// 2. Edit Parse method for each struct
// 3. Implement getPathsByMatchGroupsNew
func getPathsByMatchGroupsNew(rg common.IngressRuleGroup) (map[pathMatchKey][]ingressPath, field.ErrorList) {
	// TODO: implement this function
	return nil, nil
}

func getPathsByMatchGroups(rg common.IngressRuleGroup) (map[pathMatchKey][]ingressPath, field.ErrorList) {
	ingressPathsByMatchKey := map[pathMatchKey][]ingressPath{}
	var errs field.ErrorList

	for _, ir := range rg.Rules {
		ingress := ir.Ingress
		extraFeatures := &extra{}

		// parse header control annotations
		headerMod := headerModConfig{}
		if err := headerMod.Parse(&ingress); err != nil {
			errs = append(errs, field.Invalid(field.NewPath("metadata", "annotations"), ingress.Annotations, err.ToAggregate().Error()))
		}
		extraFeatures.headerMod = &headerMod

		// parse redirect annotations
		redirect := redirectConfig{}
		if err := redirect.Parse(&ingress); err != nil {
			errs = append(errs, field.Invalid(field.NewPath("metadata", "annotations"), ingress.Annotations, err.ToAggregate().Error()))
		}
		extraFeatures.redirect = &redirect

		// parse mirror annotations
		mirror := mirrorConfig{}
		if err := mirror.Parse(&ingress); err != nil {
			errs = append(errs, field.Invalid(field.NewPath("metadata", "annotations"), ingress.Annotations, err.ToAggregate().Error()))
		}
		extraFeatures.mirror = &mirror

		// parse timeout annotations
		timeout := timeoutConfig{}
		if err := timeout.Parse(&ingress); err != nil {
			errs = append(errs, field.Invalid(field.NewPath("metadata", "annotations"), ingress.Annotations, err.ToAggregate().Error()))
		}
		extraFeatures.timeout = &timeout

		if !redirect.redirectExsits() {

			// parse canary annotations
			canaryAnnotations := canaryConfig{}
			if err := canaryAnnotations.parse(&ingress); err != nil {
				errs = append(errs, field.Invalid(field.NewPath("metadata", "annotations"), ingress.Annotations, err.ToAggregate().Error()))
			}
			extraFeatures.canary = &canaryAnnotations

			// parse rewrite annotations
			rewrite := rewriteConfig{}
			if err := rewrite.Parse(&ingress); err != nil {
				errs = append(errs, field.Invalid(field.NewPath("metadata", "annotations"), ingress.Annotations, err.ToAggregate().Error()))
			}
			extraFeatures.rewrite = &rewrite

		}

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
	return pathMatchKey(fmt.Sprintf("%s/%s", pathType, ip.path.Path))
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
