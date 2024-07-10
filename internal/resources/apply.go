package resources

import (
	"bytes"
	"io"
	"os"

	"go.uber.org/zap"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func transformToUnstructured(filename string, logger *zap.Logger) ([]unstructured.Unstructured, error) {
	file, err := readFromFile(filename)
	if err != nil {
		logger.Sugar().Fatal(err)
	}

	yamlBytes, err := splitYAML(file)
	if err != nil {
		logger.Sugar().Fatal(err)
	}

	var unstructuredResources []unstructured.Unstructured

	for _, yamlByte := range yamlBytes {
		var objMap map[string]interface{}
		if err := yaml.Unmarshal(yamlByte, &objMap); err != nil {
			return nil, err
		}
		unstructuredResource := unstructured.Unstructured{Object: objMap}
		unstructuredResources = append(unstructuredResources, unstructuredResource)
	}

	return unstructuredResources, nil
}

func readFromFile(filename string) ([]byte, error) {
	file, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func splitYAML(resources []byte) ([][]byte, error) {
	dec := yaml.NewDecoder(bytes.NewReader(resources))

	var res [][]byte
	for {
		var value interface{}
		err := dec.Decode(&value)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		valueBytes, err := yaml.Marshal(value)
		if err != nil {
			return nil, err
		}
		res = append(res, valueBytes)
	}
	return res, nil
}
