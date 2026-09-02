package canonicaljson

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

var ErrInvalidJSON = errors.New("canonicaljson: invalid JSON")

func Marshal(data []byte) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	value, err := decodeValue(decoder)
	if err != nil {
		return nil, err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, ErrInvalidJSON
	}

	var out bytes.Buffer
	if err := encodeValue(&out, value); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

type object map[string]any

type array []any

func decodeValue(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}

	switch value := token.(type) {
	case json.Delim:
		switch value {
		case '{':
			result := object{}
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return nil, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
				}
				key, ok := keyToken.(string)
				if !ok {
					return nil, ErrInvalidJSON
				}
				if _, exists := result[key]; exists {
					return nil, fmt.Errorf("%w: duplicate object key %q", ErrInvalidJSON, key)
				}
				item, err := decodeValue(decoder)
				if err != nil {
					return nil, err
				}
				result[key] = item
			}
			if end, err := decoder.Token(); err != nil || end != json.Delim('}') {
				return nil, ErrInvalidJSON
			}
			return result, nil
		case '[':
			var result array
			for decoder.More() {
				item, err := decodeValue(decoder)
				if err != nil {
					return nil, err
				}
				result = append(result, item)
			}
			if end, err := decoder.Token(); err != nil || end != json.Delim(']') {
				return nil, ErrInvalidJSON
			}
			return result, nil
		default:
			return nil, ErrInvalidJSON
		}
	case string, bool, nil, json.Number:
		return value, nil
	default:
		return nil, ErrInvalidJSON
	}
}

func encodeValue(out *bytes.Buffer, value any) error {
	switch value := value.(type) {
	case object:
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		out.WriteByte('{')
		for i, key := range keys {
			if i > 0 {
				out.WriteByte(',')
			}
			encodedKey, _ := json.Marshal(key)
			out.Write(encodedKey)
			out.WriteByte(':')
			if err := encodeValue(out, value[key]); err != nil {
				return err
			}
		}
		out.WriteByte('}')
		return nil
	case array:
		out.WriteByte('[')
		for i, item := range value {
			if i > 0 {
				out.WriteByte(',')
			}
			if err := encodeValue(out, item); err != nil {
				return err
			}
		}
		out.WriteByte(']')
		return nil
	case string, bool, nil:
		encoded, _ := json.Marshal(value)
		out.Write(encoded)
		return nil
	case json.Number:
		number, err := canonicalNumber(value.String())
		if err != nil {
			return err
		}
		out.WriteString(number)
		return nil
	default:
		return ErrInvalidJSON
	}
}

func canonicalNumber(value string) (string, error) {
	negative := strings.HasPrefix(value, "-")
	if negative {
		value = value[1:]
	}

	exponent := 0
	if i := strings.IndexAny(value, "eE"); i >= 0 {
		var err error
		exponent, err = parseExponent(value[i+1:])
		if err != nil {
			return "", ErrInvalidJSON
		}
		value = value[:i]
	}

	fractionDigits := 0
	if i := strings.IndexByte(value, '.'); i >= 0 {
		fractionDigits = len(value) - i - 1
		value = value[:i] + value[i+1:]
	}

	value = strings.TrimLeft(value, "0")
	if value == "" {
		return "0", nil
	}

	for strings.HasSuffix(value, "0") {
		value = strings.TrimSuffix(value, "0")
		exponent++
	}
	exponent -= fractionDigits

	const maxDigits = 4096
	if len(value)+abs(exponent) > maxDigits {
		return "", fmt.Errorf("%w: number is too large", ErrInvalidJSON)
	}

	var result string
	switch {
	case exponent >= 0:
		result = value + strings.Repeat("0", exponent)
	case len(value)+exponent > 0:
		point := len(value) + exponent
		result = value[:point] + "." + value[point:]
	default:
		result = "0." + strings.Repeat("0", -exponent-len(value)) + value
	}

	if negative {
		result = "-" + result
	}
	return result, nil
}

func parseExponent(value string) (int, error) {
	if value == "" {
		return 0, ErrInvalidJSON
	}
	negative := false
	switch value[0] {
	case '+':
		value = value[1:]
	case '-':
		negative = true
		value = value[1:]
	}
	if value == "" || len(value) > 6 {
		return 0, ErrInvalidJSON
	}

	n := 0
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return 0, ErrInvalidJSON
		}
		n = n*10 + int(ch-'0')
	}
	if negative {
		n = -n
	}
	return n, nil
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
