package openapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"

	"github.com/fox-gonic/fox"
	"github.com/fox-gonic/fox-openapi"
	v1users "github.com/fox-gonic/fox-openapi/internal/collisionfixture/v1/users"
	v2users "github.com/fox-gonic/fox-openapi/internal/collisionfixture/v2/users"
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

type loginRequest struct {
	Username string `form:"username" binding:"required"`
	Password string `form:"password" binding:"required,min=8"`
}

type mismatchedURIRequest struct {
	UserID int64 `uri:"user_id" binding:"required"`
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

type documentedCreateUserRequest struct {
	// Display name for the new user.
	Name string `json:"name" binding:"required"`
}

type documentedUserResponse struct {
	// Stable user identifier.
	ID int64 `json:"id"`
}

type lineCommentResponse struct {
	ID int64 `json:"id"` // Stable user identifier from a line comment.
}

type combinedCommentResponse struct {
	// Stable user identifier.
	ID int64 `json:"id"` // Exposed in public APIs.
}

type customID string

type customIDResponse struct {
	ID customID `json:"id"`
}

type customErrorResponse struct {
	Message string `json:"message"`
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

func getItem(_ *fox.Context) string {
	return "item"
}

func noop(_ *fox.Context) {}

func login(_ *fox.Context, _ loginRequest) string {
	return "ok"
}

func getMismatchedURI(_ *fox.Context, _ mismatchedURIRequest) string {
	return "ok"
}

// Create documented user.
//
// Creates a user and returns the persisted representation.
func createDocumentedUser(_ *fox.Context, _ documentedCreateUserRequest) documentedUserResponse {
	return documentedUserResponse{}
}

func getCustomID(_ *fox.Context) customIDResponse {
	return customIDResponse{}
}

func getLineComment(_ *fox.Context) lineCommentResponse {
	return lineCommentResponse{}
}

func getCombinedComment(_ *fox.Context) combinedCommentResponse {
	return combinedCommentResponse{}
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
	require.Equal(t, "#/components/schemas/fox_openapi_test_userResponse", responseSchema["$ref"])

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
	userSchema := components["fox_openapi_test_userResponse"].(map[string]any)
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

func TestGenerateIsLazyAndPicksUpRoutesAfterNew(t *testing.T) {
	engine := fox.New()

	g := openapi.New(engine, openapi.Info("Fox Test API", "1.0.0"))
	engine.GET("/users/:id", getUser)
	engine.POST("/users", createUser)

	data, err := g.JSON()
	require.NoError(t, err)

	var spec map[string]any
	require.NoError(t, json.Unmarshal(data, &spec))

	paths := spec["paths"].(map[string]any)
	require.Contains(t, paths, "/users/{id}")
	require.Contains(t, paths, "/users")
}

func TestMountExcludesSpecEndpointsFromGeneratedPaths(t *testing.T) {
	engine := fox.New()
	engine.GET("/users/:id", getUser)

	g := openapi.New(engine, openapi.Info("Fox Test API", "1.0.0"))
	openapi.Mount(engine, g)

	jsonRecorder := httptest.NewRecorder()
	engine.ServeHTTP(jsonRecorder, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	require.Equal(t, http.StatusOK, jsonRecorder.Code)

	var spec map[string]any
	require.NoError(t, json.Unmarshal(jsonRecorder.Body.Bytes(), &spec))
	paths := spec["paths"].(map[string]any)
	require.Contains(t, paths, "/users/{id}")
	require.NotContains(t, paths, "/openapi.json")
	require.NotContains(t, paths, "/openapi.yaml")
}

func TestRegenerateRefreshesPathsAfterNewRoutes(t *testing.T) {
	engine := fox.New()
	engine.GET("/users/:id", getUser)

	g := openapi.New(engine, openapi.Info("Fox Test API", "1.0.0"))
	_, err := g.JSON()
	require.NoError(t, err)

	engine.POST("/users", createUser)
	g.Regenerate()

	data, err := g.JSON()
	require.NoError(t, err)

	var spec map[string]any
	require.NoError(t, json.Unmarshal(data, &spec))
	paths := spec["paths"].(map[string]any)
	require.Contains(t, paths, "/users/{id}")
	require.Contains(t, paths, "/users")
}

func TestMountRegistersGeneratedSpecHandlers(t *testing.T) {
	engine := fox.New()
	engine.GET("/users/:id", getUser)

	g := openapi.New(engine, openapi.Info("Fox Test API", "1.0.0"))
	openapi.Mount(engine, g)

	jsonRecorder := httptest.NewRecorder()
	engine.ServeHTTP(jsonRecorder, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	require.Equal(t, http.StatusOK, jsonRecorder.Code)
	require.Contains(t, jsonRecorder.Body.String(), `"/users/{id}"`)

	yamlRecorder := httptest.NewRecorder()
	engine.ServeHTTP(yamlRecorder, httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil))
	require.Equal(t, http.StatusOK, yamlRecorder.Code)
	require.Contains(t, yamlRecorder.Body.String(), "/users/{id}:")
}

func TestMountCanUseCustomPaths(t *testing.T) {
	engine := fox.New()
	engine.GET("/users/:id", getUser)

	g := openapi.New(engine, openapi.Info("Fox Test API", "1.0.0"))
	openapi.Mount(engine, g,
		openapi.MountYAML("/docs/spec.yaml"),
		openapi.MountJSON("/docs/spec.json"),
	)

	jsonRecorder := httptest.NewRecorder()
	engine.ServeHTTP(jsonRecorder, httptest.NewRequest(http.MethodGet, "/docs/spec.json", nil))
	require.Equal(t, http.StatusOK, jsonRecorder.Code)

	yamlRecorder := httptest.NewRecorder()
	engine.ServeHTTP(yamlRecorder, httptest.NewRequest(http.MethodGet, "/docs/spec.yaml", nil))
	require.Equal(t, http.StatusOK, yamlRecorder.Code)
}

func TestGenerateSupportsRecursiveStructSchemas(t *testing.T) {
	engine := fox.New()
	engine.GET("/tree", getTree)

	g := openapi.New(engine, openapi.Info("Fox Test API", "1.0.0"))

	data, err := g.JSON()
	require.NoError(t, err)

	var spec map[string]any
	require.NoError(t, json.Unmarshal(data, &spec))

	treeSchema := spec["components"].(map[string]any)["schemas"].(map[string]any)["fox_openapi_test_treeNode"].(map[string]any)
	props := treeSchema["properties"].(map[string]any)
	require.Equal(t, "#/components/schemas/fox_openapi_test_treeNode", props["parent"].(map[string]any)["$ref"])
	require.Equal(t, "#/components/schemas/fox_openapi_test_treeNode", props["children"].(map[string]any)["items"].(map[string]any)["$ref"])
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

func TestGenerateDocumentsFormRequestBodies(t *testing.T) {
	engine := fox.New()
	engine.POST("/login", login)

	g := openapi.New(engine, openapi.Info("Fox Test API", "1.0.0"))

	data, err := g.JSON()
	require.NoError(t, err)

	var spec map[string]any
	require.NoError(t, json.Unmarshal(data, &spec))

	loginOp := spec["paths"].(map[string]any)["/login"].(map[string]any)["post"].(map[string]any)
	content := loginOp["requestBody"].(map[string]any)["content"].(map[string]any)
	require.Contains(t, content, "application/x-www-form-urlencoded")
	require.NotContains(t, content, "application/json")

	schema := content["application/x-www-form-urlencoded"].(map[string]any)["schema"].(map[string]any)
	require.Contains(t, schema["required"], "username")
	require.Contains(t, schema["required"], "password")
	props := schema["properties"].(map[string]any)
	require.Equal(t, "string", props["username"].(map[string]any)["type"])
	require.Equal(t, float64(8), props["password"].(map[string]any)["minLength"])
}

func TestGenerateWarnsWhenURIParamDoesNotMatchPath(t *testing.T) {
	engine := fox.New()
	engine.GET("/users/:id", getMismatchedURI)

	g := openapi.New(engine, openapi.Info("Fox Test API", "1.0.0"))

	require.Contains(t, g.Warnings(), `GET /users/:id: uri parameter "user_id" does not match path parameters [id]`)
}

func TestGenerateDocumentsRoutePathParamsWithoutInputStruct(t *testing.T) {
	engine := fox.New()
	engine.GET("/items/:id", getItem)

	g := openapi.New(engine, openapi.Info("Fox Test API", "1.0.0"))

	data, err := g.JSON()
	require.NoError(t, err)

	var spec map[string]any
	require.NoError(t, json.Unmarshal(data, &spec))

	itemOp := spec["paths"].(map[string]any)["/items/{id}"].(map[string]any)["get"].(map[string]any)
	requireParameter(t, itemOp["parameters"].([]any), "id", "path", true, "string")
}

func TestGenerateDocumentsNoReturnHandlersWithEmptyOKResponse(t *testing.T) {
	engine := fox.New()
	engine.POST("/noop", noop)

	g := openapi.New(engine, openapi.Info("Fox Test API", "1.0.0"))

	data, err := g.JSON()
	require.NoError(t, err)

	var spec map[string]any
	require.NoError(t, json.Unmarshal(data, &spec))

	noopOp := spec["paths"].(map[string]any)["/noop"].(map[string]any)["post"].(map[string]any)
	response := noopOp["responses"].(map[string]any)["200"].(map[string]any)
	require.Equal(t, "OK", response["description"])
	require.NotContains(t, response, "content")
}

func TestGenerateReadsHandlerAndFieldCommentsFromSource(t *testing.T) {
	engine := fox.New()
	engine.POST("/documented-users", createDocumentedUser)

	g := openapi.New(engine,
		openapi.Info("Fox Test API", "1.0.0"),
		openapi.Source([]string{"./..."}, openapi.IncludeTestFiles()),
	)

	data, err := g.JSON()
	require.NoError(t, err)

	var spec map[string]any
	require.NoError(t, json.Unmarshal(data, &spec))

	op := spec["paths"].(map[string]any)["/documented-users"].(map[string]any)["post"].(map[string]any)
	require.Equal(t, "Create documented user.", op["summary"])
	require.Equal(t, "Create documented user.\n\nCreates a user and returns the persisted representation.", op["description"])

	requestSchema := op["requestBody"].(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)
	requestProps := requestSchema["properties"].(map[string]any)
	require.Equal(t, "Display name for the new user.", requestProps["name"].(map[string]any)["description"])

	responseSchemaName := "fox_openapi_test_documentedUserResponse"
	responseProps := spec["components"].(map[string]any)["schemas"].(map[string]any)[responseSchemaName].(map[string]any)["properties"].(map[string]any)
	require.Equal(t, "Stable user identifier.", responseProps["id"].(map[string]any)["description"])
}

func TestGenerateReadsFieldLineCommentsFromSource(t *testing.T) {
	engine := fox.New()
	engine.GET("/line-comment", getLineComment)

	g := openapi.New(engine,
		openapi.Info("Fox Test API", "1.0.0"),
		openapi.Source([]string{"./..."}, openapi.IncludeTestFiles()),
	)

	data, err := g.JSON()
	require.NoError(t, err)

	var spec map[string]any
	require.NoError(t, json.Unmarshal(data, &spec))

	schemaName := "fox_openapi_test_lineCommentResponse"
	props := spec["components"].(map[string]any)["schemas"].(map[string]any)[schemaName].(map[string]any)["properties"].(map[string]any)
	require.Equal(t, "Stable user identifier from a line comment.", props["id"].(map[string]any)["description"])
}

func TestGenerateCombinesFieldDocAndLineCommentsFromSource(t *testing.T) {
	engine := fox.New()
	engine.GET("/combined-comment", getCombinedComment)

	g := openapi.New(engine,
		openapi.Info("Fox Test API", "1.0.0"),
		openapi.Source([]string{"./..."}, openapi.IncludeTestFiles()),
	)

	data, err := g.JSON()
	require.NoError(t, err)

	var spec map[string]any
	require.NoError(t, json.Unmarshal(data, &spec))

	schemaName := "fox_openapi_test_combinedCommentResponse"
	props := spec["components"].(map[string]any)["schemas"].(map[string]any)[schemaName].(map[string]any)["properties"].(map[string]any)
	require.Equal(t, "Stable user identifier.\n\nExposed in public APIs.", props["id"].(map[string]any)["description"])
}

func TestGenerateAppliesExplicitOperationMetadata(t *testing.T) {
	engine := fox.New()
	engine.POST("/documented-users", createDocumentedUser)

	g := openapi.New(engine,
		openapi.Info("Fox Test API", "1.0.0"),
		openapi.Source([]string{"./..."}, openapi.IncludeTestFiles()),
		openapi.Operation("POST", "/documented-users",
			openapi.Summary("Create user override"),
			openapi.Description("Explicit operation description."),
			openapi.OperationID("createUser"),
			openapi.Tags("users", "admin"),
			openapi.Response(201, documentedUserResponse{}, "Created"),
		),
	)

	data, err := g.JSON()
	require.NoError(t, err)

	var spec map[string]any
	require.NoError(t, json.Unmarshal(data, &spec))

	op := spec["paths"].(map[string]any)["/documented-users"].(map[string]any)["post"].(map[string]any)
	require.Equal(t, "Create user override", op["summary"])
	require.Equal(t, "Explicit operation description.", op["description"])
	require.Equal(t, "createUser", op["operationId"])
	require.Equal(t, []any{"users", "admin"}, op["tags"])

	responses := op["responses"].(map[string]any)
	require.Contains(t, responses, "200")
	created := responses["201"].(map[string]any)
	require.Equal(t, "Created", created["description"])
	schema := created["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)
	require.Equal(t, "#/components/schemas/fox_openapi_test_documentedUserResponse", schema["$ref"])
}

func TestGenerateAppliesSecuritySchemes(t *testing.T) {
	engine := fox.New()
	engine.GET("/users/:id", getUser)

	g := openapi.New(engine,
		openapi.Info("Fox Test API", "1.0.0"),
		openapi.SecurityScheme("BearerAuth", openapi.HTTPBearerSecurity("JWT bearer token")),
		openapi.Operation("GET", "/users/:id",
			openapi.Security("BearerAuth"),
		),
	)

	data, err := g.JSON()
	require.NoError(t, err)

	var spec map[string]any
	require.NoError(t, json.Unmarshal(data, &spec))

	schemes := spec["components"].(map[string]any)["securitySchemes"].(map[string]any)
	bearer := schemes["BearerAuth"].(map[string]any)
	require.Equal(t, "http", bearer["type"])
	require.Equal(t, "bearer", bearer["scheme"])
	require.Equal(t, "JWT", bearer["bearerFormat"])
	require.Equal(t, "JWT bearer token", bearer["description"])

	op := spec["paths"].(map[string]any)["/users/{id}"].(map[string]any)["get"].(map[string]any)
	require.Equal(t, []any{map[string]any{"BearerAuth": []any{}}}, op["security"])
}

func TestGenerateAppliesGroupMetadataByPathPrefix(t *testing.T) {
	engine := fox.New()
	api := engine.Group("/api")
	api.GET("/users/:id", getUser)
	api.POST("/users", createUser)

	g := openapi.New(engine,
		openapi.Info("Fox Test API", "1.0.0"),
		openapi.SecurityScheme("BearerAuth", openapi.HTTPBearerSecurity("JWT bearer token")),
		openapi.Group("/api",
			openapi.Tags("api"),
			openapi.Security("BearerAuth"),
		),
		openapi.Operation("POST", "/api/users",
			openapi.Tags("users"),
		),
	)

	data, err := g.JSON()
	require.NoError(t, err)

	var spec map[string]any
	require.NoError(t, json.Unmarshal(data, &spec))

	getOp := spec["paths"].(map[string]any)["/api/users/{id}"].(map[string]any)["get"].(map[string]any)
	require.Equal(t, []any{"api"}, getOp["tags"])
	require.Equal(t, []any{map[string]any{"BearerAuth": []any{}}}, getOp["security"])

	postOp := spec["paths"].(map[string]any)["/api/users"].(map[string]any)["post"].(map[string]any)
	require.Equal(t, []any{"users"}, postOp["tags"])
	require.Equal(t, []any{map[string]any{"BearerAuth": []any{}}}, postOp["security"])
}

func TestGenerateUsesRegisteredFormatters(t *testing.T) {
	engine := fox.New()
	engine.GET("/custom-id", getCustomID)

	g := openapi.New(engine,
		openapi.Info("Fox Test API", "1.0.0"),
		openapi.RegisterFormatter(reflect.TypeOf(customID("")), openapi3.NewStringSchema().WithFormat("uuid")),
	)

	data, err := g.JSON()
	require.NoError(t, err)

	var spec map[string]any
	require.NoError(t, json.Unmarshal(data, &spec))

	schemaName := "fox_openapi_test_customIDResponse"
	props := spec["components"].(map[string]any)["schemas"].(map[string]any)[schemaName].(map[string]any)["properties"].(map[string]any)
	id := props["id"].(map[string]any)
	require.Equal(t, "string", id["type"])
	require.Equal(t, "uuid", id["format"])
}

func TestSchemaNameCollisionFallsBackToFullPath(t *testing.T) {
	engine := fox.New()
	engine.GET("/v1/users", func(_ *fox.Context) v1users.User { return v1users.User{} })
	engine.GET("/v2/users", func(_ *fox.Context) v2users.User { return v2users.User{} })

	g := openapi.New(engine, openapi.Info("Fox Test API", "1.0.0"))

	data, err := g.JSON()
	require.NoError(t, err)

	var spec map[string]any
	require.NoError(t, json.Unmarshal(data, &spec))

	v1Op := spec["paths"].(map[string]any)["/v1/users"].(map[string]any)["get"].(map[string]any)
	v2Op := spec["paths"].(map[string]any)["/v2/users"].(map[string]any)["get"].(map[string]any)
	v1Schema := v1Op["responses"].(map[string]any)["200"].(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)["$ref"].(string)
	v2Schema := v2Op["responses"].(map[string]any)["200"].(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)["$ref"].(string)

	require.NotEqual(t, v1Schema, v2Schema, "distinct types must produce distinct schema refs")

	schemas := spec["components"].(map[string]any)["schemas"].(map[string]any)
	v1Ref := strings.TrimPrefix(v1Schema, "#/components/schemas/")
	v2Ref := strings.TrimPrefix(v2Schema, "#/components/schemas/")
	require.Contains(t, schemas, v1Ref)
	require.Contains(t, schemas, v2Ref)

	// HandlerRoutes is sorted by path so v1 is registered first and keeps
	// the short name; v2 falls back to the full-path form.
	require.Equal(t, "users_User", v1Ref)
	require.Contains(t, v2Ref, "v2")
	require.NotEmpty(t, g.Warnings(), "collision should produce a warning")
}

func TestGenerateUsesCustomErrorSchema(t *testing.T) {
	engine := fox.New()
	engine.GET("/users/:id", getUser)

	g := openapi.New(engine,
		openapi.Info("Fox Test API", "1.0.0"),
		openapi.SetErrorSchema(customErrorResponse{}),
	)

	data, err := g.JSON()
	require.NoError(t, err)

	var spec map[string]any
	require.NoError(t, json.Unmarshal(data, &spec))

	response := spec["components"].(map[string]any)["responses"].(map[string]any)["HTTPError"].(map[string]any)
	schema := response["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)
	require.Equal(t, "#/components/schemas/fox_openapi_test_customErrorResponse", schema["$ref"])

	components := spec["components"].(map[string]any)["schemas"].(map[string]any)
	require.Contains(t, components, "fox_openapi_test_customErrorResponse")
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
