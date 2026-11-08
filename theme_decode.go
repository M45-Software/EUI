package eui

import (
	"encoding/json"
	"reflect"
	"strings"
)

// Resolve color references only at Color-typed fields; labels and comments
// that happen to equal a color name remain unchanged.
func expandThemeColorJSON(data []byte, colors map[string]Color) ([]byte, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return nil, err
	}
	var expand func(map[string]json.RawMessage, reflect.Type) error
	expand = func(values map[string]json.RawMessage, typ reflect.Type) error {
		for key, raw := range values {
			field, ok := typ.FieldByNameFunc(func(name string) bool { return strings.EqualFold(name, key) })
			if !ok || !field.IsExported() {
				continue
			}
			if field.Type == reflect.TypeFor[Color]() {
				var name string
				if json.Unmarshal(raw, &name) == nil {
					if color, ok := colors[strings.ToLower(strings.TrimSpace(name))]; ok {
						encoded, err := json.Marshal(struct{ R, G, B, A uint8 }{color.R, color.G, color.B, color.A})
						if err != nil {
							return err
						}
						values[key] = encoded
					}
				}
			} else if field.Type.Kind() == reflect.Struct {
				var child map[string]json.RawMessage
				if err := json.Unmarshal(raw, &child); err != nil {
					return err
				}
				if err := expand(child, field.Type); err != nil {
					return err
				}
				encoded, err := json.Marshal(child)
				if err != nil {
					return err
				}
				values[key] = encoded
			}
		}
		return nil
	}
	if err := expand(object, reflect.TypeFor[Theme]()); err != nil {
		return nil, err
	}
	return json.Marshal(object)
}
