package types

type Extension interface {
	Name() string
	Run() error
}
