package common

func GetStringValue(val *string) string {
	if val != nil {
		return *val
	}
	return ""
}

func GetMapValue(properties map[string]any, key string) map[string]any {
	if value, ok := properties[key]; ok {
		if m, ok := value.(map[string]any); ok {
			return m
		} else if n, ok := value.([]any); ok {
			if len(n) > 0 {
				if o, ok := n[0].(map[string]any); ok {
					return o
				}
			}
		}
	}
	return make(map[string]any)
}

func AddAttribute(attribute map[string][]string, key string, value string) {
	if attribute[key] == nil {
		attribute[key] = make([]string, 0)
	}
	attribute[key] = append(attribute[key], value)
}
