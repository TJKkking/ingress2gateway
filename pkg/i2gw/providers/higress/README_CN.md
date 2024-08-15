# 阿里巴巴 Higress Provider

该项目支持将 Higress 部分注解转换为 Gateway API 资源。同时兼容 Nginx-ingress 部分注解。

## 当前支持的注解

### 金丝雀部署注解

- **`nginx.ingress.kubernetes.io/canary`**: 
  - **描述**: 当设置为 `true` 时，启用金丝雀部署的后端权重分配。
  - **行为**: 创建一个金丝雀路由，并为后端服务分配适当的权重。

- **`nginx.ingress.kubernetes.io/canary-by-header`**: 
  - **描述**: 指定用于 HTTP 头部路由的头部名称。此注解指定头部的键，默认值为 "always"，除非被 **canary-by-header-value** 覆盖。
  - **行为**: 在相应的 HTTPRoute 中添加一个 `HTTPHeaderMatch`，以匹配具有指定头部的请求。

- **`nginx.ingress.kubernetes.io/canary-by-header-value`**: 
  - **描述**: 指定要匹配的头部的确切值。
  - **行为**: 将 `HTTPHeaderMatch.value` 设置为指定的值。

- **`nginx.ingress.kubernetes.io/canary-by-header-pattern`**: 
  - **描述**: 定义用于头部匹配的正则表达式模式。
  - **行为**: 基于指定的模式创建一个 `HeaderMatchRegularExpression`。

- **`nginx.ingress.kubernetes.io/canary-weight`**: 
  - **描述**: 为金丝雀部署中的后端分配权重。
  - **行为**: 在转换后的 HTTPRoute 中应用指定的权重到后端服务。

- **`nginx.ingress.kubernetes.io/canary-weight-total`**: 
  - **描述**: 定义分配给所有后端的总权重，默认值为 100。
  - **行为**: 根据该值计算后端的权重分配。

### 请求头控制注解

- **`higress.io/request-header-control-add`**: 
  - **描述**: 向通过该路由的请求添加头部。
  - **行为**: 在相应的 `HTTPRouteRule` 中添加一个 `RequestHeaderModifier` 过滤器。语法支持：
    - **单个头部**: `Key Value`。
    - **多个头部**: 使用 YAML 的特殊符号 `|`，每个 `Key Value` 对应一行。

- **`higress.io/request-header-control-update`**: 
  - **描述**: 更新请求中的现有头部。
  - **行为**: 根据 `HTTPRouteRule` 中指定的值修改相应的头部。语法与 "request-header-control-add" 相同，并支持多个值。

- **`higress.io/request-header-control-remove`**: 
  - **描述**: 在请求转发到后端服务之前删除指定的头部。
  - **行为**: 添加一个 `RequestHeaderModifier` 过滤器，以删除 `HTTPRouteRule` 中指定的头部。语法支持：
    - **单个头部**: `Key`。
    - **多个头部**: 逗号分隔的键列表。

### 响应头控制注解

"response" 的语法与 "request" 相同。

- **`higress.io/response-header-control-add`**: 
  - **描述**: 向响应添加头部。
  - **行为**: 在 `HTTPRouteRule` 中添加一个 `ResponseHeaderModifier` 过滤器，以在响应中包含指定的头部。

- **`higress.io/response-header-control-update`**: 
  - **描述**: 更新响应中的头部。
  - **行为**: 在 `HTTPRouteRule` 中修改指定的响应头部。

- **`higress.io/response-header-control-remove`**: 
  - **描述**: 从响应中删除指定的头部。
  - **行为**: 添加一个 `ResponseHeaderModifier` 过滤器，以从 `HTTPRouteRule` 中删除指定的头部。

### 重写目标注解

- **`higress.io/rewrite-target`**: 
  - **描述**: 指定重写请求的目标 URI。
  - **行为**: 在 HTTPRoute 上应用 `URLRewrite` 过滤器，路径设置为此注解的值。**当前不支持捕获组语法。**

### 上游vHost注解

- **`higress.io/upstream-vhost`**: 
  - **描述**: 定义上游虚拟主机。
  - **行为**: 使用 `URLRewrite` 过滤器将请求的 `Host` 头部设置为指定的上游虚拟主机。

### 重定向注解

- **`higress.io/permanent-redirect`**: 
  - **描述**: 指定永久 (HTTP 301) 重定向目标。
  - **行为**: 创建一个带有 301 状态码的 `RequestRedirect` 过滤器以执行永久重定向。

- **`higress.io/temporal-redirect`**: 
  - **描述**: 指定临时 (HTTP 302) 重定向目标。
  - **行为**: 创建一个带有 302 状态码的 `RequestRedirect` 过滤器以执行临时重定向。

### 镜像流量注解

- **`higress.io/mirror-target-service`**: 
  - **描述**: 指定镜像流量的目标服务。
  - **行为**: 在 `HTTPRouteRule` 中添加一个 `RequestMirror` 过滤器，将流量复制到指定的服务。

### 超时注解

- **`higress.io/timeout`**: 
  - **描述**: 指定请求的超时时间。
  - **行为**: 根据指定的超时设置配置相应的 `HTTPRouteRule`。