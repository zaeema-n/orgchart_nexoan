package api

// AddSecretaryEntity is deprecated. Use AddPersonEntity with role=secretary.
func (c *Client) AddSecretaryEntity(transaction map[string]interface{}, entityCounters map[string]int) (int, error) {
	transaction["role"] = "secretary"
	return c.AddPersonEntity(transaction, entityCounters)
}

// TerminateSecretaryEntity is deprecated. Use TerminatePersonEntity.
func (c *Client) TerminateSecretaryEntity(transaction map[string]interface{}) error {
	transaction["role"] = "secretary"
	return c.TerminatePersonEntity(transaction)
}
