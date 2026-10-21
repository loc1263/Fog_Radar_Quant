package yahoo

// newClientWith crea un cliente sobre un backend dado (solo tests).
func newClientWith(t *Traffic, b backend) *Client { return &Client{T: t, raw: b} }
