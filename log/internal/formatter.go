package internal

type Formatter interface {
	Format(entity *Entity) Buffer
}
