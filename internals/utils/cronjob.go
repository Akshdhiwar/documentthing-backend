package utils

import (
	"context"
	"fmt"
	"time"

	"github.com/Akshdhiwar/simpledocs-backend/internals/initializer"
)

func DeleteExpiredInvites() {
	// Calculate the time threshold (48 hours ago from now)
	thresholdTime := time.Now().Add(-48 * time.Hour)

	// Execute the SQL query to update expired invites
	_, err := initializer.DB.Exec(context.Background(), `
		 UPDATE invite
		 SET deleted_at = NOW(), is_revoked = true
		 WHERE invited_at < $1 AND is_accepted = false
	 `, thresholdTime)

	if err != nil {
		fmt.Sprintf("Failed to update expired invites: %s", err)
	}
}
