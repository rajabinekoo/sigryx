package canonicaljson

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

var (
	ErrInvalidPointer   = errors.New("canonicaljson: invalid JSON pointer")
	ErrPointerNotFound  = errors.New("canonicaljson: JSON pointer not found")
	ErrDuplicatePointer = errors.New("canonicaljson: duplicate JSON pointer")
)

// Select returns a canonical JSON object whose keys are normalized RFC 6901
// JSON pointers and whose values are the corresponding values from data.
// The pointer set is normalized, deduplicated, and sorted by normal JSON object
// canonicalization. Nested objects and arrays are supported.
func Select(data []byte, pointers []string) ([]byte, []string, error) {
	if len(pointers) == 0 {
		return nil, nil, fmt.Errorf("%w: at least one pointer is required", ErrInvalidPointer)
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	root, err := decodeValue(decoder)
	if err != nil {
		return nil, nil, err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, nil, ErrInvalidJSON
	}

	selected := object{}
	normalized := make([]string, 0, len(pointers))
	for _, pointer := range pointers {
		tokens, canonical, err := parsePointer(pointer)
		if err != nil {
			return nil, nil, err
		}
		if _, exists := selected[canonical]; exists {
			return nil, nil, fmt.Errorf("%w: %s", ErrDuplicatePointer, canonical)
		}
		value, err := resolvePointer(root, tokens)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: %s", err, canonical)
		}
		selected[canonical] = value
		normalized = append(normalized, canonical)
	}

	var out bytes.Buffer
	if err := encodeValue(&out, selected); err != nil {
		return nil, nil, err
	}

	// The object encoder sorts keys. Sort the returned schema as well so callers
	// can compare pointer sets independent of request order.
	sort.Strings(normalized)

	return out.Bytes(), normalized, nil
}

func parsePointer(pointer string) ([]string, string, error) {
	if pointer == "" || !strings.HasPrefix(pointer, "/") {
		return nil, "", fmt.Errorf("%w: %q", ErrInvalidPointer, pointer)
	}

	raw := strings.Split(pointer[1:], "/")
	tokens := make([]string, len(raw))
	normalized := make([]string, len(raw))
	for i, token := range raw {
		decoded, err := unescapePointerToken(token)
		if err != nil {
			return nil, "", err
		}
		tokens[i] = decoded
		normalized[i] = strings.ReplaceAll(strings.ReplaceAll(decoded, "~", "~0"), "/", "~1")
	}
	return tokens, "/" + strings.Join(normalized, "/"), nil
}

func unescapePointerToken(token string) (string, error) {
	var out strings.Builder
	for i := 0; i < len(token); i++ {
		if token[i] != '~' {
			out.WriteByte(token[i])
			continue
		}
		if i+1 >= len(token) {
			return "", fmt.Errorf("%w: invalid escape", ErrInvalidPointer)
		}
		i++
		switch token[i] {
		case '0':
			out.WriteByte('~')
		case '1':
			out.WriteByte('/')
		default:
			return "", fmt.Errorf("%w: invalid escape", ErrInvalidPointer)
		}
	}
	return out.String(), nil
}

func resolvePointer(value any, tokens []string) (any, error) {
	current := value
	for _, token := range tokens {
		switch node := current.(type) {
		case object:
			item, ok := node[token]
			if !ok {
				return nil, ErrPointerNotFound
			}
			current = item
		case array:
			if token == "-" || token == "" || (len(token) > 1 && token[0] == '0') {
				return nil, ErrPointerNotFound
			}
			index, err := strconv.Atoi(token)
			if err != nil || index < 0 || index >= len(node) {
				return nil, ErrPointerNotFound
			}
			current = node[index]
		default:
			return nil, ErrPointerNotFound
		}
	}
	return current, nil
}
