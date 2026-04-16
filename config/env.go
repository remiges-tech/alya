package config

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
)

// Env reads configuration from environment variables.
//
// Nested struct fields map to uppercase keys joined with underscores.
// For example, with prefix APP, database.host becomes APP_DATABASE_HOST.
type Env struct {
	Prefix string
}

// NewEnv creates an environment-backed config source.
func NewEnv(prefix string) *Env {
	return &Env{Prefix: prefix}
}

func (e *Env) Check() error {
	return nil
}

func (e *Env) Load(target any) error {
	if target == nil {
		return fmt.Errorf("target cannot be nil")
	}

	value := reflect.ValueOf(target)
	if value.Kind() != reflect.Ptr || value.IsNil() {
		return fmt.Errorf("target must be a non-nil pointer")
	}
	if value.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("target must point to a struct")
	}

	return e.loadStruct(value.Elem(), nil)
}

func (e *Env) LoadConfig(target any) error {
	return e.Load(target)
}

func (e *Env) Get(key string) (string, error) {
	envKey := e.envKey(key)
	value, ok := os.LookupEnv(envKey)
	if !ok {
		return "", &KeyNotFoundError{Key: key}
	}
	return value, nil
}

func (e *Env) GetInt(key string) (int, error) {
	value, err := e.Get(key)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(value)
}

func (e *Env) GetBool(key string) (bool, error) {
	value, err := e.Get(key)
	if err != nil {
		return false, err
	}
	return strconv.ParseBool(value)
}

// Watch is not supported for environment-backed config.
func (e *Env) Watch(ctx context.Context, key string, events chan<- Event) error {
	return fmt.Errorf("watch is not supported for environment config")
}

func (e *Env) envKey(key string) string {
	segments := strings.FieldsFunc(key, func(r rune) bool {
		return r == '.' || r == '-'
	})
	return buildEnvKey(e.Prefix, segments)
}

func (e *Env) loadStruct(value reflect.Value, path []string) error {
	valueType := value.Type()
	for i := 0; i < value.NumField(); i++ {
		fieldType := valueType.Field(i)
		if fieldType.PkgPath != "" {
			continue
		}

		fieldValue := value.Field(i)
		segment := configSegment(fieldType)
		if segment == "" {
			continue
		}

		if fieldValue.Kind() == reflect.Struct {
			if err := e.loadStruct(fieldValue, append(path, segment)); err != nil {
				return err
			}
			continue
		}

		envKey := buildEnvKey(e.Prefix, append(path, segment))
		envValue, ok := os.LookupEnv(envKey)
		if !ok {
			continue
		}

		if err := setFieldValue(fieldValue, envValue); err != nil {
			return fmt.Errorf("invalid value for %s: %w", envKey, err)
		}
	}

	return nil
}

func configSegment(field reflect.StructField) string {
	if tag := strings.TrimSpace(field.Tag.Get("env")); tag != "" {
		if tag == "-" {
			return ""
		}
		return tag
	}

	if tag := strings.TrimSpace(field.Tag.Get("json")); tag != "" {
		name := strings.Split(tag, ",")[0]
		if name == "-" {
			return ""
		}
		if name != "" {
			return name
		}
	}

	return field.Name
}

func buildEnvKey(prefix string, segments []string) string {
	parts := make([]string, 0, len(segments)+1)
	if prefix != "" {
		parts = append(parts, normalizeEnvSegment(prefix))
	}
	for _, segment := range segments {
		if segment == "" {
			continue
		}
		parts = append(parts, normalizeEnvSegment(segment))
	}
	return strings.Join(parts, "_")
}

func normalizeEnvSegment(segment string) string {
	segment = strings.TrimSpace(segment)
	segment = strings.ReplaceAll(segment, ".", "_")
	segment = strings.ReplaceAll(segment, "-", "_")
	return strings.ToUpper(segment)
}

func setFieldValue(field reflect.Value, raw string) error {
	if !field.CanSet() {
		return fmt.Errorf("field cannot be set")
	}

	switch field.Kind() {
	case reflect.String:
		field.SetString(raw)
		return nil
	case reflect.Bool:
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return err
		}
		field.SetBool(value)
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value, err := strconv.ParseInt(raw, 10, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetInt(value)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		value, err := strconv.ParseUint(raw, 10, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetUint(value)
		return nil
	case reflect.Float32, reflect.Float64:
		value, err := strconv.ParseFloat(raw, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetFloat(value)
		return nil
	default:
		return fmt.Errorf("unsupported field type %s", field.Kind())
	}
}
