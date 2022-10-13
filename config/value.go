package config

type Value struct {
	val interface{}
}

func (v *Value) Map() map[string]interface{} {
	return nil
}

func (v *Value) Slice() []interface{} {
	return nil
}

func (v *Value) Scan(pointer interface{}, mapping ...map[string]string) error {
	return nil
}
