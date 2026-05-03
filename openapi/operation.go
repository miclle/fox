package openapi

import (
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

type operationKey struct {
	Method string
	Path   string
}

type operationDoc struct {
	Summary     string
	Description string
	OperationID string
	Tags        []string
	Deprecated  *bool
	Responses   map[int]responseDoc
}

type responseDoc struct {
	Body        any
	Description string
}

// Operation adds explicit metadata for a registered route.
func Operation(method, path string, opts ...OperationOption) Option {
	return func(g *Generator) {
		doc := g.operations[operationKey{Method: strings.ToUpper(method), Path: path}]
		for _, opt := range opts {
			opt(&doc)
		}
		g.operations[operationKey{Method: strings.ToUpper(method), Path: path}] = doc
	}
}

// OperationOption configures explicit metadata for one operation.
type OperationOption func(*operationDoc)

// Summary sets the operation summary.
func Summary(value string) OperationOption {
	return func(doc *operationDoc) {
		doc.Summary = value
	}
}

// Description sets the operation description.
func Description(value string) OperationOption {
	return func(doc *operationDoc) {
		doc.Description = value
	}
}

// OperationID sets the operationId.
func OperationID(value string) OperationOption {
	return func(doc *operationDoc) {
		doc.OperationID = value
	}
}

// Tags sets the operation tags.
func Tags(values ...string) OperationOption {
	return func(doc *operationDoc) {
		doc.Tags = append([]string(nil), values...)
	}
}

// Deprecated marks the operation as deprecated.
func Deprecated(value bool) OperationOption {
	return func(doc *operationDoc) {
		doc.Deprecated = &value
	}
}

// Response adds or replaces a response for the operation.
func Response(status int, body any, description string) OperationOption {
	return func(doc *operationDoc) {
		if doc.Responses == nil {
			doc.Responses = make(map[int]responseDoc)
		}
		doc.Responses[status] = responseDoc{Body: body, Description: description}
	}
}

func (g *Generator) applyOperationDoc(op *openapi3.Operation, routeMethod, routePath string) {
	doc, ok := g.operations[operationKey{Method: strings.ToUpper(routeMethod), Path: routePath}]
	if !ok {
		return
	}
	if doc.Summary != "" {
		op.Summary = doc.Summary
	}
	if doc.Description != "" {
		op.Description = doc.Description
	}
	if doc.OperationID != "" {
		op.OperationID = doc.OperationID
	}
	if len(doc.Tags) > 0 {
		op.Tags = append([]string(nil), doc.Tags...)
	}
	if doc.Deprecated != nil {
		op.Deprecated = *doc.Deprecated
	}
	for status, response := range doc.Responses {
		op.Responses.Set(strconv.Itoa(status), g.explicitResponse(status, response))
	}
}

func (g *Generator) explicitResponse(status int, doc responseDoc) *openapi3.ResponseRef {
	description := doc.Description
	if description == "" {
		description = http.StatusText(status)
	}

	response := openapi3.NewResponse().WithDescription(description)
	if doc.Body == nil {
		return &openapi3.ResponseRef{Value: response}
	}

	value := reflect.ValueOf(doc.Body)
	if value.Kind() == reflect.Ptr && value.IsNil() {
		return &openapi3.ResponseRef{Value: response}
	}
	typ := value.Type()
	return &openapi3.ResponseRef{Value: response.WithJSONSchemaRef(g.schemaRef(typ))}
}
