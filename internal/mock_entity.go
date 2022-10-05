package internal

import (
	absser "github.com/microsoft/kiota-abstractions-go/serialization"
)

type MockEntity struct {
}

type MockEntityAble interface {
	absser.Parsable
}

func (e *MockEntity) Serialize(writer absser.SerializationWriter) error {
	return nil
}
func (e *MockEntity) GetFieldDeserializers() map[string]func(absser.ParseNode) error {
	return make(map[string]func(absser.ParseNode) error)
}
func MockEntityFactory(parseNode absser.ParseNode) (absser.Parsable, error) {
	return &MockEntity{}, nil
}
