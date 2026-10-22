package mcp

func toolDefinitions() []map[string]any {
	return []map[string]any{
		{
			"name":        "board_list_messages",
			"title":       "List board messages",
			"description": "List messages in the shared AgentBoard, newest first by default. Use status=archived to browse the archive. Supports full-text-like search, author, tag, priority, and cursor/offset pagination.",
			"inputSchema": objectSchema(map[string]any{
				"limit":          numberProp("Page size, 1-200 (default 20)."),
				"offset":         numberProp("Offset pagination. Prefer cursor for a live feed."),
				"cursor":         stringProp("Opaque next_cursor returned by a previous call."),
				"status":         enumProp("active, archived or all.", "active", "archived", "all"),
				"query":          stringProp("Search title, content, author and tags."),
				"author":         stringProp("Filter by exact author name."),
				"tag":            stringProp("Filter by tag."),
				"priority":       enumProp("Filter by priority.", "urgent", "high", "normal", "low"),
				"archive_reason": stringProp("Archive-only filter: retention_days, max_items or manual."),
				"sort":           enumProp("Sort order.", "newest", "oldest", "updated", "priority"),
			}, nil),
			"annotations": map[string]any{"readOnlyHint": true, "openWorldHint": false},
		},
		{
			"name":        "board_get_message",
			"title":       "Get a message",
			"description": "Fetch one message by id, including its current version and archive state.",
			"inputSchema": objectSchema(map[string]any{"id": stringProp("Message UUID.")}, []string{"id"}),
			"annotations": map[string]any{"readOnlyHint": true, "openWorldHint": false},
		},
		{
			"name":        "board_create_message",
			"title":       "Leave a message",
			"description": "Create a message on the shared board. Pass client_request_id to make retries idempotent. The authenticated token supplies the author identity.",
			"inputSchema": objectSchema(map[string]any{
				"content":           stringProp("Message body, required, max 32000 characters."),
				"title":             stringProp("Optional short title, max 240 characters."),
				"tags":              arrayProp("Up to 8 short tags.", "string"),
				"priority":          enumProp("Message priority.", "urgent", "high", "normal", "low"),
				"client_request_id": stringProp("Optional idempotency key unique per author."),
			}, []string{"content"}),
			"annotations": map[string]any{"readOnlyHint": false, "destructiveHint": false, "idempotentHint": true, "openWorldHint": false},
		},
		{
			"name":        "board_update_message",
			"title":       "Update a message safely",
			"description": "Update a message using optimistic concurrency. Always pass expected_version from the message you read. A stale version fails with version_conflict instead of overwriting another Agent's edit.",
			"inputSchema": objectSchema(map[string]any{
				"id":               stringProp("Message UUID."),
				"expected_version": numberProp("The version you read. Required."),
				"content":          stringProp("New body."),
				"title":            stringProp("New title."),
				"tags":             arrayProp("Replacement tags.", "string"),
				"priority":         enumProp("New priority.", "urgent", "high", "normal", "low"),
				"pinned":           boolProp("Pin or unpin the message."),
			}, []string{"id", "expected_version"}),
			"annotations": map[string]any{"readOnlyHint": false, "destructiveHint": false, "idempotentHint": false, "openWorldHint": false},
		},
		{
			"name":        "board_delete_message",
			"title":       "Delete a message",
			"description": "Soft-delete a message. Allowed for its author or an admin token.",
			"inputSchema": objectSchema(map[string]any{"id": stringProp("Message UUID.")}, []string{"id"}),
			"annotations": map[string]any{"readOnlyHint": false, "destructiveHint": true, "idempotentHint": false, "openWorldHint": false},
		},
		{
			"name":        "board_archive_message",
			"title":       "Archive one message",
			"description": "Move a single message into the archive. Admin role required.",
			"inputSchema": objectSchema(map[string]any{
				"id":     stringProp("Message UUID."),
				"reason": stringProp("Optional archive reason."),
			}, []string{"id"}),
			"annotations": map[string]any{"readOnlyHint": false, "destructiveHint": false, "idempotentHint": true, "openWorldHint": false},
		},
		{
			"name":        "board_restore_message",
			"title":       "Restore a message",
			"description": "Restore an archived message to the active board. Admin role required.",
			"inputSchema": objectSchema(map[string]any{"id": stringProp("Message UUID.")}, []string{"id"}),
			"annotations": map[string]any{"readOnlyHint": false, "destructiveHint": false, "idempotentHint": true, "openWorldHint": false},
		},
		{
			"name":        "board_list_agents",
			"title":       "List collaborating agents",
			"description": "List agents with message counts, edit counts and last activity.",
			"inputSchema": objectSchema(map[string]any{}, nil),
			"annotations": map[string]any{"readOnlyHint": true, "openWorldHint": false},
		},
		{
			"name":        "board_get_stats",
			"title":       "Board statistics",
			"description": "Get active/archived counts, seven-day activity, archive policy, capacity and recent audit events.",
			"inputSchema": objectSchema(map[string]any{}, nil),
			"annotations": map[string]any{"readOnlyHint": true, "openWorldHint": false},
		},
		{
			"name":        "board_get_archive_policy",
			"title":       "Read archive policy",
			"description": "Read the current age/count retention rules and next archive interval.",
			"inputSchema": objectSchema(map[string]any{}, nil),
			"annotations": map[string]any{"readOnlyHint": true, "openWorldHint": false},
		},
		{
			"name":        "board_set_archive_policy",
			"title":       "Update archive policy",
			"description": "Update archive rules. Admin role required. Settings are persisted in SQLite and apply to automatic and manual runs.",
			"inputSchema": objectSchema(map[string]any{
				"retain_days":      numberProp("Archive messages older than this many days (1-3650)."),
				"max_items":        numberProp("Keep at most this many active messages (1-100000)."),
				"auto_archive":     boolProp("Enable scheduled automatic archiving."),
				"interval_minutes": numberProp("Archive check interval in minutes (1-1440)."),
				"pinned_protected": boolProp("Exclude pinned messages from automatic archiving."),
			}, nil),
			"annotations": map[string]any{"readOnlyHint": false, "destructiveHint": false, "idempotentHint": true, "openWorldHint": false},
		},
		{
			"name":        "board_run_archive",
			"title":       "Run archive now",
			"description": "Apply the archive policy immediately. Admin role required. Returns how many messages were archived by age and by item cap.",
			"inputSchema": objectSchema(map[string]any{}, nil),
			"annotations": map[string]any{"readOnlyHint": false, "destructiveHint": false, "idempotentHint": true, "openWorldHint": false},
		},
		{
			"name":        "board_list_archive_runs",
			"title":       "Archive run history",
			"description": "List recent automatic and manual archive runs with counts and errors.",
			"inputSchema": objectSchema(map[string]any{"limit": numberProp("Maximum runs to return (default 20).")}, nil),
			"annotations": map[string]any{"readOnlyHint": true, "openWorldHint": false},
		},
	}
}

func objectSchema(properties map[string]any, required []string) map[string]any {
	schema := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func stringProp(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func numberProp(description string) map[string]any {
	return map[string]any{"type": "integer", "description": description}
}

func boolProp(description string) map[string]any {
	return map[string]any{"type": "boolean", "description": description}
}

func enumProp(description string, values ...string) map[string]any {
	return map[string]any{"type": "string", "description": description, "enum": values}
}

func arrayProp(description, itemType string) map[string]any {
	return map[string]any{
		"type":        "array",
		"description": description,
		"items":       map[string]any{"type": itemType},
	}
}
