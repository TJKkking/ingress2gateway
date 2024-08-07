package higress

import (
	"net/url"
	"strconv"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	SSLRedirect           = "ssl-redirect"
	ForceSSLRedirect      = "force-ssl-redirect"
	PermanentRedirect     = "permanent-redirect"
	PermanentRedirectCode = "permanent-redirect-code"
	TemporalRedirect      = "temporal-redirect"
	AppRoot               = "app-root"
)

const (
	DefaultPermanentCode   = 301
	DefaultTemporalCode    = 302
	DefaultSSLRedirectCode = 308
	RootPath               = "/"
)

func applyHTTPRouteWithRedirect(httpRoute *gatewayv1.HTTPRoute, paths []ingressPath) field.ErrorList {
	var errors field.ErrorList
	var redirectPaths []ingressPath

	for _, path := range paths {
		if path.extra != nil && path.extra.redirect != nil && path.extra.redirect.configExsits() {
			redirectPaths = append(redirectPaths, path)
		}
	}

	for _, path := range redirectPaths {
		var redirectRules []gatewayv1.HTTPRouteRule
		redirect := path.extra.redirect

		// http redirect与ssl redirect互斥，只能存在一个
		if redirect.redirectURL != "" {
			errors = append(errors, applyByRedirect(httpRoute, &path, &redirectRules)...)
		} else if redirect.sslRedirect {
			errors = append(errors, applyBySSLRedirect(httpRoute, &path, &redirectRules)...)
		}

		if redirect.rootRedirect != "" {
			errors = append(errors, applyByRootRedirect(httpRoute, &path, &redirectRules)...)
		}

		httpRoute.Spec.Rules = append(httpRoute.Spec.Rules, redirectRules...)
	}

	return errors
}

func applyByRedirect(httpRoute *gatewayv1.HTTPRoute, path *ingressPath, redirectRules *[]gatewayv1.HTTPRouteRule) field.ErrorList {
	var errors field.ErrorList
	var redirectFilter gatewayv1.HTTPRequestRedirectFilter

	redirect := path.extra.redirect
	parseURL, err := url.Parse(redirect.redirectURL)
	if err != nil {
		errors = append(errors, field.Invalid(field.NewPath("metadata", "annotations"), path.ingress.Annotations, err.Error()))
	}
	redirectFilter.Scheme = &parseURL.Scheme
	hostname := gatewayv1.PreciseHostname(parseURL.Hostname())
	redirectFilter.Hostname = &hostname
	portNumber, err := strconv.Atoi(parseURL.Port())
	if err != nil {
		errors = append(errors, field.Invalid(field.NewPath("metadata", "annotations"), path.ingress.Annotations, err.Error()))
	}
	// Implementations SHOULD NOT add the port number in the 'Location' header in the following cases:
	// The port number is the default port number for the scheme (http:80, https:443).
	if !((parseURL.Scheme == "http" && (portNumber == 80 || portNumber == 0)) || (parseURL.Scheme == "https" && (portNumber == 443 || portNumber == 0))) {
		port := gatewayv1.PortNumber(portNumber)
		redirectFilter.Port = &port
	}

	redirectFilter.Path = &gatewayv1.HTTPPathModifier{
		Type:            gatewayv1.FullPathHTTPPathModifier,
		ReplaceFullPath: &parseURL.Path,
	}
	redirectFilter.StatusCode = &redirect.redirectCode

	match := gatewayv1.HTTPRouteMatch{
		Path: &gatewayv1.HTTPPathMatch{
			Type:  ConvertPathType(path.path.PathType),
			Value: ptr.To(path.path.Path),
		},
	}
	// redirect rule exists
	if rule := redirectPathRuleExists(httpRoute, path); rule != nil {
		rule.Filters = append(rule.Filters, gatewayv1.HTTPRouteFilter{
			Type:            gatewayv1.HTTPRouteFilterRequestRedirect,
			RequestRedirect: &redirectFilter,
		})
	} else if rule, err := sameRedirectRuleExists(httpRoute, redirect); err != nil {
		errors = append(errors, err...)
	} else if rule != nil {
		rule.Matches = append(rule.Matches, match)
	} else {
		*redirectRules = append(*redirectRules, *createHTTPRouteRule(createHTTPRouteRuleParam{
			matchs: []gatewayv1.HTTPRouteMatch{match},
			filters: []gatewayv1.HTTPRouteFilter{
				{
					Type:            gatewayv1.HTTPRouteFilterRequestRedirect,
					RequestRedirect: &redirectFilter,
				},
			},
		}))
	}
	err = deleteBackendNew(httpRoute, path)
	if err != nil {
		errors = append(errors, field.Invalid(field.NewPath("metadata", "annotations"), path.ingress.Annotations, err.Error()))
	}

	return errors
}

func applyBySSLRedirect(httpRoute *gatewayv1.HTTPRoute, path *ingressPath, redirectRules *[]gatewayv1.HTTPRouteRule) field.ErrorList {
	var errors field.ErrorList

	match := gatewayv1.HTTPPathMatch{
		Type:  ConvertPathType(path.path.PathType),
		Value: ptr.To(path.path.Path),
	}
	redirectFilter := gatewayv1.HTTPRequestRedirectFilter{
		Scheme: ptr.To("https"),
		// 308 is the default status code for ssl redirect
		StatusCode: ptr.To(DefaultSSLRedirectCode),
	}

	// ssl redirect filter at the beginning of rules and filters
	if rule := redirectPathRuleExists(httpRoute, path); rule != nil {
		rule.Filters = append([]gatewayv1.HTTPRouteFilter{{
			Type:            gatewayv1.HTTPRouteFilterRequestRedirect,
			RequestRedirect: &redirectFilter,
		}}, rule.Filters...)
	} else if rule := sslRedirectRuleExists(httpRoute); rule != nil {
		rule.Matches = append(rule.Matches, gatewayv1.HTTPRouteMatch{Path: &match})
	} else {
		*redirectRules = append([]gatewayv1.HTTPRouteRule{
			*createHTTPRouteRule(createHTTPRouteRuleParam{
				matchs: []gatewayv1.HTTPRouteMatch{{Path: &match}},
				filters: []gatewayv1.HTTPRouteFilter{
					{
						Type:            gatewayv1.HTTPRouteFilterRequestRedirect,
						RequestRedirect: &redirectFilter,
					},
				},
			}),
		}, *redirectRules...)
	}
	err := deleteBackendNew(httpRoute, path)
	if err != nil {
		errors = append(errors, field.Invalid(field.NewPath("metadata", "annotations"), path.ingress.Annotations, err.Error()))
	}

	return errors
}

func applyByRootRedirect(httpRoute *gatewayv1.HTTPRoute, path *ingressPath, redirectRules *[]gatewayv1.HTTPRouteRule) field.ErrorList {
	if rootRedirectRuleExists(httpRoute) {
		return nil
	}

	// one host can only have one root redirect, if exists, skip
	var errors field.ErrorList
	var redirectFilter gatewayv1.HTTPRequestRedirectFilter

	redirect := path.extra.redirect
	targetURL := redirect.rootRedirect
	redirectFilter.Path = &gatewayv1.HTTPPathModifier{
		Type:            gatewayv1.FullPathHTTPPathModifier,
		ReplaceFullPath: &targetURL,
	}
	statusCode := DefaultPermanentCode
	redirectFilter.StatusCode = &statusCode

	pathMatchTypeExact := gatewayv1.PathMatchExact
	match := gatewayv1.HTTPRouteMatch{
		Path: &gatewayv1.HTTPPathMatch{
			Type:  &pathMatchTypeExact,
			Value: ptr.To("/"),
		},
	}

	*redirectRules = append(*redirectRules, *createHTTPRouteRule(createHTTPRouteRuleParam{
		matchs: []gatewayv1.HTTPRouteMatch{match},
		filters: []gatewayv1.HTTPRouteFilter{
			{
				Type:            gatewayv1.HTTPRouteFilterRequestRedirect,
				RequestRedirect: &redirectFilter,
			},
		},
	}))

	return errors
}

// check if the httpRoute contains root redirect
func rootRedirectRuleExists(httpRoute *gatewayv1.HTTPRoute) bool {
	for _, rule := range httpRoute.Spec.Rules {
		for _, match := range rule.Matches {
			if match.Path != nil && *match.Path.Type == gatewayv1.PathMatchExact && match.Path.Value != nil && *match.Path.Value == RootPath {
				for _, filter := range rule.Filters {
					if filter.RequestRedirect != nil && filter.RequestRedirect.Path != nil && filter.RequestRedirect.Path.ReplaceFullPath != nil {
						return true
					}
				}
			}

		}
	}
	return false
}

func sslRedirectRuleExists(httpRoute *gatewayv1.HTTPRoute) *gatewayv1.HTTPRouteRule {
	for i, rule := range httpRoute.Spec.Rules {
		// confirm the rule only has a sslRedirect filter
		if len(rule.Filters) == 1 {
			if filter := rule.Filters[0]; filter.RequestRedirect != nil && filter.RequestRedirect.Scheme != nil && *filter.RequestRedirect.Scheme == "https" {
				return &httpRoute.Spec.Rules[i]
			}
		}
	}
	return nil
}

func redirectPathRuleExists(httpRoute *gatewayv1.HTTPRoute, path *ingressPath) *gatewayv1.HTTPRouteRule {
	if ru := singleBackendRuleExists(httpRoute, path); ru != nil {
		for _, filter := range ru.Filters {
			if filter.RequestRedirect == nil {
				return ru
			}
		}
	}

	return nil
}

func sameRedirectRuleExists(httpRoute *gatewayv1.HTTPRoute, redirectConf *redirectConfig) (*gatewayv1.HTTPRouteRule, field.ErrorList) {
	var errors field.ErrorList
	parseURL, err := url.Parse(redirectConf.redirectURL)
	if err != nil {
		errors = append(errors, field.Invalid(field.NewPath("metadata", "annotations"), redirectConf.redirectURL, err.Error()))
		return nil, errors
	}
	for i, rule := range httpRoute.Spec.Rules {
		for _, filter := range rule.Filters {
			if filter.RequestRedirect != nil && filterPathEqual(filter.RequestRedirect, parseURL) {
				return &httpRoute.Spec.Rules[i], nil
			}
		}
	}
	return nil, nil
}

func filterPathEqual(re *gatewayv1.HTTPRequestRedirectFilter, parseURL *url.URL) bool {
	return re.Hostname != nil && *re.Hostname == gatewayv1.PreciseHostname(parseURL.Hostname()) &&
		re.Path != nil && re.Path.ReplaceFullPath != nil && *re.Path.ReplaceFullPath == parseURL.Path
}

type redirectConfig struct {
	sslRedirect  bool
	redirectURL  string
	redirectCode int
	rootRedirect string
}

func (r *redirectConfig) Parse(ingress *networkingv1.Ingress) field.ErrorList {
	var errs field.ErrorList
	var err error

	if sslRedirect := findAnnotationValue(ingress.Annotations, SSLRedirect); sslRedirect != "" {
		r.sslRedirect = true
	}

	if forceSSLRedirect := findAnnotationValue(ingress.Annotations, ForceSSLRedirect); forceSSLRedirect != "" {
		r.sslRedirect = true
	}

	if permanentRedirect := findAnnotationValue(ingress.Annotations, PermanentRedirect); permanentRedirect != "" && isValidURL(permanentRedirect) == nil {
		r.redirectURL = permanentRedirect
		r.redirectCode = DefaultPermanentCode
	}

	if prc := findAnnotationValue(ingress.Annotations, PermanentRedirectCode); prc != "" {
		r.redirectCode, err = strconv.Atoi(prc)
		if err != nil {
			errs = append(errs, field.TypeInvalid(field.NewPath("metadata", "annotations"), PermanentRedirectCode, err.Error()))
		}
	}

	if temporalRedirect := findAnnotationValue(ingress.Annotations, TemporalRedirect); temporalRedirect != "" && isValidURL(temporalRedirect) == nil {
		r.redirectURL = temporalRedirect
		r.redirectCode = DefaultTemporalCode
	}

	// fmt.Println("appRoot: ", ingress.Annotations[buildNginxAnnotationKey(AppRoot)])
	if appRoot := findAnnotationValue(ingress.Annotations, AppRoot); appRoot != "" {
		r.rootRedirect = appRoot
	}

	return errs
}

func (r *redirectConfig) configExsits() bool {
	return r.sslRedirect || r.redirectURL != "" || r.rootRedirect != ""
}

func (r *redirectConfig) redirectExsits() bool {
	return r.redirectURL != "" || r.sslRedirect
}
