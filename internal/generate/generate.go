// generate/composite_resource.go

package generate

import (
	"context"
	"encoding/json"
	"os"

	"github.com/charmbracelet/log"
	crossv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// xr represents a Crossplane composite resource with metadata and specifications.
type xr struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              map[string]interface{} `json:"spec"`
}

// GenerateCompositeResource reads a CompositeResourceDefinition from a YAML file,
// generates an example composite resource, and prints it as YAML.
func GenerateCompositeResource(ctx context.Context, path string, logger *log.Logger) error {
	data, err := os.ReadFile(path)
	if err != nil {
		logger.Errorf("failed to read file: %v", err)
	}

	var xrd crossv1.CompositeResourceDefinition
	if err := yaml.Unmarshal(data, &xrd); err != nil {
		logger.Errorf("failed to unmarshal YAML: %v", err)
	}

	xrSpec, err := generateSpec(xrd, logger)
	if err != nil {
		logger.Errorf("failed to generate spec: %v", err)
	}

	xr := xr{
		TypeMeta: metav1.TypeMeta{
			Kind:       xrd.Spec.Names.Kind,
			APIVersion: xrd.Spec.Group + "/" + xrd.Spec.Versions[0].Name,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "example-resource",
		},
		Spec: xrSpec,
	}

	yamlXR, err := yaml.Marshal(xr)
	if err != nil {
		logger.Errorf("failed to marshal YAML: %v", err)
	}

	logger.Print(string(yamlXR))
	return nil
}

// generateSpec creates an example spec map based on the schema defined in the CompositeResourceDefinition.
func generateSpec(xrd crossv1.CompositeResourceDefinition, logger *log.Logger) (map[string]interface{}, error) {
	xrSpec := make(map[string]interface{})
	rawData := xrd.Spec.Versions[0].Schema.OpenAPIV3Schema.Raw

	var schema map[string]interface{}
	if err := json.Unmarshal(rawData, &schema); err != nil {
		logger.Errorf("failed to unmarshal schema JSON: %v", err)
	}

	specProperties, ok := schema["properties"].(map[string]interface{})["spec"].(map[string]interface{})["properties"].(map[string]interface{})
	if !ok {
		logger.Error("invalid schema format")
	}

	for key, prop := range specProperties {
		propMap, ok := prop.(map[string]interface{})
		if !ok {
			xrSpec[key] = nil
			continue
		}
		xrSpec[key] = assignValues(propMap)
	}
	return xrSpec, nil
}

// asignValues checks each type of properties and assign example data for a given property schema.
func assignValues(propMap map[string]interface{}) interface{} {
	if types, ok := propMap["type"].(string); ok {
		switch types {
		case "string":
			return "example"
		case "integer":
			return 123
		case "boolean":
			return false
		case "object":
			return processObjectTypes(propMap)
		default:
			return nil
		}
	}
	return nil
}

// processObjectTypes recursively generates example data for object properties.
func processObjectTypes(objMap map[string]interface{}) map[string]interface{} {
	data := make(map[string]interface{})

	properties, ok := objMap["properties"].(map[string]interface{})
	if !ok {
		return data
	}

	for key, prop := range properties {
		propMap, ok := prop.(map[string]interface{})
		if !ok {
			data[key] = nil
			continue
		}
		data[key] = assignValues(propMap)
	}

	return data
}
