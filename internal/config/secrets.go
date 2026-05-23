package config

import (
	"fmt"
	"regexp"
)

type SecretMap map[string]string

var tokenRE = regexp.MustCompile(`\$([A-Z_][A-Z0-9_]*)`)

func (s SecretMap) Get(key string) (string, error) {
	v, ok := s[key]
	if !ok {
		return "", fmt.Errorf("secret %q not found", key)
	}
	return v, nil
}

func (s SecretMap) Expand(in string) (string, error) {
	var errToken string
	out := tokenRE.ReplaceAllStringFunc(in, func(match string) string {
		key := match[1:]
		v, ok := s[key]
		if !ok {
			errToken = key
			return match
		}
		return v
	})
	if errToken != "" {
		return "", fmt.Errorf("secret %q referenced but not present", errToken)
	}
	return out, nil
}

func (s SecretMap) ExpandMap(in map[string]string) (map[string]string, error) {
	out := make(map[string]string, len(in))
	for k, v := range in {
		ev, err := s.Expand(v)
		if err != nil {
			return nil, err
		}
		out[k] = ev
	}
	return out, nil
}
