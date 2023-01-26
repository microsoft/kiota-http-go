package internal

import (
	"time"

	"github.com/google/uuid"
	absser "github.com/microsoft/kiota-abstractions-go/serialization"
)

type MockParseNodeFactory struct {
}

func (e *MockParseNodeFactory) GetValidContentType() (string, error) {
	return "application/json", nil
}
func (e *MockParseNodeFactory) GetRootParseNode(contentType string, content []byte) (absser.ParseNode, error) {
	return &MockParseNode{}, nil
}

type MockParseNode struct {
}

func (e *MockParseNode) GetOnBeforeAssignFieldValues() absser.ParsableAction {
	//TODO implement me
	panic("implement me")
}

func (e *MockParseNode) SetOnBeforeAssignFieldValues(action absser.ParsableAction) error {
	//TODO implement me
	panic("implement me")
}

func (e *MockParseNode) GetOnAfterAssignFieldValues() absser.ParsableAction {
	//TODO implement me
	panic("implement me")
}

func (e *MockParseNode) SetOnAfterAssignFieldValues(action absser.ParsableAction) error {
	//TODO implement me
	panic("implement me")
}

func (*MockParseNode) GetRawValue() (interface{}, error) {
	return nil, nil
}

func (e *MockParseNode) GetChildNode(index string) (absser.ParseNode, error) {
	return nil, nil
}
func (e *MockParseNode) GetCollectionOfObjectValues(ctor absser.ParsableFactory) ([]absser.Parsable, error) {
	return nil, nil
}
func (e *MockParseNode) GetCollectionOfPrimitiveValues(targetType string) ([]interface{}, error) {
	return nil, nil
}
func (e *MockParseNode) GetCollectionOfEnumValues(parser absser.EnumFactory) ([]interface{}, error) {
	return nil, nil
}
func (e *MockParseNode) GetObjectValue(ctor absser.ParsableFactory) (absser.Parsable, error) {
	return &MockEntity{}, nil
}
func (e *MockParseNode) GetStringValue() (*string, error) {
	return nil, nil
}
func (e *MockParseNode) GetBoolValue() (*bool, error) {
	return nil, nil

}
func (e *MockParseNode) GetInt8Value() (*int8, error) {
	return nil, nil

}
func (e *MockParseNode) GetByteValue() (*byte, error) {
	return nil, nil

}
func (e *MockParseNode) GetFloat32Value() (*float32, error) {
	return nil, nil

}
func (e *MockParseNode) GetFloat64Value() (*float64, error) {
	return nil, nil

}
func (e *MockParseNode) GetInt32Value() (*int32, error) {
	return nil, nil

}
func (e *MockParseNode) GetInt64Value() (*int64, error) {
	return nil, nil

}
func (e *MockParseNode) GetTimeValue() (*time.Time, error) {
	return nil, nil

}
func (e *MockParseNode) GetISODurationValue() (*absser.ISODuration, error) {
	return nil, nil

}
func (e *MockParseNode) GetTimeOnlyValue() (*absser.TimeOnly, error) {
	return nil, nil

}
func (e *MockParseNode) GetDateOnlyValue() (*absser.DateOnly, error) {
	return nil, nil

}
func (e *MockParseNode) GetUUIDValue() (*uuid.UUID, error) {
	return nil, nil

}
func (e *MockParseNode) GetEnumValue(parser absser.EnumFactory) (interface{}, error) {
	return nil, nil

}
func (e *MockParseNode) GetByteArrayValue() ([]byte, error) {
	return nil, nil

}
