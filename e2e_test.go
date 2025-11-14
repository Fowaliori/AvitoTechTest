package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

const baseURL = "http://localhost:8080"

func TestCreateTeam(t *testing.T) {
	teamName := generateID("team")
	user1 := generateID("user")
	user2 := generateID("user")

	team := map[string]interface{}{
		"team_name": teamName,
		"members": []map[string]interface{}{
			{"user_id": user1, "username": "Alice", "is_active": true},
			{"user_id": user2, "username": "Bob", "is_active": true},
		},
	}

	resp, err := makeRequest("POST", baseURL+"/team/add", team)
	if err != nil {
		t.Fatalf("Ошибка запроса: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Ожидался статус 201, получен %d. Тело: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Ошибка декодирования ответа: %v", err)
	}

	if result["team"] == nil {
		t.Fatal("Команда не создана")
	}
}

func TestGetTeam(t *testing.T) {
	teamName := generateID("team")
	user1 := generateID("user")
	user2 := generateID("user")

	// Сначала создаем команду
	team := map[string]interface{}{
		"team_name": teamName,
		"members": []map[string]interface{}{
			{"user_id": user1, "username": "Alice", "is_active": true},
			{"user_id": user2, "username": "Bob", "is_active": true},
		},
	}

	_, err := makeRequest("POST", baseURL+"/team/add", team)
	if err != nil {
		t.Fatalf("Ошибка создания команды: %v", err)
	}

	// Получаем команду
	resp, err := makeRequest("GET", baseURL+"/team/get?team_name="+teamName, nil)
	if err != nil {
		t.Fatalf("Ошибка запроса: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Ожидался статус 200, получен %d. Тело: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Ошибка декодирования ответа: %v", err)
	}

	if result["team_name"] != teamName {
		t.Fatalf("Ожидалось имя команды %s, получено %v", teamName, result["team_name"])
	}
}

func TestCreatePRWithAutoReviewers(t *testing.T) {
	teamName := generateID("team")
	author := generateID("user")
	reviewer1 := generateID("user")
	reviewer2 := generateID("user")

	// Создаем команду с автором и ревьюверами
	team := map[string]interface{}{
		"team_name": teamName,
		"members": []map[string]interface{}{
			{"user_id": author, "username": "Author", "is_active": true},
			{"user_id": reviewer1, "username": "Reviewer1", "is_active": true},
			{"user_id": reviewer2, "username": "Reviewer2", "is_active": true},
		},
	}

	_, err := makeRequest("POST", baseURL+"/team/add", team)
	if err != nil {
		t.Fatalf("Ошибка создания команды: %v", err)
	}

	// Создаем PR
	prID := generateID("pr")
	prReq := map[string]interface{}{
		"pull_request_id":   prID,
		"pull_request_name": "Test PR",
		"author_id":         author,
	}

	resp, err := makeRequest("POST", baseURL+"/pullRequest/create", prReq)
	if err != nil {
		t.Fatalf("Ошибка запроса: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Ожидался статус 201, получен %d. Тело: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Ошибка декодирования ответа: %v", err)
	}

	pr := result["pull_request"].(map[string]interface{})
	reviewers := pr["assigned_reviewers"].([]interface{})

	if len(reviewers) == 0 {
		t.Fatal("Ревьюверы не назначены")
	}

	if len(reviewers) > 2 {
		t.Fatalf("Назначено больше 2 ревьюверов: %d", len(reviewers))
	}

	// Проверяем, что автор не назначен ревьювером
	for _, r := range reviewers {
		if r.(string) == author {
			t.Fatal("Автор не должен быть назначен ревьювером")
		}
	}
}

func TestMergePR(t *testing.T) {
	teamName := generateID("team")
	author := generateID("user")
	reviewer1 := generateID("user")

	// Создаем команду
	team := map[string]interface{}{
		"team_name": teamName,
		"members": []map[string]interface{}{
			{"user_id": author, "username": "Author", "is_active": true},
			{"user_id": reviewer1, "username": "Reviewer1", "is_active": true},
		},
	}

	respTeam, err := makeRequest("POST", baseURL+"/team/add", team)
	if err != nil {
		t.Fatalf("Ошибка создания команды: %v", err)
	}
	defer respTeam.Body.Close()

	if respTeam.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(respTeam.Body)
		t.Fatalf("Ожидался статус 201 при создании команды, получен %d. Тело: %s", respTeam.StatusCode, string(body))
	}

	// Небольшая задержка для гарантии сохранения в БД
	time.Sleep(100 * time.Millisecond)

	// Создаем PR
	prID := generateID("pr")
	prReq := map[string]interface{}{
		"pull_request_id":   prID,
		"pull_request_name": "Test PR",
		"author_id":         author,
	}

	respCreate, err := makeRequest("POST", baseURL+"/pullRequest/create", prReq)
	if err != nil {
		t.Fatalf("Ошибка создания PR: %v", err)
	}
	defer respCreate.Body.Close()

	if respCreate.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(respCreate.Body)
		t.Fatalf("Ожидался статус 201 при создании PR, получен %d. Тело: %s", respCreate.StatusCode, string(body))
	}

	// Merge PR
	mergeReq := map[string]interface{}{
		"pull_request_id": prID,
	}

	resp, err := makeRequest("POST", baseURL+"/pullRequest/merge", mergeReq)
	if err != nil {
		t.Fatalf("Ошибка запроса: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Ожидался статус 200, получен %d. Тело: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Ошибка декодирования ответа: %v", err)
	}

	pr := result["pull_request"].(map[string]interface{})
	if pr["status"] != "MERGED" {
		t.Fatalf("Ожидался статус MERGED, получен %v", pr["status"])
	}

	// Проверяем идемпотентность - повторный merge не должен вызывать ошибку
	resp2, err := makeRequest("POST", baseURL+"/pullRequest/merge", mergeReq)
	if err != nil {
		t.Fatalf("Ошибка повторного merge: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp2.Body)
		t.Fatalf("Повторный merge должен быть успешным, получен статус %d. Тело: %s", resp2.StatusCode, string(body))
	}
}

func TestReassignReviewer(t *testing.T) {
	teamName := generateID("team")
	author := generateID("user")
	oldReviewer := generateID("user")
	newReviewer := generateID("user")

	// Создаем команду
	team := map[string]interface{}{
		"team_name": teamName,
		"members": []map[string]interface{}{
			{"user_id": author, "username": "Author", "is_active": true},
			{"user_id": oldReviewer, "username": "OldReviewer", "is_active": true},
			{"user_id": newReviewer, "username": "NewReviewer", "is_active": true},
		},
	}

	_, err := makeRequest("POST", baseURL+"/team/add", team)
	if err != nil {
		t.Fatalf("Ошибка создания команды: %v", err)
	}

	// Создаем PR
	prID := generateID("pr")
	prReq := map[string]interface{}{
		"pull_request_id":   prID,
		"pull_request_name": "Test PR",
		"author_id":         author,
	}

	resp, err := makeRequest("POST", baseURL+"/pullRequest/create", prReq)
	if err != nil {
		t.Fatalf("Ошибка создания PR: %v", err)
	}
	defer resp.Body.Close()

	// Получаем список ревьюверов
	var createResult map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&createResult)
	pr := createResult["pull_request"].(map[string]interface{})
	reviewers := pr["assigned_reviewers"].([]interface{})

	if len(reviewers) == 0 {
		t.Fatal("Нет ревьюверов для переназначения")
	}

	oldReviewerID := reviewers[0].(string)

	// Переназначаем ревьювера
	reassignReq := map[string]interface{}{
		"pull_request_id": prID,
		"old_reviewer_id": oldReviewerID,
		"new_reviewer_id": newReviewer,
	}

	resp2, err := makeRequest("POST", baseURL+"/pullRequest/reassign", reassignReq)
	if err != nil {
		t.Fatalf("Ошибка запроса: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp2.Body)
		t.Fatalf("Ожидался статус 200, получен %d. Тело: %s", resp2.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp2.Body).Decode(&result); err != nil {
		t.Fatalf("Ошибка декодирования ответа: %v", err)
	}

	updatedPR := result["pull_request"].(map[string]interface{})
	updatedReviewers := updatedPR["assigned_reviewers"].([]interface{})

	found := false
	for _, r := range updatedReviewers {
		if r.(string) == newReviewer {
			found = true
			break
		}
	}

	if !found {
		t.Fatal("Новый ревьювер не найден в списке")
	}
}

func TestCannotReassignAfterMerge(t *testing.T) {
	teamName := generateID("team")
	author := generateID("user")
	oldReviewer := generateID("user")
	newReviewer := generateID("user")

	// Создаем команду
	team := map[string]interface{}{
		"team_name": teamName,
		"members": []map[string]interface{}{
			{"user_id": author, "username": "Author", "is_active": true},
			{"user_id": oldReviewer, "username": "OldReviewer", "is_active": true},
			{"user_id": newReviewer, "username": "NewReviewer", "is_active": true},
		},
	}

	_, err := makeRequest("POST", baseURL+"/team/add", team)
	if err != nil {
		t.Fatalf("Ошибка создания команды: %v", err)
	}

	// Создаем PR
	prID := generateID("pr")
	prReq := map[string]interface{}{
		"pull_request_id":   prID,
		"pull_request_name": "Test PR",
		"author_id":         author,
	}

	resp, err := makeRequest("POST", baseURL+"/pullRequest/create", prReq)
	if err != nil {
		t.Fatalf("Ошибка создания PR: %v", err)
	}
	defer resp.Body.Close()

	// Получаем список ревьюверов
	var createResult map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&createResult)
	pr := createResult["pull_request"].(map[string]interface{})
	reviewers := pr["assigned_reviewers"].([]interface{})

	if len(reviewers) == 0 {
		t.Fatal("Нет ревьюверов")
	}

	oldReviewerID := reviewers[0].(string)

	// Merge PR
	mergeReq := map[string]interface{}{
		"pull_request_id": prID,
	}

	_, err = makeRequest("POST", baseURL+"/pullRequest/merge", mergeReq)
	if err != nil {
		t.Fatalf("Ошибка merge: %v", err)
	}

	// Пытаемся переназначить после merge
	reassignReq := map[string]interface{}{
		"pull_request_id": prID,
		"old_reviewer_id": oldReviewerID,
		"new_reviewer_id": newReviewer,
	}

	resp2, err := makeRequest("POST", baseURL+"/pullRequest/reassign", reassignReq)
	if err != nil {
		t.Fatalf("Ошибка запроса: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode == http.StatusOK {
		t.Fatal("Переназначение после merge должно быть запрещено")
	}

	var errorResult map[string]interface{}
	json.NewDecoder(resp2.Body).Decode(&errorResult)
	errorObj := errorResult["error"].(map[string]interface{})
	if errorObj["code"] != "PR_MERGED" {
		t.Fatalf("Ожидалась ошибка PR_MERGED, получена: %v", errorObj["code"])
	}
}

func TestInactiveUsersNotAssigned(t *testing.T) {
	teamName := generateID("team")
	author := generateID("user")
	inactiveUser := generateID("user")
	activeUser := generateID("user")

	// Создаем команду с неактивным пользователем
	team := map[string]interface{}{
		"team_name": teamName,
		"members": []map[string]interface{}{
			{"user_id": author, "username": "Author", "is_active": true},
			{"user_id": inactiveUser, "username": "Inactive", "is_active": false},
			{"user_id": activeUser, "username": "Active", "is_active": true},
		},
	}

	_, err := makeRequest("POST", baseURL+"/team/add", team)
	if err != nil {
		t.Fatalf("Ошибка создания команды: %v", err)
	}

	// Создаем PR
	prID := generateID("pr")
	prReq := map[string]interface{}{
		"pull_request_id":   prID,
		"pull_request_name": "Test PR",
		"author_id":         author,
	}

	resp, err := makeRequest("POST", baseURL+"/pullRequest/create", prReq)
	if err != nil {
		t.Fatalf("Ошибка запроса: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Ожидался статус 201, получен %d. Тело: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Ошибка декодирования ответа: %v", err)
	}

	pr := result["pull_request"].(map[string]interface{})
	reviewersRaw := pr["assigned_reviewers"]
	if reviewersRaw == nil {
		t.Fatal("Ревьюверы должны быть назначены")
	}

	reviewers := reviewersRaw.([]interface{})

	// Проверяем, что неактивный пользователь не назначен
	for _, r := range reviewers {
		if r.(string) == inactiveUser {
			t.Fatal("Неактивный пользователь не должен быть назначен ревьювером")
		}
	}

	// Проверяем, что активный пользователь назначен
	foundActive := false
	for _, r := range reviewers {
		if r.(string) == activeUser {
			foundActive = true
			break
		}
	}
	if !foundActive && len(reviewers) > 0 {
		t.Fatal("Активный пользователь должен быть назначен ревьювером")
	}
}

func TestGetUserPullRequests(t *testing.T) {
	teamName := generateID("team")
	author := generateID("user")
	reviewer := generateID("user")

	// Создаем команду
	team := map[string]interface{}{
		"team_name": teamName,
		"members": []map[string]interface{}{
			{"user_id": author, "username": "Author", "is_active": true},
			{"user_id": reviewer, "username": "Reviewer", "is_active": true},
		},
	}

	_, err := makeRequest("POST", baseURL+"/team/add", team)
	if err != nil {
		t.Fatalf("Ошибка создания команды: %v", err)
	}

	// Создаем PR
	prID := generateID("pr")
	prReq := map[string]interface{}{
		"pull_request_id":   prID,
		"pull_request_name": "Test PR",
		"author_id":         author,
	}

	_, err = makeRequest("POST", baseURL+"/pullRequest/create", prReq)
	if err != nil {
		t.Fatalf("Ошибка создания PR: %v", err)
	}

	// Получаем PR'ы ревьювера
	resp, err := makeRequest("GET", baseURL+"/users/getReview?user_id="+reviewer, nil)
	if err != nil {
		t.Fatalf("Ошибка запроса: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Ожидался статус 200, получен %d. Тело: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Ошибка декодирования ответа: %v", err)
	}

	if result["user_id"] != reviewer {
		t.Fatalf("Ожидался user_id %s, получен %v", reviewer, result["user_id"])
	}

	prsRaw, ok := result["pull_requests"]
	if !ok || prsRaw == nil {
		t.Fatal("Ревьювер должен иметь назначенные PR")
	}

	prs := prsRaw.([]interface{})
	if len(prs) == 0 {
		t.Fatal("Ревьювер должен иметь назначенные PR")
	}
}

func TestSetUserActive(t *testing.T) {
	teamName := generateID("team")
	userID := generateID("user")

	// Создаем команду
	team := map[string]interface{}{
		"team_name": teamName,
		"members": []map[string]interface{}{
			{"user_id": userID, "username": "TestUser", "is_active": true},
		},
	}

	_, err := makeRequest("POST", baseURL+"/team/add", team)
	if err != nil {
		t.Fatalf("Ошибка создания команды: %v", err)
	}

	// Деактивируем пользователя
	setActiveReq := map[string]interface{}{
		"user_id":   userID,
		"is_active": false,
	}

	resp, err := makeRequest("POST", baseURL+"/users/setIsActive", setActiveReq)
	if err != nil {
		t.Fatalf("Ошибка запроса: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Ожидался статус 200, получен %d. Тело: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Ошибка декодирования ответа: %v", err)
	}

	user := result["user"].(map[string]interface{})
	if user["is_active"] != false {
		t.Fatalf("Ожидалось is_active=false, получено %v", user["is_active"])
	}
}

// Генерируем уникальный ID для тестов
func generateID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// Helper для HTTP запросов
func makeRequest(method, url string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	return client.Do(req)
}
