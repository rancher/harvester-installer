package config

import (
	"fmt"

	"github.com/ghodss/yaml"
	"github.com/rancher/mapper"
	"github.com/rancher/mapper/convert"
	"github.com/rancher/mapper/mappers"
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

type Converter func(val interface{}) interface{}

type typeConverter struct {
	mappers.DefaultMapper
	converter Converter
	fieldType string
	mappers   mapper.Mappers
}

func NewTypeConverter(fieldType string, converter Converter) mapper.Mapper {
	return &typeConverter{
		fieldType: fieldType,
		converter: converter,
	}
}

func NewToMap() mapper.Mapper {
	return NewTypeConverter("map[string]", func(val interface{}) interface{} {
		if m, ok := val.(map[string]interface{}); ok {
			obj := make(map[string]string, len(m))
			for k, v := range m {
				obj[k] = convert.ToString(v)
			}
			return obj
		}
		return val
	})
}

func NewToSlice() mapper.Mapper {
	return NewTypeConverter("array[string]", func(val interface{}) interface{} {
		if str, ok := val.(string); ok {
			return []string{str}
		}
		return val
	})
}

func NewToBool() mapper.Mapper {
	return NewTypeConverter("boolean", func(val interface{}) interface{} {
		if str, ok := val.(string); ok {
			return str == "true"
		}
		return val
	})
}

type FuzzyNames struct {
	mappers.DefaultMapper
	names map[string]string
}

func LoadHarvesterConfig(yamlBytes []byte) (*HarvesterConfig, error) {
	result := NewHarvesterConfig()
	data := map[string]interface{}{}
	if err := yaml.Unmarshal(yamlBytes, &data); err != nil {
		return result, fmt.Errorf("failed to unmarshal yaml: %v", err)
	}
	schema.Mapper.ToInternal(data)
	return result, convert.ToObj(data, result)
}
