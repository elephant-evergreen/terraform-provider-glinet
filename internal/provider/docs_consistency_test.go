package provider

import (
	"context"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/action"
	frameworkdatasource "github.com/hashicorp/terraform-plugin-framework/datasource"
	frameworkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
)

func registeredTypeNames(t *testing.T) map[string]bool {
	t.Helper()
	ctx := context.Background()
	provider := New("test")()
	names := map[string]bool{}
	for _, factory := range provider.Resources(ctx) {
		var response frameworkresource.MetadataResponse
		factory().Metadata(ctx, frameworkresource.MetadataRequest{ProviderTypeName: "glinet"}, &response)
		names[response.TypeName] = true
	}
	for _, factory := range provider.DataSources(ctx) {
		var response frameworkdatasource.MetadataResponse
		factory().Metadata(ctx, frameworkdatasource.MetadataRequest{ProviderTypeName: "glinet"}, &response)
		names[response.TypeName] = true
	}
	if withActions, ok := provider.(frameworkprovider.ProviderWithActions); ok {
		for _, factory := range withActions.Actions(ctx) {
			var response action.MetadataResponse
			factory().Metadata(ctx, action.MetadataRequest{ProviderTypeName: "glinet"}, &response)
			names[response.TypeName] = true
		}
	}
	return names
}

// TestCoverageDocumentsEveryRegisteredType keeps the coverage matrix and the
// registered resource set from drifting apart when a resource is added.
func TestCoverageDocumentsEveryRegisteredType(t *testing.T) {
	data, err := os.ReadFile("../../docs/coverage.md")
	if err != nil {
		t.Fatal(err)
	}
	for name := range registeredTypeNames(t) {
		if !strings.Contains(string(data), name) {
			t.Errorf("docs/coverage.md does not mention %s", name)
		}
	}
}

// TestDocumentedTypesAreRegistered fails when a document references a resource
// or data source that the provider no longer registers.
func TestDocumentedTypesAreRegistered(t *testing.T) {
	names := registeredTypeNames(t)
	pattern := regexp.MustCompile(`glinet_[a-z0-9_]+`)
	for _, document := range []string{"../../docs/coverage.md", "../../docs/typed-resources.md"} {
		data, err := os.ReadFile(document)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range pattern.FindAllString(string(data), -1) {
			if !names[name] {
				t.Errorf("%s references unregistered %s", document, name)
			}
		}
	}
}
