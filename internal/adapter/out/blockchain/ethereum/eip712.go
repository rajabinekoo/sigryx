package ethereum

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"sort"
	"strconv"
	"strings"
)

type typedField struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type typedDataEnvelope struct {
	Types       map[string][]typedField `json:"types"`
	PrimaryType string                  `json:"primaryType"`
	Domain      map[string]any          `json:"domain"`
	Message     map[string]any          `json:"message"`
}

func typedDataDigest(raw []byte) ([32]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()

	var data typedDataEnvelope
	if err := decoder.Decode(&data); err != nil {
		return [32]byte{}, fmt.Errorf("%w: %v", ErrInvalidTypedData, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return [32]byte{}, fmt.Errorf("%w: trailing JSON data", ErrInvalidTypedData)
	}
	if data.PrimaryType == "" || data.Types == nil || data.Domain == nil || data.Message == nil {
		return [32]byte{}, fmt.Errorf("%w: missing required fields", ErrInvalidTypedData)
	}
	if _, ok := data.Types["EIP712Domain"]; !ok {
		return [32]byte{}, fmt.Errorf("%w: missing EIP712Domain type", ErrInvalidTypedData)
	}
	if _, ok := data.Types[data.PrimaryType]; !ok {
		return [32]byte{}, fmt.Errorf("%w: primary type %q is not defined", ErrInvalidTypedData, data.PrimaryType)
	}

	domainHash, err := hashStruct(data.Types, "EIP712Domain", data.Domain)
	if err != nil {
		return [32]byte{}, err
	}
	messageHash, err := hashStruct(data.Types, data.PrimaryType, data.Message)
	if err != nil {
		return [32]byte{}, err
	}

	encoded := make([]byte, 0, 66)
	encoded = append(encoded, 0x19, 0x01)
	encoded = append(encoded, domainHash[:]...)
	encoded = append(encoded, messageHash[:]...)
	return keccak256(encoded), nil
}

func hashStruct(types map[string][]typedField, typeName string, value map[string]any) ([32]byte, error) {
	encodedType, err := encodeType(types, typeName)
	if err != nil {
		return [32]byte{}, err
	}
	typeHash := keccak256([]byte(encodedType))

	encoded := make([]byte, 0, 32*(len(types[typeName])+1))
	encoded = append(encoded, typeHash[:]...)
	for _, field := range types[typeName] {
		fieldValue, ok := value[field.Name]
		if !ok {
			return [32]byte{}, fmt.Errorf("%w: missing field %s.%s", ErrInvalidTypedData, typeName, field.Name)
		}
		word, err := encodeTypedValue(types, field.Type, fieldValue)
		if err != nil {
			return [32]byte{}, fmt.Errorf("%w: field %s.%s: %v", ErrInvalidTypedData, typeName, field.Name, err)
		}
		encoded = append(encoded, word[:]...)
	}
	return keccak256(encoded), nil
}

func encodeType(types map[string][]typedField, primary string) (string, error) {
	if _, ok := types[primary]; !ok {
		return "", fmt.Errorf("%w: undefined type %q", ErrInvalidTypedData, primary)
	}

	seen := map[string]bool{primary: true}
	var dependencies []string
	var walk func(string) error
	walk = func(name string) error {
		fields, ok := types[name]
		if !ok {
			return fmt.Errorf("%w: undefined type %q", ErrInvalidTypedData, name)
		}
		for _, field := range fields {
			if field.Name == "" || field.Type == "" {
				return fmt.Errorf("%w: empty type field", ErrInvalidTypedData)
			}
			base := baseType(field.Type)
			if isPrimitiveType(base) {
				if err := validateType(field.Type, types); err != nil {
					return err
				}
				continue
			}
			if _, ok := types[base]; !ok {
				return fmt.Errorf("%w: undefined type %q", ErrInvalidTypedData, base)
			}
			if seen[base] {
				continue
			}
			seen[base] = true
			dependencies = append(dependencies, base)
			if err := walk(base); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(primary); err != nil {
		return "", err
	}
	sort.Strings(dependencies)

	var out strings.Builder
	out.WriteString(typeDefinition(primary, types[primary]))
	for _, dependency := range dependencies {
		out.WriteString(typeDefinition(dependency, types[dependency]))
	}
	return out.String(), nil
}

func typeDefinition(name string, fields []typedField) string {
	var out strings.Builder
	out.WriteString(name)
	out.WriteByte('(')
	for i, field := range fields {
		if i > 0 {
			out.WriteByte(',')
		}
		out.WriteString(field.Type)
		out.WriteByte(' ')
		out.WriteString(field.Name)
	}
	out.WriteByte(')')
	return out.String()
}

func encodeTypedValue(types map[string][]typedField, typeName string, value any) ([32]byte, error) {
	if elementType, fixedLength, ok := arrayType(typeName); ok {
		items, ok := value.([]any)
		if !ok {
			return [32]byte{}, errors.New("expected array")
		}
		if fixedLength >= 0 && len(items) != fixedLength {
			return [32]byte{}, fmt.Errorf("expected array length %d", fixedLength)
		}
		encoded := make([]byte, 0, len(items)*32)
		for _, item := range items {
			word, err := encodeTypedValue(types, elementType, item)
			if err != nil {
				return [32]byte{}, err
			}
			encoded = append(encoded, word[:]...)
		}
		return keccak256(encoded), nil
	}

	if _, ok := types[typeName]; ok {
		object, ok := value.(map[string]any)
		if !ok {
			return [32]byte{}, fmt.Errorf("expected object for %s", typeName)
		}
		return hashStruct(types, typeName, object)
	}

	switch typeName {
	case "string":
		text, ok := value.(string)
		if !ok {
			return [32]byte{}, errors.New("expected string")
		}
		return keccak256([]byte(text)), nil
	case "bytes":
		text, ok := value.(string)
		if !ok {
			return [32]byte{}, errors.New("expected hex string")
		}
		decoded, err := decodeHex(text)
		if err != nil {
			return [32]byte{}, err
		}
		return keccak256(decoded), nil
	case "bool":
		boolean, ok := value.(bool)
		if !ok {
			return [32]byte{}, errors.New("expected boolean")
		}
		var out [32]byte
		if boolean {
			out[31] = 1
		}
		return out, nil
	case "address":
		text, ok := value.(string)
		if !ok {
			return [32]byte{}, errors.New("expected address string")
		}
		address, err := decodeFixedHex(text, 20)
		if err != nil {
			return [32]byte{}, err
		}
		var out [32]byte
		copy(out[12:], address)
		return out, nil
	}

	if size, ok := bytesN(typeName); ok {
		text, ok := value.(string)
		if !ok {
			return [32]byte{}, errors.New("expected hex string")
		}
		decoded, err := decodeFixedHex(text, size)
		if err != nil {
			return [32]byte{}, err
		}
		var out [32]byte
		copy(out[:], decoded)
		return out, nil
	}
	if bits, signed, ok := integerType(typeName); ok {
		return encodeInteger(value, bits, signed)
	}
	return [32]byte{}, fmt.Errorf("unsupported type %q", typeName)
}

func encodeInteger(value any, bits int, signed bool) ([32]byte, error) {
	text, err := typedIntegerString(value)
	if err != nil {
		return [32]byte{}, err
	}

	base := 10
	negative := strings.HasPrefix(text, "-")
	unsignedText := text
	if negative {
		unsignedText = text[1:]
	}
	if strings.HasPrefix(unsignedText, "0x") || strings.HasPrefix(unsignedText, "0X") {
		if negative {
			return [32]byte{}, errors.New("negative hexadecimal integers are not supported")
		}
		base = 16
		unsignedText = unsignedText[2:]
	}
	if unsignedText == "" {
		return [32]byte{}, errors.New("empty integer")
	}

	n, ok := new(big.Int).SetString(unsignedText, base)
	if !ok {
		return [32]byte{}, errors.New("invalid integer")
	}
	if negative {
		n.Neg(n)
	}

	if signed {
		limit := new(big.Int).Lsh(big.NewInt(1), uint(bits-1))
		min := new(big.Int).Neg(new(big.Int).Set(limit))
		max := new(big.Int).Sub(new(big.Int).Set(limit), big.NewInt(1))
		if n.Cmp(min) < 0 || n.Cmp(max) > 0 {
			return [32]byte{}, errors.New("signed integer out of range")
		}
		if n.Sign() < 0 {
			n.Add(n, new(big.Int).Lsh(big.NewInt(1), 256))
		}
	} else {
		if n.Sign() < 0 || n.BitLen() > bits {
			return [32]byte{}, errors.New("unsigned integer out of range")
		}
	}

	var out [32]byte
	n.FillBytes(out[:])
	return out, nil
}

func typedIntegerString(value any) (string, error) {
	switch value := value.(type) {
	case json.Number:
		text := value.String()
		if strings.ContainsAny(text, ".eE") {
			return "", errors.New("integer must not contain decimal or exponent")
		}
		return text, nil
	case string:
		return value, nil
	default:
		return "", errors.New("expected integer number or string")
	}
}

func validateType(typeName string, types map[string][]typedField) error {
	if element, _, ok := arrayType(typeName); ok {
		return validateType(element, types)
	}
	if isPrimitiveType(typeName) {
		return nil
	}
	if _, ok := types[typeName]; ok {
		return nil
	}
	return fmt.Errorf("%w: unsupported type %q", ErrInvalidTypedData, typeName)
}

func isPrimitiveType(typeName string) bool {
	if typeName == "string" || typeName == "bytes" || typeName == "bool" || typeName == "address" {
		return true
	}
	if _, ok := bytesN(typeName); ok {
		return true
	}
	_, _, ok := integerType(typeName)
	return ok
}

func bytesN(typeName string) (int, bool) {
	if !strings.HasPrefix(typeName, "bytes") || typeName == "bytes" {
		return 0, false
	}
	size, err := strconv.Atoi(strings.TrimPrefix(typeName, "bytes"))
	return size, err == nil && size >= 1 && size <= 32
}

func integerType(typeName string) (bits int, signed bool, ok bool) {
	prefix := "uint"
	if strings.HasPrefix(typeName, "int") && !strings.HasPrefix(typeName, "uint") {
		prefix = "int"
		signed = true
	} else if !strings.HasPrefix(typeName, "uint") {
		return 0, false, false
	}

	width := strings.TrimPrefix(typeName, prefix)
	if width == "" {
		return 0, false, false
	}
	bits, err := strconv.Atoi(width)
	if err != nil || bits < 8 || bits > 256 || bits%8 != 0 {
		return 0, false, false
	}
	return bits, signed, true
}

func arrayType(typeName string) (element string, fixedLength int, ok bool) {
	if !strings.HasSuffix(typeName, "]") {
		return "", 0, false
	}
	open := strings.LastIndexByte(typeName, '[')
	if open <= 0 {
		return "", 0, false
	}
	length := typeName[open+1 : len(typeName)-1]
	if length == "" {
		return typeName[:open], -1, true
	}
	n, err := strconv.Atoi(length)
	if err != nil || n <= 0 {
		return "", 0, false
	}
	return typeName[:open], n, true
}

func baseType(typeName string) string {
	for {
		element, _, ok := arrayType(typeName)
		if !ok {
			return typeName
		}
		typeName = element
	}
}
