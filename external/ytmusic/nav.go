package ytmusic

import "strings"

// The InnerTube API answers with deeply nested renderer objects. These
// helpers walk them without a struct for every renderer, and return zero
// values for any missing step, so a layout change degrades to missing data
// instead of a crash.

// get follows path through maps (string keys) and slices (int indexes).
func get(o any, path ...any) any {
	cur := o
	for _, step := range path {
		switch key := step.(type) {
		case string:
			m, ok := cur.(map[string]any)
			if !ok {
				return nil
			}
			cur = m[key]
		case int:
			s, ok := cur.([]any)
			if !ok {
				return nil
			}
			if key < 0 {
				key += len(s)
			}
			if key < 0 || key >= len(s) {
				return nil
			}
			cur = s[key]
		default:
			return nil
		}
		if cur == nil {
			return nil
		}
	}
	return cur
}

func str(o any, path ...any) string {
	s, _ := get(o, path...).(string)
	return s
}

func arr(o any, path ...any) []any {
	s, _ := get(o, path...).([]any)
	return s
}

func obj(o any, path ...any) map[string]any {
	m, _ := get(o, path...).(map[string]any)
	return m
}

// text returns the text of a runs or simpleText object.
func text(o any) string {
	m, ok := o.(map[string]any)
	if !ok {
		return ""
	}
	if s, ok := m["simpleText"].(string); ok {
		return s
	}
	var b strings.Builder
	for _, r := range arr(m, "runs") {
		b.WriteString(str(r, "text"))
	}
	return b.String()
}

// findAll collects every value stored under key anywhere inside o. Slices
// keep their order; sibling map keys are visited in no fixed order. limit
// caps the results; 0 means no cap.
func findAll(o any, key string, limit int) []any {
	var out []any
	var walk func(any) bool
	walk = func(v any) bool {
		switch t := v.(type) {
		case map[string]any:
			if found, ok := t[key]; ok {
				out = append(out, found)
				if limit > 0 && len(out) >= limit {
					return false
				}
			}
			for k, child := range t {
				if k == key {
					continue
				}
				if !walk(child) {
					return false
				}
			}
		case []any:
			for _, child := range t {
				if !walk(child) {
					return false
				}
			}
		}
		return true
	}
	walk(o)
	return out
}

// findFirst returns the first value stored under key inside o.
func findFirst(o any, key string) any {
	if found := findAll(o, key, 1); len(found) > 0 {
		return found[0]
	}
	return nil
}
