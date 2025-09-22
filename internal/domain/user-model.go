package domain

// JustEntry represents a user registration in the just table
type JustEntry struct {
	Id             int64  `json:"id" db:"id"`
	UserId         int64  `json:"userID" db:"id_user"`
	UserName       string `json:"userName" db:"userName"`
	DateRegistered string `json:"dateRegistered" db:"dataRegistred"`
}
