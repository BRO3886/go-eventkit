---
name: eventkit-mcp
description: Use macOS Calendar and Reminders through the go-eventkit MCP server.
---

# eventkit-mcp

Use the `eventkit` MCP server for read/write Calendar and Reminders work on macOS.

Prefer read tools before write tools so the target calendar, reminder list, event id, or reminder id is explicit. Calendar event queries must include a bounded date range unless using `event_today`, `event_upcoming`, or `event_search` defaults.

Write tools can change real user data. For tests, create a temporary calendar or reminder list, use it for all writes, and delete it during cleanup.

Useful tools:
- Calendars: `calendar_list`, `calendar_create`, `calendar_update`, `calendar_delete`
- Events: `event_list`, `event_today`, `event_upcoming`, `event_search`, `event_get`, `event_create`, `event_update`, `event_delete`, `event_export`, `event_import`
- Reminder lists: `reminder_list_list`, `reminder_list_create`, `reminder_list_update`, `reminder_list_delete`
- Reminders: `reminder_list`, `reminder_today`, `reminder_overdue`, `reminder_upcoming`, `reminder_search`, `reminder_stats`, `reminder_get`, `reminder_create`, `reminder_update`, `reminder_complete`, `reminder_uncomplete`, `reminder_delete`, `reminder_export`, `reminder_import`
- Limitations: `eventkit_limitations`

Natural language date strings are accepted anywhere a date string is documented, including `tomorrow 9am`, `next friday`, `in 3 hours`, `eow`, and RFC3339 timestamps.

If a tool returns an EventKit permission error, instruct the user to grant Calendar or Reminders access to the terminal or host app in System Settings > Privacy & Security.
