package common

func GetStringValue(val *string) string {
	if val != nil {
		return *val
	}
	return ""
}

func GetMapValue(properties map[string]interface{}, key string) map[string]interface{} {
	if value, ok := properties[key]; ok {
		if m, ok := value.(map[string]interface{}); ok {
			return m
		} else if n, ok := value.([]interface{}); ok {
			if len(n) > 0 {
				if o, ok := n[0].(map[string]interface{}); ok {
					return o
				}
			}
		}
	}
	return make(map[string]interface{})
}
