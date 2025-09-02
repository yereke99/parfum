package domain

import "github.com/google/uuid"

type Product struct {
	Id          uuid.UUID
	NameParfume string
	Sex         string
	Description string
	Price       int
	PhotoPath   string
}
