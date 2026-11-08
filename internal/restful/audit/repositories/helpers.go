package repositories

import "uuid"

func nullableUUID(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return id.String()
}

func nullableText(value string) any {
	if value == "" {
		return nil
	}
	return value
}
