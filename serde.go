package hypercache

import "github.com/vmihailenco/msgpack/v5"

type serde interface {
	serialize(interface{}) ([]byte, error)
	deserialize(buff []byte, value interface{}) error
}

type defaultSerde struct {
}

func (cd defaultSerde) serialize(value interface{}) ([]byte, error) {
	switch value := value.(type) {
	case nil:
		return nil, nil
	case []byte:
		return value, nil
	case string:
		return []byte(value), nil
	}

	b, err := msgpack.Marshal(value)
	if err != nil {
		return nil, err
	}
	println(b)

	// TODO: Maybe compress the value?
	return b, nil
}

func (cd *defaultSerde) deserialize(buff []byte, value interface{}) error {
	if buff == nil {
		return nil
	}
	if len(buff) == 0 {
		return nil
	}

	switch value := value.(type) {
	case nil:
		return nil
	case *[]byte:
		clone := make([]byte, len(buff))
		copy(clone, buff)
		*value = clone
		return nil
	case *string:
		*value = string(buff)
		return nil
	}

	return msgpack.Unmarshal(buff, &value)
}
