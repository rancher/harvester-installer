package config

import (
	"fmt"
	"strings"

	"github.com/rancher/mapper/convert"

	"github.com/harvester/harvester-installer/pkg/util"
)

const (
	kernelParamPrefix = "harvester"
)

// ReadConfig constructs a config by reading various sources
func ReadConfig() (HarvesterConfig, error) {
	result := NewHarvesterConfig()
	data, err := util.ReadCmdline(kernelParamPrefix)
	if err != nil {
		return *result, err
	}
	schema.Mapper.ToInternal(data)
	return *result, convert.ToObj(data, result)
}

func ToEnv(prefix string, obj interface{}) ([]string, error) {
	data, err := convert.EncodeToMap(obj)
	if err != nil {
		return nil, err
	}

	return mapToEnv(prefix, data), nil
}

func mapToEnv(prefix string, data map[string]interface{}) []string {
	var result []string
	for k, v := range data {
		keyName := strings.ToUpper(prefix + convert.ToYAMLKey(k))
		if data, ok := v.(map[string]interface{}); ok {
			subResult := mapToEnv(keyName+"_", data)
			result = append(result, subResult...)
		} else {
			result = append(result, fmt.Sprintf("%s=%v", keyName, v))
		}
	}
	return result
}
