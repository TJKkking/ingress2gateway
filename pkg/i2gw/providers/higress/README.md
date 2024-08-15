# Alibaba Higress Provider

The project supports translating Higress-specific annotations to Gateway API resources. Also compatible with some annotations of Nginx-ingress provider.

## Currently Supported Annotations

### Canary Deployment Annotations

- **`nginx.ingress.kubernetes.io/canary`**: 
  - **Description**: Enables canary deployments by weighting backends when set to `true`.
  - **Behavior**: Creates a canary route with appropriate weights assigned to the backend services.

- **`nginx.ingress.kubernetes.io/canary-by-header`**: 
  - **Description**: Specifies the header name used for HTTP header-based routing. The header's key is provided by this annotation, and the default value is "always" unless overridden by **canary-by-header-value**.
  - **Behavior**: Adds a `HTTPHeaderMatch` to the corresponding HTTPRoute, matching requests with the specified header.

- **`nginx.ingress.kubernetes.io/canary-by-header-value`**: 
  - **Description**: Specifies the exact value of the header to match.
  - **Behavior**: Sets the `HTTPHeaderMatch.value` to the specified value.

- **`nginx.ingress.kubernetes.io/canary-by-header-pattern`**: 
  - **Description**: Defines a regular expression pattern for header matching.
  - **Behavior**: Creates a `HeaderMatchRegularExpression` based on the specified pattern.

- **`nginx.ingress.kubernetes.io/canary-weight`**: 
  - **Description**: Assigns weight to backends in a canary deployment.
  - **Behavior**: Applies the specified weight to the backend services in the translated HTTPRoute.

- **`nginx.ingress.kubernetes.io/canary-weight-total`**: 
  - **Description**: Defines the total weight to distribute among all backends, with a default value of 100.
  - **Behavior**: Calculates the weight distribution across backends based on this value.

### Request Header Control Annotations

- **`higress.io/request-header-control-add`**: 
  - **Description**: Adds headers to requests passing through the route.
  - **Behavior**: Adds a `RequestHeaderModifier` filter to the corresponding `HTTPRouteRule`. The syntax supports:
    - **Single Header**: `Key Value`.
    - **Multiple Headers**: Use YAML's special symbol `|`, with each `Key Value` pair on a separate line.

- **`higress.io/request-header-control-update`**: 
  - **Description**: Updates existing headers in requests.
  - **Behavior**: Modifies the corresponding headers according to the specified values in the `HTTPRouteRule`. The syntax is the same as "request-header-control-add" and supports multiple values.

- **`higress.io/request-header-control-remove`**: 
  - **Description**: Removes specified headers from requests before forwarding them to the backend service.
  - **Behavior**: Adds a `RequestHeaderModifier` filter to remove the specified headers in the `HTTPRouteRule`. The syntax supports:
    - **Single Header**: `Key`.
    - **Multiple Headers**: Comma-separated list of keys.

### Response Header Control Annotations

The syntax for "response" is the same as for "request".

- **`higress.io/response-header-control-add`**: 
  - **Description**: Adds headers to responses.
  - **Behavior**: Adds a `ResponseHeaderModifier` filter to the `HTTPRouteRule` to include the specified headers in responses.

- **`higress.io/response-header-control-update`**: 
  - **Description**: Updates headers in responses.
  - **Behavior**: Modifies the specified response headers in the `HTTPRouteRule`.

- **`higress.io/response-header-control-remove`**: 
  - **Description**: Removes specified headers from responses.
  - **Behavior**: Adds a `ResponseHeaderModifier` filter to remove the specified headers in the `HTTPRouteRule`.

### Rewrite Target Annotation

- **`higress.io/rewrite-target`**: 
  - **Description**: Specifies the target URI to rewrite requests to.
  - **Behavior**: Applies a `URLRewrite` filter to the HTTPRoute, with the path set to the value of this annotation. **Capture groups syntax are NOT supported now.**

### Upstream Vhost Annotation

- **`higress.io/upstream-vhost`**: 
  - **Description**: Defines the upstream virtual host.
  - **Behavior**: Sets the `Host` header of the request to the specified upstream virtual host using the `URLRewrite` filter.

### Redirect Annotations

- **`higress.io/permanent-redirect`**: 
  - **Description**: Specifies a permanent (HTTP 301) redirect target.
  - **Behavior**: Creates a `RequestRedirect` filter with a 301 status code to perform the permanent redirect.

- **`higress.io/temporal-redirect`**: 
  - **Description**: Specifies a temporary (HTTP 302) redirect target.
  - **Behavior**: Creates a `RequestRedirect` filter with a 302 status code to perform the temporary redirect.

### Mirror Traffic Annotation

- **`higress.io/mirror-target-service`**: 
  - **Description**: Specifies the target service to mirror traffic to.
  - **Behavior**: Adds a `RequestMirror` filter to the `HTTPRouteRule`, duplicating traffic to the specified service.

### Timeout Annotation

- **`higress.io/timeout`**: 
  - **Description**: Specifies the timeout duration for requests. 
  - **Behavior**: Configures the corresponding `HTTPRouteRule` with the specified timeout settings.
