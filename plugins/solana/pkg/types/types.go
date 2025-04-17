package types

type LogsNotification struct {
	Params struct {
		Result struct {
			Value struct {
				Logs []string `json:"logs"`
			} `json:"value"`
		} `json:"result"`
	} `json:"params"`
}

type LogsSubscribeParams struct {
	Mentions   []string    `json:"mentions,omitempty"`
	All        interface{} `json:"all,omitempty"`
	Commitment string      `json:"commitment"`
}

type LogsSubscribeRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}
