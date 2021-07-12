package config

import (
	"fmt"

	"github.com/ghodss/yaml"
	"github.com/rancher/mapper"
	"github.com/rancher/mapper/convert"
)

var (
	schemas = mapper.NewSchemas().Init(func(s *mapper.Schemas) *mapper.Schemas {
		s.DefaultMappers = func() []mapper.Mapper {
			return []mapper.Mapper{
				NewToMap(),
				NewToSlice(),
				NewToBool(),
				&FuzzyNames{},
			}
		}
		return s
	}).MustImport(HarvesterConfig{})
	schema = schemas.Schema("harvesterConfig")
)

func LoadHarvesterConfig(yamlBytes []byte) (*HarvesterConfig, error) {
	result := NewHarvesterConfig()
	data := map[string]interface{}{}
	if err := yaml.Unmarshal(yamlBytes, &data); err != nil {
		return result, fmt.Errorf("failed to unmarshal yaml: %v", err)
	}
	schema.Mapper.ToInternal(data)
	return result, convert.ToObj(data, result)
}
