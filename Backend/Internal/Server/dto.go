package server

type registerReq struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type meResp struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Photo string `json:"photo"`
}
