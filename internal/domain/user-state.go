package domain

type UserState struct {
	State         string `json:"state"`
	BroadCastType string `json:"broadcast_type"`
	Count         int    `json:"count"`
	Contact       string `json:"contact"`
	IsPaid        bool   `json:"is_paid"`
}
