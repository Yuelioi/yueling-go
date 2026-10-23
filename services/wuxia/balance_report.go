package wuxia

import (
	_ "embed"
	"encoding/json"
)

//go:embed equipment_balance.json
var equipmentBalanceReport []byte

func EquipmentBalanceReport() json.RawMessage {
	return append(json.RawMessage(nil), equipmentBalanceReport...)
}
