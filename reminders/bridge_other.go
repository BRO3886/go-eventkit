//go:build !darwin

package reminders

// New creates a new Reminders [Client] and requests reminders access.
//
// On non-darwin platforms, this always returns [ErrUnsupported].
func New() (*Client, error) {
	return nil, ErrUnsupported
}

// Lists returns all reminder lists across all accounts.
func (c *Client) Lists() ([]List, error) {
	return nil, ErrUnsupported
}

// Reminders returns reminders matching the given filter options.
func (c *Client) Reminders(opts ...ListOption) ([]Reminder, error) {
	return nil, ErrUnsupported
}

// Reminder returns a single reminder by ID or ID prefix.
func (c *Client) Reminder(id string) (*Reminder, error) {
	return nil, ErrUnsupported
}

// CreateReminder creates a new reminder and returns it with its assigned ID.
func (c *Client) CreateReminder(input CreateReminderInput) (*Reminder, error) {
	return nil, ErrUnsupported
}

// UpdateReminder updates an existing reminder and returns the updated version.
func (c *Client) UpdateReminder(id string, input UpdateReminderInput) (*Reminder, error) {
	return nil, ErrUnsupported
}

// DeleteReminder permanently deletes a reminder by ID.
func (c *Client) DeleteReminder(id string) error {
	return ErrUnsupported
}

// CompleteReminder marks a reminder as completed and returns the updated version.
func (c *Client) CompleteReminder(id string) (*Reminder, error) {
	return nil, ErrUnsupported
}

// UncompleteReminder marks a reminder as incomplete and returns the updated version.
func (c *Client) UncompleteReminder(id string) (*Reminder, error) {
	return nil, ErrUnsupported
}
