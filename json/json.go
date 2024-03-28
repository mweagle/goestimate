package json

import (
	"fmt"
	"strconv"
)

func Uint(key string, dict map[string]interface{}) uint64 {
	curVal, curValOk := dict[key]
	if !curValOk {
		curVal = 0
	}
	f64, f64OK := curVal.(float64)
	if f64OK {
		return uint64(f64)
	}
	return 0
}

func String(key string, dict map[string]interface{}) string {
	curVal, curValOk := dict[key]
	if !curValOk {
		curVal = ""
	}
	strVal, _ := curVal.(string)
	return strVal
}

func Boolean(key string, dict map[string]interface{}) bool {
	// By default, an empty string is false
	boolVal := false
	curVal, curValOk := dict[key]
	if !curValOk {
		curVal = ""
	}
	boolVal, _ = strconv.ParseBool(fmt.Sprintf("%v", curVal))
	return boolVal
}
