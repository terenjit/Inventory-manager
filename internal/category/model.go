package category

type Category struct {
	ID   uint   `db:"id"   json:"id"`
	Name string `db:"name" json:"name"`
}

type CreateInput struct {
	Name string `json:"name" binding:"required,min=1,max=80"`
}

type UpdateInput struct {
	Name string `json:"name" binding:"required,min=1,max=80"`
}
