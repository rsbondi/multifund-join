package multijoin

type JoinStartResponseData struct {
	Message string `json:"message"`
	Id      int    `json:"id"`
	Pid     int    `json:"pid"`
}

type JoinStartResponse struct {
	Response *JoinStartResponseData `json:"response"`
	Error    string                 `json:"error"`
}

type JoinStatusResponse struct {
	Tx    *[]byte `json:"tx"`
	Error string  `json:"error"`
}

type TransactionSubmission struct {
	Tx  string `json:"tx"`
	Id  int    `json:"id"`
	Pid int    `json:"pid"`
}
