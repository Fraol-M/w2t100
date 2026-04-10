package integration_test

import (
	"fmt"
	"net/http"
	"testing"

	"propertyops/backend/internal/common"
)

// TestNotifications_SendAndList verifies the basic send → list → mark-read → unread-count flow.
func TestNotifications_SendAndList(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	// PM sends a notification; recipient is a tenant.
	_, pmPw := createTestUser(t, db, "notif_pm", common.RolePropertyManager)
	recipientUser, recipientPw := createTestUser(t, db, "notif_tenant", common.RoleTenant)

	pmToken := loginUser(t, router, "notif_pm", pmPw)
	recipientToken := loginUser(t, router, "notif_tenant", recipientPw)

	// Send a direct notification.
	w := makeRequest(t, router, http.MethodPost, "/api/v1/notifications/send", pmToken, map[string]interface{}{
		"recipient_id": recipientUser.ID,
		"subject":      "Maintenance scheduled",
		"body":         "A plumber will visit your unit tomorrow between 9 am and 12 pm.",
		"category":     "Maintenance",
	})
	assertStatus(t, w, http.StatusCreated)

	var sendResp struct {
		Data struct {
			ID          uint64 `json:"id"`
			Subject     string `json:"subject"`
			RecipientID uint64 `json:"recipient_id"`
			IsRead      bool   `json:"is_read"`
		} `json:"data"`
	}
	parseResponse(t, w, &sendResp)
	notifID := sendResp.Data.ID

	if notifID == 0 {
		t.Fatalf("expected non-zero notification ID")
	}
	if sendResp.Data.RecipientID != recipientUser.ID {
		t.Errorf("expected recipient_id=%d, got %d", recipientUser.ID, sendResp.Data.RecipientID)
	}
	if sendResp.Data.IsRead {
		t.Errorf("new notification should not be marked read")
	}

	// Recipient lists their notifications — should contain the new one.
	w = makeRequest(t, router, http.MethodGet, "/api/v1/notifications", recipientToken, nil)
	assertStatus(t, w, http.StatusOK)

	var listResp struct {
		Data []struct {
			ID      uint64 `json:"id"`
			Subject string `json:"subject"`
			IsRead  bool   `json:"is_read"`
		} `json:"data"`
	}
	parseResponse(t, w, &listResp)

	found := false
	for _, n := range listResp.Data {
		if n.ID == notifID {
			found = true
			if n.IsRead {
				t.Errorf("notification %d should not be read before MarkRead", notifID)
			}
		}
	}
	if !found {
		t.Errorf("expected notification %d in list, not found", notifID)
	}

	// Unread count before marking read.
	w = makeRequest(t, router, http.MethodGet, "/api/v1/notifications/unread-count", recipientToken, nil)
	assertStatus(t, w, http.StatusOK)

	var countResp struct {
		Data struct {
			Count int64 `json:"count"`
		} `json:"data"`
	}
	parseResponse(t, w, &countResp)
	if countResp.Data.Count < 1 {
		t.Errorf("expected unread count >= 1, got %d", countResp.Data.Count)
	}

	// Mark the notification as read.
	w = makeRequest(t, router, http.MethodPatch,
		fmt.Sprintf("/api/v1/notifications/%d/read", notifID), recipientToken, nil)
	assertStatus(t, w, http.StatusNoContent)

	// After marking read, GET /:id must show is_read=true.
	w = makeRequest(t, router, http.MethodGet,
		fmt.Sprintf("/api/v1/notifications/%d", notifID), recipientToken, nil)
	assertStatus(t, w, http.StatusOK)

	var getResp struct {
		Data struct {
			ID     uint64 `json:"id"`
			IsRead bool   `json:"is_read"`
		} `json:"data"`
	}
	parseResponse(t, w, &getResp)
	if !getResp.Data.IsRead {
		t.Errorf("notification should be marked read after PATCH /:id/read")
	}
}

// TestNotifications_Send_OnlyPMAndAdmin verifies that a Tenant cannot send
// notifications via POST /notifications/send.
func TestNotifications_Send_OnlyPMAndAdmin(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	recipientUser, _ := createTestUser(t, db, "nsend_recipient", common.RoleTenant)
	_, tenantPw := createTestUser(t, db, "nsend_sender", common.RoleTenant)
	tenantToken := loginUser(t, router, "nsend_sender", tenantPw)

	w := makeRequest(t, router, http.MethodPost, "/api/v1/notifications/send", tenantToken, map[string]interface{}{
		"recipient_id": recipientUser.ID,
		"subject":      "Hello",
		"body":         "This tenant is trying to send a notification to another user.",
	})
	if w.Code != http.StatusForbidden {
		t.Errorf("Tenant should not be able to send notifications, got %d; body: %s", w.Code, w.Body.String())
	}
}

// TestNotifications_MarkRead_WrongUser verifies that one user cannot mark
// another user's notification as read.
func TestNotifications_MarkRead_WrongUser(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	_, pmPw := createTestUser(t, db, "mr_pm", common.RolePropertyManager)
	recipientUser, recipientPw := createTestUser(t, db, "mr_recipient", common.RoleTenant)
	_, otherPw := createTestUser(t, db, "mr_other", common.RoleTenant)

	pmToken := loginUser(t, router, "mr_pm", pmPw)
	_ = loginUser(t, router, "mr_recipient", recipientPw)
	otherToken := loginUser(t, router, "mr_other", otherPw)

	// PM sends to recipient.
	w := makeRequest(t, router, http.MethodPost, "/api/v1/notifications/send", pmToken, map[string]interface{}{
		"recipient_id": recipientUser.ID,
		"subject":      "Private notification",
		"body":         "This notification belongs only to the designated recipient user.",
	})
	assertStatus(t, w, http.StatusCreated)
	var resp struct {
		Data struct{ ID uint64 `json:"id"` } `json:"data"`
	}
	parseResponse(t, w, &resp)
	notifID := resp.Data.ID

	// Another tenant tries to mark it read — must fail (404 or 403).
	w = makeRequest(t, router, http.MethodPatch,
		fmt.Sprintf("/api/v1/notifications/%d/read", notifID), otherToken, nil)
	if w.Code == http.StatusNoContent || w.Code == http.StatusOK {
		t.Errorf("another user should not be able to mark someone else's notification read; got %d", w.Code)
	}
}

// TestNotifications_Unauthenticated verifies all notification endpoints return 401 without a token.
func TestNotifications_Unauthenticated(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/notifications"},
		{http.MethodGet, "/api/v1/notifications/unread-count"},
		{http.MethodGet, "/api/v1/notifications/1"},
		{http.MethodPatch, "/api/v1/notifications/1/read"},
		{http.MethodPost, "/api/v1/notifications/send"},
		{http.MethodPost, "/api/v1/notifications/threads"},
		{http.MethodGet, "/api/v1/notifications/threads"},
		{http.MethodGet, "/api/v1/notifications/threads/1"},
		{http.MethodPost, "/api/v1/notifications/threads/1/messages"},
		{http.MethodGet, "/api/v1/notifications/threads/1/messages"},
	}

	for _, ep := range endpoints {
		ep := ep
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			w := makeRequest(t, router, ep.method, ep.path, "", nil)
			if w.Code != http.StatusUnauthorized {
				t.Errorf("expected 401 without token, got %d; body: %s", w.Code, w.Body.String())
			}
		})
	}
}

// TestThreads_CreateAndMessage exercises the full thread lifecycle:
// create → add message → list messages → add participant → list participants.
func TestThreads_CreateAndMessage(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	_, user1Pw := createTestUser(t, db, "thr_user1", common.RoleTenant)
	user2, user2Pw := createTestUser(t, db, "thr_user2", common.RoleTenant)

	token1 := loginUser(t, router, "thr_user1", user1Pw)
	token2 := loginUser(t, router, "thr_user2", user2Pw)

	// Create a thread.
	w := makeRequest(t, router, http.MethodPost, "/api/v1/notifications/threads", token1,
		map[string]interface{}{
			"subject": "Lease renewal discussion for next year",
		})
	assertStatus(t, w, http.StatusCreated)

	var createResp struct {
		Data struct {
			ID       uint64 `json:"id"`
			Subject  string `json:"subject"`
			IsClosed bool   `json:"is_closed"`
		} `json:"data"`
	}
	parseResponse(t, w, &createResp)
	threadID := createResp.Data.ID

	if threadID == 0 {
		t.Fatalf("expected non-zero thread ID")
	}
	if createResp.Data.Subject != "Lease renewal discussion for next year" {
		t.Errorf("expected subject=%q, got %q", "Lease renewal discussion for next year", createResp.Data.Subject)
	}
	if createResp.Data.IsClosed {
		t.Errorf("new thread should not be closed")
	}

	// Get the thread.
	w = makeRequest(t, router, http.MethodGet,
		fmt.Sprintf("/api/v1/notifications/threads/%d", threadID), token1, nil)
	assertStatus(t, w, http.StatusOK)

	// Add a message to the thread.
	w = makeRequest(t, router, http.MethodPost,
		fmt.Sprintf("/api/v1/notifications/threads/%d/messages", threadID), token1,
		map[string]string{"body": "I would like to renew my lease for another 12 months please."})
	assertStatus(t, w, http.StatusCreated)

	var msgResp struct {
		Data struct {
			ID       uint64 `json:"id"`
			ThreadID uint64 `json:"thread_id"`
			SenderID uint64 `json:"sender_id"`
			Body     string `json:"body"`
		} `json:"data"`
	}
	parseResponse(t, w, &msgResp)
	if msgResp.Data.ID == 0 {
		t.Fatalf("expected non-zero message ID")
	}
	if msgResp.Data.ThreadID != threadID {
		t.Errorf("expected thread_id=%d, got %d", threadID, msgResp.Data.ThreadID)
	}

	// List messages.
	w = makeRequest(t, router, http.MethodGet,
		fmt.Sprintf("/api/v1/notifications/threads/%d/messages", threadID), token1, nil)
	assertStatus(t, w, http.StatusOK)

	var listMsgs struct {
		Data []struct {
			ID uint64 `json:"id"`
		} `json:"data"`
	}
	parseResponse(t, w, &listMsgs)
	if len(listMsgs.Data) == 0 {
		t.Errorf("expected at least one message in thread")
	}

	// Add participant (user2).
	w = makeRequest(t, router, http.MethodPost,
		fmt.Sprintf("/api/v1/notifications/threads/%d/participants", threadID), token1,
		map[string]interface{}{"user_id": user2.ID})
	assertStatus(t, w, http.StatusCreated)

	// List participants.
	w = makeRequest(t, router, http.MethodGet,
		fmt.Sprintf("/api/v1/notifications/threads/%d/participants", threadID), token1, nil)
	assertStatus(t, w, http.StatusOK)

	var listParts struct {
		Data []struct {
			UserID uint64 `json:"user_id"`
		} `json:"data"`
	}
	parseResponse(t, w, &listParts)
	found := false
	for _, p := range listParts.Data {
		if p.UserID == user2.ID {
			found = true
		}
	}
	if !found {
		t.Errorf("expected user2 (id=%d) to be in participants after AddParticipant", user2.ID)
	}

	// Participant (user2) can also add a message.
	w = makeRequest(t, router, http.MethodPost,
		fmt.Sprintf("/api/v1/notifications/threads/%d/messages", threadID), token2,
		map[string]string{"body": "Happy to proceed with the renewal on the same terms as before."})
	assertStatus(t, w, http.StatusCreated)
}

// TestThreads_ListOwn verifies that listing threads returns only threads the
// user created or participates in, and that counts are non-negative.
func TestThreads_ListOwn(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	_, u1Pw := createTestUser(t, db, "tlist_u1", common.RoleTenant)
	_, u2Pw := createTestUser(t, db, "tlist_u2", common.RoleTenant)

	token1 := loginUser(t, router, "tlist_u1", u1Pw)
	token2 := loginUser(t, router, "tlist_u2", u2Pw)

	// User1 creates two threads.
	for i := 0; i < 2; i++ {
		makeRequest(t, router, http.MethodPost, "/api/v1/notifications/threads", token1,
			map[string]interface{}{
				"subject": fmt.Sprintf("Thread subject number %d for listing test", i+1),
			})
	}

	// User1 sees their threads.
	w := makeRequest(t, router, http.MethodGet, "/api/v1/notifications/threads", token1, nil)
	assertStatus(t, w, http.StatusOK)

	var resp1 struct {
		Data []struct{ ID uint64 `json:"id"` } `json:"data"`
	}
	parseResponse(t, w, &resp1)
	if len(resp1.Data) < 2 {
		t.Errorf("user1 should see at least 2 threads, got %d", len(resp1.Data))
	}

	// User2 sees 0 threads (they own none, are in none).
	w = makeRequest(t, router, http.MethodGet, "/api/v1/notifications/threads", token2, nil)
	assertStatus(t, w, http.StatusOK)

	var resp2 struct {
		Data []struct{ ID uint64 `json:"id"` } `json:"data"`
	}
	parseResponse(t, w, &resp2)
	if len(resp2.Data) != 0 {
		t.Errorf("user2 should see 0 threads before being added, got %d", len(resp2.Data))
	}
}

// TestThreads_NonParticipantBlocked verifies that a user who is not a participant
// of a thread cannot read its messages.
func TestThreads_NonParticipantBlocked(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	_, ownerPw := createTestUser(t, db, "tperm_owner", common.RoleTenant)
	_, strangerPw := createTestUser(t, db, "tperm_stranger", common.RoleTenant)

	ownerToken := loginUser(t, router, "tperm_owner", ownerPw)
	strangerToken := loginUser(t, router, "tperm_stranger", strangerPw)

	// Owner creates a thread.
	w := makeRequest(t, router, http.MethodPost, "/api/v1/notifications/threads", ownerToken,
		map[string]interface{}{"subject": "Private thread not visible to strangers at all"})
	assertStatus(t, w, http.StatusCreated)
	var resp struct {
		Data struct{ ID uint64 `json:"id"` } `json:"data"`
	}
	parseResponse(t, w, &resp)
	threadID := resp.Data.ID

	// Stranger tries to get the thread — must fail (403 or 404).
	w = makeRequest(t, router, http.MethodGet,
		fmt.Sprintf("/api/v1/notifications/threads/%d", threadID), strangerToken, nil)
	if w.Code == http.StatusOK {
		t.Errorf("non-participant should not be able to GET thread, got 200; body: %s", w.Body.String())
	}

	// Stranger tries to post a message — must fail.
	w = makeRequest(t, router, http.MethodPost,
		fmt.Sprintf("/api/v1/notifications/threads/%d/messages", threadID), strangerToken,
		map[string]string{"body": "Unauthorized message attempt from non-participant user."})
	if w.Code == http.StatusCreated || w.Code == http.StatusOK {
		t.Errorf("non-participant should not be able to post messages, got %d", w.Code)
	}
}

// TestNotifications_ScheduledFor verifies that a future scheduled_for timestamp
// creates the notification in Scheduled status.
func TestNotifications_ScheduledFor(t *testing.T) {
	db := setupTestDB(t)
	seedRoles(t, db)
	cfg := testConfig()
	router := newTestRouter(db, cfg)

	_, adminPw := createTestUser(t, db, "sched_admin", common.RoleSystemAdmin)
	recipientUser, _ := createTestUser(t, db, "sched_recipient", common.RoleTenant)

	adminToken := loginUser(t, router, "sched_admin", adminPw)

	// Send with a future scheduled_for timestamp.
	w := makeRequest(t, router, http.MethodPost, "/api/v1/notifications/send", adminToken, map[string]interface{}{
		"recipient_id":  recipientUser.ID,
		"subject":       "Scheduled reminder",
		"body":          "This is a scheduled notification that will be delivered at a future time.",
		"category":      "System",
		"scheduled_for": "2099-12-31T10:00:00Z",
	})
	assertStatus(t, w, http.StatusCreated)

	var resp struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	parseResponse(t, w, &resp)

	// Scheduled notifications should have status Scheduled (not Sent).
	if resp.Data.Status == "Sent" {
		t.Errorf("notification with future scheduled_for should not be Sent immediately; got %q", resp.Data.Status)
	}
}
