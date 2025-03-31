package types

type MsgResponse struct {
	JSONRPC string `json:"jsonrpc" validate:"required,eq=2.0"`
	ID      int    `json:"id" validate:"required"`
	Result  Result `json:"result" validate:"required"`
}

type Result struct {
	Query  string `json:"query" validate:"required"`
	Data   Data   `json:"data" validate:"required"`
	Events Events `json:"events" validate:"required"`
}

type Data struct {
	Type  string `json:"type" validate:"required"`
	Value struct {
		TxResult TxResult `json:"TxResult" validate:"required"`
	} `json:"value" validate:"required"`
}

type TxResult struct {
	Height string `json:"height" validate:"required,numeric"`
	Tx     string `json:"tx" validate:"required"`
	Result struct {
		Data      string  `json:"data" validate:"required"`
		GasWanted string  `json:"gas_wanted" validate:"required,numeric"`
		GasUsed   string  `json:"gas_used" validate:"required,numeric"`
		Events    []Event `json:"events" validate:"required,dive"`
	} `json:"result" validate:"required"`
}

type Event struct {
	Type       string      `json:"type" validate:"required"`
	Attributes []Attribute `json:"attributes" validate:"required,dive"`
}

type Attribute struct {
	Key   string `json:"key" validate:"required"`
	Value string `json:"value" validate:"required"`
	Index bool   `json:"index"`
}
type Events map[string][]string
