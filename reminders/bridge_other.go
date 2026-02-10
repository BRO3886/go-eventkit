//go:build !darwin

package reminders

// New creates a new Reminders client.
// Returns ErrUnsupported on non-darwin platforms.
func New() (*Client, error) {
	return nil, ErrUnsupported
}

// Lists returns all reminder lists.
func (c *Client) Lists() ([]List, error) {
	return nil, ErrUnsupported
}

// Reminders returns reminders matching the given options.
func (c *Client) Reminders(opts ...ListOption) ([]Reminder, error) {
	return nil, ErrUnsupported
}

// Reminder returns a single reminder by ID.
func (c *Client) Reminder(id string) (*Reminder, error) {
	return nil, ErrUnsupported
}

// CreateReminder creates a new reminder.
func (c *Client) CreateReminder(input CreateReminderInput) (*Reminder, error) {
	return nil, ErrUnsupported
}

// UpdateReminder updates an existing reminder.
func (c *Client) UpdateReminder(id string, input UpdateReminderInput) (*Reminder, error) {
	return nil, ErrUnsupported
}

// DeleteReminder deletes a reminder by ID.
func (c *Client) DeleteReminder(id string) error {
	return ErrUnsupported
}

// CompleteReminder marks a reminder as completed.
func (c *Client) CompleteReminder(id string) (*Reminder, error) {
	return nil, ErrUnsupported
}

// UncompleteReminder marks a reminder as incomplete.
func (c *Client) UncompleteReminder(id string) (*Reminder, error) {
	return nil, ErrUnsupported
}
