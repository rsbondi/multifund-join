package multijoin

type JoinStartResponseData struct {
	Message string `json:"message"`
	Id      int    `json:"id"`
}

type JoinStartResponse struct {
	Response *JoinStartResponseData `json:"response"`
	Error    string                 `json:"error"`
}

type JoinStatusResponse struct {
	Tx    *[]byte `json:"tx"`
	Error string  `json:"error"`
}
