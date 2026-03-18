package interfaces

// GetInterface 描述具备主键读取能力的对象。
type GetInterface interface {
	GetId() uint
}
