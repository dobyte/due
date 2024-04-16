package client

type Call struct {
	data chan []byte
}

func (c *Call) Done() <-chan []byte {
	return c.data
}
