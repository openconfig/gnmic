// © 2026 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package formatters

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"

	"github.com/itchyny/gojq"
)

func CheckCondition(code *gojq.Code, event *EventMsg) (bool, error) {
	if code == nil {
		return true, nil
	}

	input, err := conditionInput(event)
	if err != nil {
		return false, err
	}
	result, ok := code.Run(input).Next()
	if !ok {
		return false, nil
	}
	if err, ok := result.(error); ok {
		return false, err
	}
	matched, ok := result.(bool)
	if !ok {
		return false, fmt.Errorf("unexpected condition return type: %T | %v", result, result)
	}
	return matched, nil
}

func conditionInput(event *EventMsg) (map[string]interface{}, error) {
	input := event.ToMap()
	if input == nil {
		return nil, nil
	}
	cloned, err := cloneConditionValue(input)
	if err != nil {
		return nil, err
	}
	return cloned.(map[string]interface{}), nil
}

// cloneConditionValue copies mutable containers and converts values that are
// not native gojq inputs while preserving encoding/json byte-slice semantics.
func cloneConditionValue(value interface{}) (interface{}, error) {
	switch value := value.(type) {
	case nil, bool, string, int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64:
		return value, nil
	case float64:
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return cloneConditionValueFallback(value)
		}
		return value, nil
	case float32:
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return cloneConditionValueFallback(value)
		}
		return value, nil
	case []byte:
		if value == nil {
			return nil, nil
		}
		return base64.StdEncoding.EncodeToString(value), nil
	case map[string]interface{}:
		if value == nil {
			return nil, nil
		}
		cloned := make(map[string]interface{}, len(value))
		for key, child := range value {
			var err error
			cloned[key], err = cloneConditionValue(child)
			if err != nil {
				return nil, err
			}
		}
		return cloned, nil
	case map[string]string:
		if value == nil {
			return nil, nil
		}
		cloned := make(map[string]interface{}, len(value))
		for key, child := range value {
			cloned[key] = child
		}
		return cloned, nil
	case []interface{}:
		if value == nil {
			return nil, nil
		}
		cloned := make([]interface{}, len(value))
		for index, child := range value {
			var err error
			cloned[index], err = cloneConditionValue(child)
			if err != nil {
				return nil, err
			}
		}
		return cloned, nil
	case []string:
		if value == nil {
			return nil, nil
		}
		cloned := make([]interface{}, len(value))
		for index, child := range value {
			cloned[index] = child
		}
		return cloned, nil
	default:
		return cloneConditionValueFallback(value)
	}
}

func cloneConditionValueFallback(value interface{}) (interface{}, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var cloned interface{}
	if err := json.Unmarshal(b, &cloned); err != nil {
		return nil, err
	}
	return cloned, nil
}
