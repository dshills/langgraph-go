package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/dshills/langgraph-go/graph/emit"
)

// DefaultMaxResourceSize is the default maximum size for resources (10MB)
const DefaultMaxResourceSize = 10 * 1024 * 1024 // 10MB

// Resource URI validation pattern: supports both simple paths (e.g., "workflow_state/current")
// and full URIs with schemes (e.g., "file:///docs/readme.txt", "dynamic:///status/health")
// Pattern: lowercase start, allows lowercase letters, digits, underscores, colons, slashes, dots (no hyphens)
var resourceURIPattern = regexp.MustCompile(`^[a-z][a-z0-9_:/.]*$`)

// Errors for resource operations
var (
	ErrResourceNotFound      = errors.New("resource not found")
	ErrInvalidResourceURI    = errors.New("invalid resource URI")
	ErrResourceAlreadyExists = errors.New("resource already exists")
	ErrResourceTooLarge      = errors.New("resource exceeds size limit")
	ErrEmptyName             = errors.New("resource name cannot be empty")
	ErrInvalidGenerator      = errors.New("generator function cannot be nil")
)

// ResourceInfo contains metadata about a resource
type ResourceInfo struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MimeType    string `json:"mimeType"`
}

// ResourceContent represents the content returned by resources/read
type ResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"`
}

// Resource is the interface implemented by all resource types
type Resource interface {
	// URI returns the resource identifier
	URI() string

	// MimeType returns the content type
	MimeType() string

	// Read fetches resource content (may be cached or computed)
	Read(ctx context.Context) ([]byte, error)

	// Info returns the resource metadata
	Info() ResourceInfo
}

// StaticResource represents a resource with fixed content
type StaticResource struct {
	uri         string
	name        string
	description string
	mimeType    string
	content     []byte
}

// NewStaticResource creates a new static resource
func NewStaticResource(uri, name, description, mimeType string, content []byte) *StaticResource {
	return &StaticResource{
		uri:         uri,
		name:        name,
		description: description,
		mimeType:    mimeType,
		content:     content,
	}
}

// URI returns the resource URI
func (r *StaticResource) URI() string {
	return r.uri
}

// MimeType returns the MIME type
func (r *StaticResource) MimeType() string {
	return r.mimeType
}

// Read returns the static content
func (r *StaticResource) Read(ctx context.Context) ([]byte, error) {
	// Check for context cancellation
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return r.content, nil
}

// Info returns the resource metadata
func (r *StaticResource) Info() ResourceInfo {
	return ResourceInfo{
		URI:         r.uri,
		Name:        r.name,
		Description: r.description,
		MimeType:    r.mimeType,
	}
}

// DynamicResource represents a resource with computed content
type DynamicResource struct {
	uri         string
	name        string
	description string
	mimeType    string
	generator   func(context.Context) ([]byte, error)
}

// NewDynamicResource creates a new dynamic resource
func NewDynamicResource(uri, name, description, mimeType string, generator func(context.Context) ([]byte, error)) *DynamicResource {
	return &DynamicResource{
		uri:         uri,
		name:        name,
		description: description,
		mimeType:    mimeType,
		generator:   generator,
	}
}

// URI returns the resource URI
func (r *DynamicResource) URI() string {
	return r.uri
}

// MimeType returns the MIME type
func (r *DynamicResource) MimeType() string {
	return r.mimeType
}

// Read computes and returns the dynamic content
func (r *DynamicResource) Read(ctx context.Context) ([]byte, error) {
	// Check for context cancellation before computation
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	data, err := r.generator(ctx)
	if err != nil {
		// Check if error is due to cancellation
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("resource generation failed: %w", err)
	}

	return data, nil
}

// Info returns the resource metadata
func (r *DynamicResource) Info() ResourceInfo {
	return ResourceInfo{
		URI:         r.uri,
		Name:        r.name,
		Description: r.description,
		MimeType:    r.mimeType,
	}
}

// ResourceProvider manages resource registration and access
type ResourceProvider interface {
	// RegisterStatic registers a resource with fixed content
	RegisterStatic(uri, name, description, mimeType string, content []byte) error

	// RegisterDynamic registers a resource with computed content
	RegisterDynamic(uri, name, description, mimeType string, generator func(context.Context) ([]byte, error)) error

	// Get retrieves a resource by URI
	Get(uri string) (Resource, error)

	// List returns all resource metadata
	List() []ResourceInfo

	// Read fetches resource content by URI
	Read(ctx context.Context, uri string) ([]byte, error)
}

// resourceProvider is the default implementation of ResourceProvider
type resourceProvider struct {
	resources    map[string]Resource
	mu           sync.RWMutex
	emitter      emit.Emitter
	maxSize      int
	allowMutable bool // If false, registration after creation is forbidden
}

// ResourceProviderOption configures a ResourceProvider
type ResourceProviderOption func(*resourceProvider)

// WithMaxResourceSize sets the maximum resource size
func WithMaxResourceSize(maxSize int) ResourceProviderOption {
	return func(rp *resourceProvider) {
		rp.maxSize = maxSize
	}
}

// WithResourceEmitter sets the emitter for observability
func WithResourceEmitter(emitter emit.Emitter) ResourceProviderOption {
	return func(rp *resourceProvider) {
		rp.emitter = emitter
	}
}

// WithMutableResources allows resources to be registered after creation
func WithMutableResources(allow bool) ResourceProviderOption {
	return func(rp *resourceProvider) {
		rp.allowMutable = allow
	}
}

// NewResourceProvider creates a new resource provider
func NewResourceProvider(opts ...ResourceProviderOption) ResourceProvider {
	rp := &resourceProvider{
		resources:    make(map[string]Resource),
		maxSize:      DefaultMaxResourceSize,
		allowMutable: true,
	}

	for _, opt := range opts {
		opt(rp)
	}

	return rp
}

// validateURI checks if the URI matches the required pattern
func validateURI(uri string) error {
	if uri == "" {
		return fmt.Errorf("%w: uri parameter is required", ErrInvalidResourceURI)
	}
	if !resourceURIPattern.MatchString(uri) {
		return fmt.Errorf("%w: uri must match pattern ^[a-z][a-z0-9_:/.]*$ (got: %s)", ErrInvalidResourceURI, uri)
	}
	return nil
}

// RegisterStatic registers a static resource with fixed content
func (rp *resourceProvider) RegisterStatic(uri, name, description, mimeType string, content []byte) error {
	// Validate URI
	if err := validateURI(uri); err != nil {
		return err
	}

	// Validate name and description
	if name == "" {
		return fmt.Errorf("%w: name cannot be empty for resource '%s'", ErrEmptyName, uri)
	}
	if description == "" {
		return fmt.Errorf("%w: description cannot be empty for resource '%s'", ErrEmptyDescription, uri)
	}

	// Check size limit
	if len(content) > rp.maxSize {
		return fmt.Errorf("%w: resource '%s' size %d exceeds limit %d", ErrResourceTooLarge, uri, len(content), rp.maxSize)
	}

	// Check for duplicate
	rp.mu.Lock()
	defer rp.mu.Unlock()

	if _, exists := rp.resources[uri]; exists {
		return fmt.Errorf("%w: resource '%s' already registered", ErrResourceAlreadyExists, uri)
	}

	// Create and register resource
	resource := NewStaticResource(uri, name, description, mimeType, content)
	rp.resources[uri] = resource

	// Emit observability event
	if rp.emitter != nil {
		rp.emitter.Emit(emit.Event{
			Msg: "resource_registered",
			Meta: map[string]interface{}{
				"uri":      uri,
				"type":     "static",
				"mimeType": mimeType,
				"size":     len(content),
			},
		})
	}

	return nil
}

// RegisterDynamic registers a dynamic resource with computed content
func (rp *resourceProvider) RegisterDynamic(uri, name, description, mimeType string, generator func(context.Context) ([]byte, error)) error {
	// Validate generator
	if generator == nil {
		return fmt.Errorf("%w: generator cannot be nil for resource '%s'", ErrInvalidGenerator, uri)
	}

	// Validate URI
	if err := validateURI(uri); err != nil {
		return err
	}

	// Validate name and description
	if name == "" {
		return fmt.Errorf("%w: name cannot be empty for resource '%s'", ErrEmptyName, uri)
	}
	if description == "" {
		return fmt.Errorf("%w: description cannot be empty for resource '%s'", ErrEmptyDescription, uri)
	}

	// Check for duplicate
	rp.mu.Lock()
	defer rp.mu.Unlock()

	if _, exists := rp.resources[uri]; exists {
		return fmt.Errorf("%w: resource '%s' already registered", ErrResourceAlreadyExists, uri)
	}

	// Create and register resource
	resource := NewDynamicResource(uri, name, description, mimeType, generator)
	rp.resources[uri] = resource

	// Emit observability event
	if rp.emitter != nil {
		rp.emitter.Emit(emit.Event{
			Msg: "resource_registered",
			Meta: map[string]interface{}{
				"uri":      uri,
				"type":     "dynamic",
				"mimeType": mimeType,
			},
		})
	}

	return nil
}

// Get retrieves a resource by URI
func (rp *resourceProvider) Get(uri string) (Resource, error) {
	rp.mu.RLock()
	defer rp.mu.RUnlock()

	resource, exists := rp.resources[uri]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrResourceNotFound, uri)
	}

	return resource, nil
}

// List returns all resource metadata
func (rp *resourceProvider) List() []ResourceInfo {
	rp.mu.RLock()
	defer rp.mu.RUnlock()

	infos := make([]ResourceInfo, 0, len(rp.resources))
	for _, resource := range rp.resources {
		infos = append(infos, resource.Info())
	}

	return infos
}

// Read fetches resource content by URI and enforces size limits
func (rp *resourceProvider) Read(ctx context.Context, uri string) ([]byte, error) {
	// Validate URI
	if err := validateURI(uri); err != nil {
		return nil, err
	}

	// Get the resource
	resource, err := rp.Get(uri)
	if err != nil {
		return nil, err
	}

	// Emit read start event
	if rp.emitter != nil {
		rp.emitter.Emit(emit.Event{
			Msg: "resource_read_start",
			Meta: map[string]interface{}{
				"uri": uri,
			},
		})
	}

	// Read content
	content, err := resource.Read(ctx)
	if err != nil {
		// Emit read error event
		if rp.emitter != nil {
			rp.emitter.Emit(emit.Event{
				Msg: "resource_read_error",
				Meta: map[string]interface{}{
					"uri":   uri,
					"error": err.Error(),
				},
			})
		}
		return nil, fmt.Errorf("failed to read resource '%s': %w", uri, err)
	}

	// Check size limit for dynamic resources (static already checked at registration)
	if len(content) > rp.maxSize {
		// Emit size limit error
		if rp.emitter != nil {
			rp.emitter.Emit(emit.Event{
				Msg: "resource_size_exceeded",
				Meta: map[string]interface{}{
					"uri":     uri,
					"size":    len(content),
					"maxSize": rp.maxSize,
				},
			})
		}
		return nil, fmt.Errorf("%w: resource '%s' size %d exceeds limit %d", ErrResourceTooLarge, uri, len(content), rp.maxSize)
	}

	// Emit read success event
	if rp.emitter != nil {
		rp.emitter.Emit(emit.Event{
			Msg: "resource_read_complete",
			Meta: map[string]interface{}{
				"uri":  uri,
				"size": len(content),
			},
		})
	}

	return content, nil
}

// FormatResourceContent formats resource content for JSON-RPC response
func FormatResourceContent(uri, mimeType string, content []byte) ResourceContent {
	rc := ResourceContent{
		URI:      uri,
		MimeType: mimeType,
	}

	// Determine if content is text-based or binary based on MIME type
	if isTextMIME(mimeType) {
		rc.Text = string(content)
	} else {
		// Binary content: base64 encode
		rc.Blob = base64.StdEncoding.EncodeToString(content)
	}

	return rc
}

// isTextMIME determines if a MIME type represents text-based content
func isTextMIME(mimeType string) bool {
	// Text-based MIME types use the "text" field
	textPrefixes := []string{
		"text/",
		"application/json",
		"application/xml",
		"application/javascript",
		"application/ld+json",
	}

	lower := strings.ToLower(mimeType)
	for _, prefix := range textPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}

	return false
}

// ParseResourceContent parses ResourceContent back to bytes
func ParseResourceContent(rc ResourceContent) ([]byte, error) {
	if rc.Text != "" {
		return []byte(rc.Text), nil
	}
	if rc.Blob != "" {
		return base64.StdEncoding.DecodeString(rc.Blob)
	}
	return nil, errors.New("resource content has neither text nor blob field")
}

// ResourcesListResult is the result for resources/list method
type ResourcesListResult struct {
	Resources []ResourceInfo `json:"resources"`
}

// ResourcesReadParams are the parameters for resources/read method
type ResourcesReadParams struct {
	URI string `json:"uri"`
}

// ResourcesReadResult is the result for resources/read method
type ResourcesReadResult struct {
	Contents []ResourceContent `json:"contents"`
}

// MarshalResourcesListResult creates the JSON-RPC result for resources/list
func MarshalResourcesListResult(resources []ResourceInfo) (interface{}, error) {
	result := ResourcesListResult{
		Resources: resources,
	}
	// Return as map for JSON-RPC result field
	data, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	var resultMap map[string]interface{}
	if err := json.Unmarshal(data, &resultMap); err != nil {
		return nil, err
	}
	return resultMap, nil
}

// MarshalResourcesReadResult creates the JSON-RPC result for resources/read
func MarshalResourcesReadResult(uri, mimeType string, content []byte) (interface{}, error) {
	resourceContent := FormatResourceContent(uri, mimeType, content)
	result := ResourcesReadResult{
		Contents: []ResourceContent{resourceContent},
	}
	// Return as map for JSON-RPC result field
	data, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	var resultMap map[string]interface{}
	if err := json.Unmarshal(data, &resultMap); err != nil {
		return nil, err
	}
	return resultMap, nil
}
