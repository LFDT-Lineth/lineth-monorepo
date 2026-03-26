package ifaces

// Item is a very generic interfaces that represents any component
// of a protocol. Can be a column, a coin, a query ...
type Item interface {
	Round() int
}
