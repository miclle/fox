package openapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/goccy/go-yaml"

	"github.com/fox-gonic/fox"
)

// Option configures a Generator.
type Option func(*Generator)

// Generator builds and serializes an OpenAPI specification for a Fox engine.
type Generator struct {
	engine      *fox.Engine
	spec        *openapi3.T
	schemaNames map[reflect.Type]string
	warnings    []string
	docs        *commentDocs
	operations  map[operationKey]operationDoc
}

// Info sets the OpenAPI info title and version.
func Info(title, version string) Option {
	return func(g *Generator) {
		g.spec.Info.Title = title
		g.spec.Info.Version = version
	}
}

// Server appends a server URL to the generated OpenAPI spec.
func Server(url string) Option {
	return func(g *Generator) {
		g.spec.AddServer(&openapi3.Server{URL: url})
	}
}

// New creates a Generator and immediately scans the engine's current routes.
func New(engine *fox.Engine, opts ...Option) *Generator {
	components := openapi3.NewComponents()
	components.Schemas = openapi3.Schemas{}

	g := &Generator{
		engine:      engine,
		schemaNames: make(map[reflect.Type]string),
		operations:  make(map[operationKey]operationDoc),
		spec: &openapi3.T{
			OpenAPI:    "3.0.3",
			Info:       &openapi3.Info{Title: "Fox API", Version: "0.0.0"},
			Paths:      openapi3.NewPaths(),
			Components: &components,
		},
	}

	for _, opt := range opts {
		opt(g)
	}

	g.addHTTPErrorSchema()
	g.generate()
	return g
}

// Spec returns the generated OpenAPI model.
func (g *Generator) Spec() *openapi3.T {
	return g.spec
}

// Warnings returns non-fatal generation warnings.
func (g *Generator) Warnings() []string {
	return append([]string(nil), g.warnings...)
}

func (g *Generator) warnf(format string, args ...any) {
	g.warnings = append(g.warnings, fmt.Sprintf(format, args...))
}

// JSON serializes the generated spec as formatted JSON.
func (g *Generator) JSON() ([]byte, error) {
	return json.MarshalIndent(g.spec, "", "  ")
}

// YAML serializes the generated spec as YAML.
func (g *Generator) YAML() ([]byte, error) {
	return yaml.Marshal(g.spec)
}

// WriteYAML writes the generated YAML spec to w.
func (g *Generator) WriteYAML(w io.Writer) error {
	data, err := g.YAML()
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func (g *Generator) generate() {
	for _, route := range g.engine.HandlerRoutes() {
		op := openapi3.NewOperation()
		op.OperationID = operationID(route)
		op.Responses = openapi3.NewResponses()
		if g.docs != nil {
			if text := g.docs.funcDoc(route.HandlerName); text != "" {
				op.Summary = firstParagraph(text)
				op.Description = text
			}
		}

		if route.HandlerType.NumIn() == 2 {
			g.addInput(op, route, route.HandlerType.In(1))
		}
		g.addMissingPathParams(op, route.Path)
		g.addResponses(op, route.HandlerType)
		g.applyOperationDoc(op, route.Method, route.Path)

		g.spec.AddOperation(openAPIPath(route.Path), route.Method, op)
	}
}

func (g *Generator) addMissingPathParams(op *openapi3.Operation, path string) {
	for name := range pathParamNames(path) {
		if hasParameter(op, name, "path") {
			continue
		}
		op.AddParameter(&openapi3.Parameter{
			Name:     name,
			In:       "path",
			Required: true,
			Schema:   &openapi3.SchemaRef{Value: openapi3.NewStringSchema()},
		})
	}
}

func hasParameter(op *openapi3.Operation, name, in string) bool {
	for _, ref := range op.Parameters {
		if ref == nil || ref.Value == nil {
			continue
		}
		if ref.Value.Name == name && ref.Value.In == in {
			return true
		}
	}
	return false
}

func (g *Generator) addInput(op *openapi3.Operation, route fox.RouteInfo, typ reflect.Type) {
	typ = deref(typ)
	if typ.Kind() != reflect.Struct {
		return
	}

	body := openapi3.NewObjectSchema()
	body.Properties = openapi3.Schemas{}
	bodyMediaType := "application/json"
	pathParams := pathParamNames(route.Path)

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue
		}

		if name := tagName(field, "uri"); name != "" {
			if _, ok := pathParams[name]; !ok {
				g.warnf(`%s %s: uri parameter %q does not match path parameters %s`, route.Method, route.Path, name, formatParamNames(pathParams))
			}
			op.AddParameter(g.parameter(name, "path", true, typ, field))
			continue
		}
		if name := tagName(field, "query"); name != "" {
			op.AddParameter(g.parameter(name, "query", hasBinding(field, "required"), typ, field))
			continue
		}
		if name := tagName(field, "header"); name != "" {
			op.AddParameter(g.parameter(name, "header", hasBinding(field, "required"), typ, field))
			continue
		}
		if tagName(field, "context") != "" {
			continue
		}

		name := tagName(field, "form")
		if name != "" {
			bodyMediaType = "application/x-www-form-urlencoded"
		}
		if name == "" {
			name = tagName(field, "json")
		}
		if name == "" {
			name = lowerFirst(field.Name)
		}
		body.Properties[name] = g.fieldSchemaRefForType(typ, field)
		if hasBinding(field, "required") {
			body.Required = append(body.Required, name)
		}
	}

	if len(body.Properties) > 0 {
		op.RequestBody = &openapi3.RequestBodyRef{Value: openapi3.NewRequestBody().
			WithRequired(len(body.Required) > 0).
			WithSchema(body, []string{bodyMediaType})}
	}
}

func (g *Generator) parameter(name, in string, required bool, owner reflect.Type, field reflect.StructField) *openapi3.Parameter {
	return &openapi3.Parameter{
		Name:     name,
		In:       in,
		Required: required,
		Schema:   g.fieldSchemaRefForType(owner, field),
	}
}

func (g *Generator) fieldSchemaRef(field reflect.StructField) *openapi3.SchemaRef {
	return g.fieldSchemaRefForType(nil, field)
}

func (g *Generator) fieldSchemaRefForType(owner reflect.Type, field reflect.StructField) *openapi3.SchemaRef {
	ref := g.schemaRef(field.Type)
	if ref.Value != nil {
		applyBinding(ref.Value, field)
		if owner != nil {
			if text := g.docs.fieldDoc(deref(owner).Name(), field.Name); text != "" {
				ref.Value.Description = text
			}
		}
	}
	return ref
}

func (g *Generator) addResponses(op *openapi3.Operation, typ reflect.Type) {
	if typ.NumOut() == 0 {
		op.Responses.Set("200", &openapi3.ResponseRef{Value: openapi3.NewResponse().
			WithDescription(http.StatusText(http.StatusOK))})
		return
	}

	firstOut := typ.Out(0)
	if !isError(firstOut) {
		op.Responses.Set("200", &openapi3.ResponseRef{Value: g.successResponse(firstOut)})
	}

	if typ.NumOut() == 2 || isError(firstOut) {
		op.Responses.Set("default", &openapi3.ResponseRef{Ref: "#/components/responses/HTTPError"})
	}
}

func (g *Generator) successResponse(typ reflect.Type) *openapi3.Response {
	response := openapi3.NewResponse().WithDescription(http.StatusText(http.StatusOK))
	if deref(typ).Kind() == reflect.String {
		return response.WithContent(openapi3.Content{
			"text/plain": openapi3.NewMediaType().WithSchemaRef(g.schemaRef(typ)),
		})
	}
	return response.WithJSONSchemaRef(g.schemaRef(typ))
}

func (g *Generator) schemaRef(typ reflect.Type) *openapi3.SchemaRef {
	typ = deref(typ)
	if typ.Kind() == reflect.Struct && typ != reflect.TypeOf(time.Time{}) {
		return g.componentSchemaRef(typ)
	}
	schema := g.schema(typ)
	return &openapi3.SchemaRef{Value: schema}
}

func (g *Generator) componentSchemaRef(typ reflect.Type) *openapi3.SchemaRef {
	if name, ok := g.schemaNames[typ]; ok {
		return &openapi3.SchemaRef{Ref: "#/components/schemas/" + name}
	}

	name := schemaName(typ)
	g.schemaNames[typ] = name

	if _, exists := g.spec.Components.Schemas[name]; !exists {
		g.spec.Components.Schemas[name] = &openapi3.SchemaRef{Value: openapi3.NewObjectSchema()}
		g.spec.Components.Schemas[name] = &openapi3.SchemaRef{Value: g.objectSchema(typ)}
	}

	return &openapi3.SchemaRef{Ref: "#/components/schemas/" + name}
}

func (g *Generator) schema(typ reflect.Type) *openapi3.Schema {
	nullable := false
	for typ.Kind() == reflect.Ptr {
		nullable = true
		typ = typ.Elem()
	}

	var schema *openapi3.Schema
	switch typ.Kind() {
	case reflect.Bool:
		schema = openapi3.NewBoolSchema()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		schema = openapi3.NewInt32Schema()
	case reflect.Int64:
		schema = openapi3.NewInt64Schema()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		schema = openapi3.NewInt32Schema()
	case reflect.Uint64:
		schema = openapi3.NewInt64Schema()
	case reflect.Float32:
		schema = openapi3.NewFloat64Schema()
		schema.Format = "float"
	case reflect.Float64:
		schema = openapi3.NewFloat64Schema()
	case reflect.String:
		schema = openapi3.NewStringSchema()
	case reflect.Slice, reflect.Array:
		if typ.Elem().Kind() == reflect.Uint8 {
			schema = openapi3.NewBytesSchema()
			break
		}
		schema = openapi3.NewArraySchema()
		schema.Items = g.schemaRef(typ.Elem())
	case reflect.Map:
		schema = openapi3.NewObjectSchema()
		if typ.Key().Kind() == reflect.String {
			schema.WithAdditionalProperties(g.schema(typ.Elem()))
		} else {
			schema.WithAnyAdditionalProperties()
		}
	case reflect.Struct:
		if typ == reflect.TypeOf(time.Time{}) {
			schema = openapi3.NewDateTimeSchema()
			break
		}
		schema = g.objectSchema(typ)
	case reflect.Interface:
		schema = openapi3.NewObjectSchema().WithAnyAdditionalProperties()
	default:
		schema = openapi3.NewSchema()
	}

	if nullable {
		schema.Nullable = true
	}
	return schema
}

func (g *Generator) objectSchema(typ reflect.Type) *openapi3.Schema {
	schema := openapi3.NewObjectSchema()
	schema.Properties = openapi3.Schemas{}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue
		}
		name := tagName(field, "json")
		if name == "" {
			name = lowerFirst(field.Name)
		}
		schema.Properties[name] = g.fieldSchemaRefForType(typ, field)
		if hasBinding(field, "required") {
			schema.Required = append(schema.Required, name)
		}
	}

	return schema
}

func (g *Generator) addHTTPErrorSchema() {
	g.spec.Components.Schemas["HTTPError"] = &openapi3.SchemaRef{Value: openapi3.NewObjectSchema().
		WithProperty("code", openapi3.NewStringSchema()).
		WithProperty("error", openapi3.NewStringSchema()).
		WithRequired([]string{"code", "error"}).
		WithAnyAdditionalProperties()}

	g.spec.Components.Responses = openapi3.ResponseBodies{
		"HTTPError": &openapi3.ResponseRef{Value: openapi3.NewResponse().
			WithDescription("Error response").
			WithJSONSchemaRef(&openapi3.SchemaRef{Ref: "#/components/schemas/HTTPError"})},
	}
}

func operationID(route fox.RouteInfo) string {
	if route.HandlerName == "" {
		return route.Method + "_" + route.Path
	}
	replacer := strings.NewReplacer("/", "_", ".", "_", "-", "_", ":", "_", "*", "_")
	return strings.Trim(replacer.Replace(route.HandlerName), "_")
}

func schemaName(typ reflect.Type) string {
	pkg := typ.PkgPath()
	if idx := strings.LastIndex(pkg, "/"); idx >= 0 {
		pkg = pkg[idx+1:]
	}
	if pkg == "" {
		return typ.Name()
	}
	return sanitizeName(pkg + "_" + typ.Name())
}

func sanitizeName(value string) string {
	replacer := strings.NewReplacer("/", "_", ".", "_", "-", "_", ":", "_", "*", "_")
	return strings.Trim(replacer.Replace(value), "_")
}

var pathParamPattern = regexp.MustCompile(`[:*]([A-Za-z0-9_]+)`)

func openAPIPath(path string) string {
	return pathParamPattern.ReplaceAllString(path, `{$1}`)
}

func pathParamNames(path string) map[string]struct{} {
	matches := pathParamPattern.FindAllStringSubmatch(path, -1)
	names := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		names[match[1]] = struct{}{}
	}
	return names
}

func formatParamNames(params map[string]struct{}) string {
	names := make([]string, 0, len(params))
	for name := range params {
		names = append(names, name)
	}
	sort.Strings(names)
	return "[" + strings.Join(names, " ") + "]"
}

func tagName(field reflect.StructField, key string) string {
	tag := field.Tag.Get(key)
	if tag == "" || tag == "-" {
		return ""
	}
	name := strings.Split(tag, ",")[0]
	if name == "-" {
		return ""
	}
	return name
}

func hasBinding(field reflect.StructField, rule string) bool {
	for _, part := range strings.Split(field.Tag.Get("binding"), ",") {
		name := strings.Split(part, "=")[0]
		if name == rule {
			return true
		}
	}
	return false
}

func applyBinding(schema *openapi3.Schema, field reflect.StructField) {
	for _, part := range strings.Split(field.Tag.Get("binding"), ",") {
		name, value, _ := strings.Cut(part, "=")
		switch name {
		case "email":
			schema.Format = "email"
		case "url", "uri":
			schema.Format = "uri"
		case "uuid", "uuid4":
			schema.Format = "uuid"
		case "alphanum":
			schema.Pattern = "^[a-zA-Z0-9]+$"
		case "min", "gte":
			applyLowerBound(schema, field.Type, value)
		case "max", "lte":
			applyUpperBound(schema, field.Type, value)
		case "gt":
			if n, ok := parseBindingFloat(value); ok {
				schema.WithExclusiveMinValue(n)
			}
		case "lt":
			if n, ok := parseBindingFloat(value); ok {
				schema.WithExclusiveMaxValue(n)
			}
		case "len":
			applyLength(schema, field.Type, value)
		case "oneof":
			for _, item := range strings.Fields(value) {
				schema.Enum = append(schema.Enum, item)
			}
		}
	}
}

func applyLowerBound(schema *openapi3.Schema, typ reflect.Type, value string) {
	n, ok := parseBindingFloat(value)
	if !ok {
		return
	}
	switch deref(typ).Kind() {
	case reflect.String:
		schema.WithMinLength(int64(n))
	case reflect.Slice, reflect.Array:
		schema.WithMinItems(int64(n))
	default:
		schema.WithMin(n)
	}
}

func applyUpperBound(schema *openapi3.Schema, typ reflect.Type, value string) {
	n, ok := parseBindingFloat(value)
	if !ok {
		return
	}
	switch deref(typ).Kind() {
	case reflect.String:
		schema.WithMaxLength(int64(n))
	case reflect.Slice, reflect.Array:
		schema.WithMaxItems(int64(n))
	default:
		schema.WithMax(n)
	}
}

func applyLength(schema *openapi3.Schema, typ reflect.Type, value string) {
	n, ok := parseBindingFloat(value)
	if !ok {
		return
	}
	switch deref(typ).Kind() {
	case reflect.String:
		schema.WithLength(int64(n))
	case reflect.Slice, reflect.Array:
		schema.WithMinItems(int64(n))
		schema.WithMaxItems(int64(n))
	default:
		schema.WithMin(n)
		schema.WithMax(n)
	}
}

func deref(typ reflect.Type) reflect.Type {
	for typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	return typ
}

func isError(typ reflect.Type) bool {
	return typ.Implements(reflect.TypeOf((*error)(nil)).Elem())
}

func lowerFirst(value string) string {
	if value == "" {
		return ""
	}
	return strings.ToLower(value[:1]) + value[1:]
}

func parseBindingFloat(value string) (float64, bool) {
	n, err := strconv.ParseFloat(value, 64)
	return n, err == nil
}
