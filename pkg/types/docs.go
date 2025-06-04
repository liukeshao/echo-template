package types

// OpenAPIInfo OpenAPI文档基础信息
type OpenAPIInfo struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Contact     struct {
		Name  string `json:"name,omitempty"`
		Email string `json:"email,omitempty"`
		URL   string `json:"url,omitempty"`
	} `json:"contact,omitempty"`
	License struct {
		Name string `json:"name,omitempty"`
		URL  string `json:"url,omitempty"`
	} `json:"license,omitempty"`
}

// OpenAPIServer 服务器信息
type OpenAPIServer struct {
	URL         string `json:"url"`
	Description string `json:"description"`
}

// OpenAPITag 标签信息
type OpenAPITag struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// OpenAPISpec OpenAPI规范结构
type OpenAPISpec struct {
	OpenAPI string          `json:"openapi"`
	Info    OpenAPIInfo     `json:"info"`
	Servers []OpenAPIServer `json:"servers"`
	Tags    []OpenAPITag    `json:"tags,omitempty"`
	Paths   interface{}     `json:"paths"`
}
