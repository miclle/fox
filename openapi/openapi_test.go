package openapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fox-gonic/fox"
	"github.com/fox-gonic/fox/openapi"
)

type getUserRequest struct {
	ID      int64  `uri:"id" binding:"required,gt=0"`
	Verbose bool   `query:"verbose"`
	Token   string `header:"X-Token" binding:"required"`
}

type createUserRequest struct {
	Name  string `json:"name" binding:"required,min=3"`
	Email string `json:"email" binding:"required,email"`
}

type userResponse struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type treeNode struct {
	ID       int64      `json:"id"`
	Parent   *treeNode  `json:"parent"`
	Children []treeNode `json:"children"`
}

func getUser(_ *fox.Context, _ getUserRequest) (userResponse, error) {
	return userResponse{}, nil
}

func createUser(_ *fox.Context, _ createUserRequest) userResponse {
	return userResponse{}
}

func getTree(_ *fox.Context) treeNode {
	return treeNode{}
}

func ping(_ *fox.Context) string {
	return "pong"
}

func TestGenerateDocumentsRoutesParametersBodiesAndResponses(t *testing.T) {
	engine := fox.New()
	engine.GET("/users/:id", getUser)
	engine.POST("/users", createUser)

	g := openapi.New(engine,
		openapi.Info("Fox Test API", "1.0.0"),
		openapi.Server("https://api.example.test"),
	)

	data, err := g.JSON()
	require.NoError(t, err)

	var spec map[string]any
	require.NoError(t, json.Unmarshal(data, &spec))

	require.Equal(t, "3.0.3", spec["openapi"])
	require.Equal(t, "Fox Test API", spec["info"].(map[string]any)["title"])
	require.Equal(t, "https://api.example.test", spec["servers"].([]any)[0].(map[string]any)["url"])

	paths := spec["paths"].(map[string]any)
	getOp := paths["/users/{id}"].(map[string]any)["get"].(map[string]any)
	require.NotEmpty(t, getOp["operationId"])

	parameters := getOp["parameters"].([]any)
	requireParameter(t, parameters, "id", "path", true, "integer")
	requireParameter(t, parameters, "verbose", "query", false, "boolean")
	requireParameter(t, parameters, "X-Token", "header", true, "string")

	responses := getOp["responses"].(map[string]any)
	require.Contains(t, responses, "200")
	require.Contains(t, responses, "default")
	responseSchema := responses["200"].(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)
	require.Equal(t, "#/components/schemas/openapi_test_userResponse", responseSchema["$ref"])

	postOp := paths["/users"].(map[string]any)["post"].(map[string]any)
	bodySchema := postOp["requestBody"].(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)
	require.Contains(t, bodySchema["required"], "name")
	require.Contains(t, bodySchema["required"], "email")

	props := bodySchema["properties"].(map[string]any)
	require.Equal(t, "string", props["name"].(map[string]any)["type"])
	require.Equal(t, float64(3), props["name"].(map[string]any)["minLength"])
	require.Equal(t, "string", props["email"].(map[string]any)["type"])
	require.Equal(t, "email", props["email"].(map[string]any)["format"])

	components := spec["components"].(map[string]any)["schemas"].(map[string]any)
	userSchema := components["openapi_test_userResponse"].(map[string]any)
	require.Equal(t, "object", userSchema["type"])
	require.Contains(t, userSchema["properties"].(map[string]any), "id")
}

func TestHandlersServeGeneratedSpec(t *testing.T) {
	engine := fox.New()
	engine.GET("/users/:id", getUser)

	g := openapi.New(engine, openapi.Info("Fox Test API", "1.0.0"))
	engine.GET("/openapi.json", openapi.JSONHandler(g))
	engine.GET("/openapi.yaml", openapi.YAMLHandler(g))

	jsonRecorder := httptest.NewRecorder()
	engine.ServeHTTP(jsonRecorder, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	require.Equal(t, http.StatusOK, jsonRecorder.Code)
	require.Equal(t, "application/json; charset=utf-8", jsonRecorder.Header().Get("Content-Type"))
	require.Contains(t, jsonRecorder.Body.String(), `"/users/{id}"`)

	yamlRecorder := httptest.NewRecorder()
	engine.ServeHTTP(yamlRecorder, httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil))
	require.Equal(t, http.StatusOK, yamlRecorder.Code)
	require.Equal(t, "application/yaml; charset=utf-8", yamlRecorder.Header().Get("Content-Type"))
	require.Contains(t, yamlRecorder.Body.String(), "/users/{id}:")
}

func TestGenerateSupportsRecursiveStructSchemas(t *testing.T) {
	engine := fox.New()
	engine.GET("/tree", getTree)

	g := openapi.New(engine, openapi.Info("Fox Test API", "1.0.0"))

	data, err := g.JSON()
	require.NoError(t, err)

	var spec map[string]any
	require.NoError(t, json.Unmarshal(data, &spec))

	treeSchema := spec["components"].(map[string]any)["schemas"].(map[string]any)["openapi_test_treeNode"].(map[string]any)
	props := treeSchema["properties"].(map[string]any)
	require.Equal(t, "#/components/schemas/openapi_test_treeNode", props["parent"].(map[string]any)["$ref"])
	require.Equal(t, "#/components/schemas/openapi_test_treeNode", props["children"].(map[string]any)["items"].(map[string]any)["$ref"])
}

func TestGenerateDocumentsStringResponsesAsTextPlain(t *testing.T) {
	engine := fox.New()
	engine.GET("/ping", ping)

	g := openapi.New(engine, openapi.Info("Fox Test API", "1.0.0"))

	data, err := g.JSON()
	require.NoError(t, err)

	var spec map[string]any
	require.NoError(t, json.Unmarshal(data, &spec))

	pingOp := spec["paths"].(map[string]any)["/ping"].(map[string]any)["get"].(map[string]any)
	content := pingOp["responses"].(map[string]any)["200"].(map[string]any)["content"].(map[string]any)
	require.Contains(t, content, "text/plain")
	require.NotContains(t, content, "application/json")
	require.Equal(t, "string", content["text/plain"].(map[string]any)["schema"].(map[string]any)["type"])
}

func requireParameter(t *testing.T, parameters []any, name, in string, required bool, schemaType string) {
	t.Helper()

	for _, raw := range parameters {
		param := raw.(map[string]any)
		if param["name"] == name && param["in"] == in {
			if required {
				require.Equal(t, true, param["required"])
			} else {
				require.NotEqual(t, true, param["required"])
			}
			require.Equal(t, schemaType, param["schema"].(map[string]any)["type"])
			return
		}
	}

	t.Fatalf("parameter %s in %s not found", name, in)
}
