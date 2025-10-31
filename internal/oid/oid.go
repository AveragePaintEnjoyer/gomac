package oid

import (
	"encoding/json"
	"os"
)

var (
	OperState  map[int]string
	IntTypeNum map[int]string
)

func Load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var cfg struct {
		OperState  map[int]string `json:"oper_state"`
		IntTypeNum map[int]string `json:"int_type_num"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}

	OperState = cfg.OperState
	IntTypeNum = cfg.IntTypeNum
	return nil
}
