//go:build !darwin

package reminders

import "context"

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

// DeleteReminders permanently removes multiple reminders in a single bridge call.
func (c *Client) DeleteReminders(ids []string) map[string]error {
	result := make(map[string]error)
	for _, id := range ids {
		result[id] = ErrUnsupported
	}
	return result
}

// CreateList creates a new reminder list and returns it with its assigned ID.
func (c *Client) CreateList(input CreateListInput) (*List, error) { return nil, ErrUnsupported }

// UpdateList updates an existing reminder list and returns the updated version.
func (c *Client) UpdateList(id string, input UpdateListInput) (*List, error) {
	return nil, ErrUnsupported
}

// DeleteList permanently removes a reminder list and all its reminders.
func (c *Client) DeleteList(id string) error { return ErrUnsupported }

// CompleteReminder marks a reminder as completed and returns the updated version.
func (c *Client) CompleteReminder(id string) (*Reminder, error) {
	return nil, ErrUnsupported
}

// UncompleteReminder marks a reminder as incomplete and returns the updated version.
func (c *Client) UncompleteReminder(id string) (*Reminder, error) {
	return nil, ErrUnsupported
}

// WatchChanges returns a channel that receives a value whenever the
// EventKit reminders database changes.
//
// Returns [ErrUnsupported] on non-darwin platforms.
func (c *Client) WatchChanges(ctx context.Context) (<-chan struct{}, error) {
	return nil, ErrUnsupported
}
