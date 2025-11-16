//nolint:errcheck
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"pr-reviewer/internal/models"
	"testing"
)

const baseURL = "http://localhost:8080"

func TestCreateTeam(t *testing.T) {
	teamName := generateID("team")
	user1 := generateID("user")
	user2 := generateID("user")

	team := models.Team{
		TeamName: teamName,
		Members: []models.TeamMember{
			{UserId: user1, Username: "Alice", IsActive: true},
			{UserId: user2, Username: "Bob", IsActive: true},
		},
	}

	resp, err := makeRequest("POST", baseURL+"/team/add", team)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatal("wrong status")
	}
}

func TestGetTeam(t *testing.T) {
	teamName := generateID("team")
	user1 := generateID("user")
	user2 := generateID("user")

	team := models.Team{
		TeamName: teamName,
		Members: []models.TeamMember{
			{UserId: user1, Username: "Alice", IsActive: true},
			{UserId: user2, Username: "Bob", IsActive: true},
		},
	}

	_, err := makeRequest("POST", baseURL+"/team/add", team)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := makeRequest("GET", baseURL+"/team/get?team_name="+teamName, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatal("wrong status")
	}
}

func TestCreatePR(t *testing.T) {
	teamName := generateID("team")
	author := generateID("user")
	reviewer1 := generateID("user")
	reviewer2 := generateID("user")

	team := models.Team{
		TeamName: teamName,
		Members: []models.TeamMember{
			{UserId: author, Username: "Author", IsActive: true},
			{UserId: reviewer1, Username: "Reviewer1", IsActive: true},
			{UserId: reviewer2, Username: "Reviewer2", IsActive: true},
		},
	}

	_, err := makeRequest("POST", baseURL+"/team/add", team)
	if err != nil {
		t.Fatal(err)
	}

	prID := generateID("pr")
	prReq := models.PostPullRequestCreateJSONBody{
		PullRequestId:   prID,
		PullRequestName: "Test PR",
		AuthorId:        author,
	}

	resp, err := makeRequest("POST", baseURL+"/pullRequest/create", prReq)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatal("wrong status")
	}

	var result struct {
		PullRequest *models.PullRequest `json:"pull_request"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if len(result.PullRequest.AssignedReviewers) == 0 {
		t.Fatal("no reviewers")
	}
}

func TestMergePR(t *testing.T) {
	teamName := generateID("team")
	author := generateID("user")
	reviewer1 := generateID("user")

	team := models.Team{
		TeamName: teamName,
		Members: []models.TeamMember{
			{UserId: author, Username: "Author", IsActive: true},
			{UserId: reviewer1, Username: "Reviewer1", IsActive: true},
		},
	}

	_, err := makeRequest("POST", baseURL+"/team/add", team)
	if err != nil {
		t.Fatal(err)
	}

	prID := generateID("pr")
	prReq := models.PostPullRequestCreateJSONBody{
		PullRequestId:   prID,
		PullRequestName: "Test PR",
		AuthorId:        author,
	}

	_, err = makeRequest("POST", baseURL+"/pullRequest/create", prReq)
	if err != nil {
		t.Fatal(err)
	}

	mergeReq := models.PostPullRequestMergeJSONBody{
		PullRequestId: prID,
	}

	resp, err := makeRequest("POST", baseURL+"/pullRequest/merge", mergeReq)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatal("wrong status")
	}
}

func TestReassignReviewer(t *testing.T) {
	teamName := generateID("team")
	author := generateID("user")
	oldReviewer := generateID("user")
	anotherReviewer := generateID("user")
	anotherReviewer2 := generateID("user")

	team := models.Team{
		TeamName: teamName,
		Members: []models.TeamMember{
			{UserId: author, Username: "Author", IsActive: true},
			{UserId: oldReviewer, Username: "OldReviewer", IsActive: true},
			{UserId: anotherReviewer, Username: "AnotherReviewer", IsActive: true},
			{UserId: anotherReviewer2, Username: "AnotherReviewer2", IsActive: true},
		},
	}

	_, err := makeRequest("POST", baseURL+"/team/add", team)
	if err != nil {
		t.Fatal(err)
	}

	prID := generateID("pr")
	prReq := models.PostPullRequestCreateJSONBody{
		PullRequestId:   prID,
		PullRequestName: "Test PR",
		AuthorId:        author,
	}

	resp, err := makeRequest("POST", baseURL+"/pullRequest/create", prReq)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var createResult struct {
		PullRequest *models.PullRequest `json:"pull_request"`
	}
	json.NewDecoder(resp.Body).Decode(&createResult)

	oldReviewerID := createResult.PullRequest.AssignedReviewers[0]

	reassignReq := struct {
		PullRequestId string `json:"pull_request_id"`
		OldUserId     string `json:"old_user_id"`
	}{
		PullRequestId: prID,
		OldUserId:     oldReviewerID,
	}

	resp2, err := makeRequest("POST", baseURL+"/pullRequest/reassign", reassignReq)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Fatal("wrong status", resp2.Status)
	}
}

func TestCannotReassignAfterMerge(t *testing.T) {
	teamName := generateID("team")
	author := generateID("user")
	oldReviewer := generateID("user")
	newReviewer := generateID("user")

	team := models.Team{
		TeamName: teamName,
		Members: []models.TeamMember{
			{UserId: author, Username: "Author", IsActive: true},
			{UserId: oldReviewer, Username: "OldReviewer", IsActive: true},
			{UserId: newReviewer, Username: "NewReviewer", IsActive: true},
		},
	}

	_, err := makeRequest("POST", baseURL+"/team/add", team)
	if err != nil {
		t.Fatal(err)
	}

	prID := generateID("pr")
	prReq := models.PostPullRequestCreateJSONBody{
		PullRequestId:   prID,
		PullRequestName: "Test PR",
		AuthorId:        author,
	}

	resp, err := makeRequest("POST", baseURL+"/pullRequest/create", prReq)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var createResult struct {
		PullRequest *models.PullRequest `json:"pull_request"`
	}
	json.NewDecoder(resp.Body).Decode(&createResult)

	oldReviewerID := createResult.PullRequest.AssignedReviewers[0]

	mergeReq := models.PostPullRequestMergeJSONBody{
		PullRequestId: prID,
	}

	_, err = makeRequest("POST", baseURL+"/pullRequest/merge", mergeReq)
	if err != nil {
		t.Fatal(err)
	}

	reassignReq := struct {
		PullRequestId string `json:"pull_request_id"`
		OldUserId     string `json:"old_user_id"`
	}{
		PullRequestId: prID,
		OldUserId:     oldReviewerID,
	}

	resp2, err := makeRequest("POST", baseURL+"/pullRequest/reassign", reassignReq)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode == http.StatusOK {
		t.Fatal("should fail")
	}
}

func TestInactiveUsersNotAssigned(t *testing.T) {
	teamName := generateID("team")
	author := generateID("user")
	inactiveUser := generateID("user")
	activeUser := generateID("user")

	team := models.Team{
		TeamName: teamName,
		Members: []models.TeamMember{
			{UserId: author, Username: "Author", IsActive: true},
			{UserId: inactiveUser, Username: "Inactive", IsActive: false},
			{UserId: activeUser, Username: "Active", IsActive: true},
		},
	}

	_, err := makeRequest("POST", baseURL+"/team/add", team)
	if err != nil {
		t.Fatal(err)
	}

	prID := generateID("pr")
	prReq := models.PostPullRequestCreateJSONBody{
		PullRequestId:   prID,
		PullRequestName: "Test PR",
		AuthorId:        author,
	}

	resp, err := makeRequest("POST", baseURL+"/pullRequest/create", prReq)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var result struct {
		PullRequest *models.PullRequest `json:"pull_request"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	for _, r := range result.PullRequest.AssignedReviewers {
		if r == inactiveUser {
			t.Fatal("inactive user assigned")
		}
	}
}

func TestGetUserPullRequests(t *testing.T) {
	teamName := generateID("team")
	author := generateID("user")
	reviewer := generateID("user")

	team := models.Team{
		TeamName: teamName,
		Members: []models.TeamMember{
			{UserId: author, Username: "Author", IsActive: true},
			{UserId: reviewer, Username: "Reviewer", IsActive: true},
		},
	}

	_, err := makeRequest("POST", baseURL+"/team/add", team)
	if err != nil {
		t.Fatal(err)
	}

	prID := generateID("pr")
	prReq := models.PostPullRequestCreateJSONBody{
		PullRequestId:   prID,
		PullRequestName: "Test PR",
		AuthorId:        author,
	}

	_, err = makeRequest("POST", baseURL+"/pullRequest/create", prReq)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := makeRequest("GET", baseURL+"/users/getReview?user_id="+reviewer, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatal("wrong status")
	}
}

func TestSetUserActive(t *testing.T) {
	teamName := generateID("team")
	userID := generateID("user")

	team := models.Team{
		TeamName: teamName,
		Members: []models.TeamMember{
			{UserId: userID, Username: "TestUser", IsActive: true},
		},
	}

	_, err := makeRequest("POST", baseURL+"/team/add", team)
	if err != nil {
		t.Fatal(err)
	}

	setActiveReq := models.PostUsersSetIsActiveJSONBody{
		UserId:   userID,
		IsActive: false,
	}

	resp, err := makeRequest("POST", baseURL+"/users/setIsActive", setActiveReq)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatal("wrong status")
	}
}

func generateID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, rand.Int()) //nolint:gosec
}

func makeRequest(method, url string, body any) (*http.Response, error) {
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

	client := &http.Client{}
	return client.Do(req)
}
