package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"flashcards/internal/content"
	"flashcards/internal/progress"
	"flashcards/internal/review"
)

func TestTopicHandlerRendersCreateAndEditPagesForAuthenticatedKid(t *testing.T) {
	contentService := content.NewService(t.TempDir())
	if err := contentService.SaveList("kid-1", content.List{Subject: "Frans", Period: "Hoofdstuk 4", Cards: []content.Card{{Front: "bonjour", Back: "goedendag"}}}); err != nil {
		t.Fatalf("save list: %v", err)
	}
	handler := NewTopicHandler(TopicDependencies{Sessions: kidSessions(t), Content: contentService})
	cookie, csrfToken := topicKidLogin(t, handler)
	for _, target := range []string{"/kid/topics/new", "/kid/topics/edit?" + url.Values{"subject": {"Frans"}, "period": {"Hoofdstuk 4"}}.Encode()} {
		request := httptest.NewRequest(http.MethodGet, target, nil)
		request.AddCookie(cookie)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Flashcard-editor") || !strings.Contains(response.Body.String(), csrfToken) {
			t.Fatalf("%s response = %d: %s", target, response.Code, response.Body.String())
		}
		if !strings.Contains(response.Body.String(), `href="/student/sessions/new"`) {
			t.Fatalf("%s does not link back to the series selection screen", target)
		}
	}
}

func TestSessionSelectionOffersEditingForSelectedList(t *testing.T) {
	contentService := content.NewService(t.TempDir())
	if err := contentService.SaveList("kid-1", content.List{Subject: "Frans", Period: "Hoofdstuk 4", Cards: []content.Card{{Front: "bonjour", Back: "goedendag"}}}); err != nil {
		t.Fatalf("save list: %v", err)
	}
	handler := NewTopicHandler(TopicDependencies{Sessions: kidSessions(t), Content: contentService, Progress: progress.NewStore(t.TempDir())})
	cookie, _ := topicKidLogin(t, handler)
	request := httptest.NewRequest(http.MethodGet, "/student/sessions/new", nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Serie bewerken of CSV importeren") || !strings.Contains(response.Body.String(), `data-edit-list`) {
		t.Fatalf("session selection response = %d: %s", response.Code, response.Body.String())
	}
}

func TestTopicHandlerRefusesCrossKidContentAccess(t *testing.T) {
	contentService := content.NewService(t.TempDir())
	if err := contentService.SaveList("kid-2", content.List{Subject: "Frans", Period: "Hoofdstuk 4"}); err != nil {
		t.Fatalf("save other kid list: %v", err)
	}
	handler := NewTopicHandler(TopicDependencies{Sessions: kidSessions(t), Content: contentService})
	cookie, _ := topicKidLogin(t, handler)
	request := httptest.NewRequest(http.MethodGet, "/kid/topics/edit?subject=Frans&period=Hoofdstuk+4", nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("cross-kid status = %d, want 404", response.Code)
	}
}

func topicKidLogin(t *testing.T, handler http.Handler) (*http.Cookie, string) {
	t.Helper()
	sessions := kidSessions(t)
	cookie, session, err := sessions.Login("mia", "112233")
	if err != nil {
		t.Fatalf("create kid session: %v", err)
	}
	return cookie, session.CSRFToken
}

func TestTopicHandlerSavesImportsAndDeletesPairsWithCSRF(t *testing.T) {
	contentService := content.NewService(t.TempDir())
	progressStore := progress.NewStore(t.TempDir())
	handler := NewTopicHandler(TopicDependencies{Sessions: kidSessions(t), Content: contentService, Progress: progressStore})
	cookie, csrfToken := topicKidLogin(t, handler)

	saveForm := url.Values{"csrf_token": {csrfToken}, "subject": {"Frans"}, "period": {"Hoofdstuk 4"}, "side_a_default_language": {"fr"}, "side_b_default_language": {"nl"}, "side_a": {"bonjour"}, "side_b": {"goedendag"}, "side_a_language": {""}, "side_b_language": {""}}
	saveRequest := httptest.NewRequest(http.MethodPost, "/kid/topics", strings.NewReader(saveForm.Encode()))
	saveRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	saveRequest.AddCookie(cookie)
	saveResponse := httptest.NewRecorder()
	handler.ServeHTTP(saveResponse, saveRequest)
	if saveResponse.Code != http.StatusSeeOther {
		t.Fatalf("save status = %d", saveResponse.Code)
	}
	list, err := contentService.LoadList("kid-1", "Frans", "Hoofdstuk 4")
	if err != nil || len(list.Cards) != 1 || list.Cards[0].ID == "" || list.SideADefaultLanguage != "fr" {
		t.Fatalf("saved list = %#v, err = %v", list, err)
	}

	importForm := url.Values{"csrf_token": {csrfToken}, "subject": {"Frans"}, "format": {"json"}, "data": {`[{"side_a":"merci","side_b":"dank je"}]`}}
	importRequest := httptest.NewRequest(http.MethodPost, "/kid/topics/import", strings.NewReader(importForm.Encode()))
	importRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	importRequest.AddCookie(cookie)
	importResponse := httptest.NewRecorder()
	handler.ServeHTTP(importResponse, importRequest)
	if importResponse.Code != http.StatusOK || !strings.Contains(importResponse.Body.String(), "merci") {
		t.Fatalf("import response = %d: %s", importResponse.Code, importResponse.Body.String())
	}

	csvImport := url.Values{"csrf_token": {csrfToken}, "subject": {"Frans"}, "format": {"csv"}, "data": {"front;back\nchat;kat"}}
	csvRequest := httptest.NewRequest(http.MethodPost, "/student/topics/import", strings.NewReader(csvImport.Encode()))
	csvRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	csvRequest.AddCookie(cookie)
	csvResponse := httptest.NewRecorder()
	handler.ServeHTTP(csvResponse, csvRequest)
	if csvResponse.Code != http.StatusOK || !strings.Contains(csvResponse.Body.String(), "chat") {
		t.Fatalf("student CSV import response = %d: %s", csvResponse.Code, csvResponse.Body.String())
	}

	deleteForm := url.Values{"csrf_token": {csrfToken}, "subject": {"Frans"}, "period": {"Hoofdstuk 4"}}
	deleteRequest := httptest.NewRequest(http.MethodPost, "/kid/topics/delete", strings.NewReader(deleteForm.Encode()))
	deleteRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	deleteRequest.AddCookie(cookie)
	deleteResponse := httptest.NewRecorder()
	handler.ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d", deleteResponse.Code)
	}
}

func TestTopicHandlerRefusesWriteWithoutCSRF(t *testing.T) {
	handler := NewTopicHandler(TopicDependencies{Sessions: kidSessions(t), Content: content.NewService(t.TempDir())})
	cookie, _ := topicKidLogin(t, handler)
	request := httptest.NewRequest(http.MethodPost, "/kid/topics", strings.NewReader("subject=Frans&period=Hoofdstuk+4"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF status = %d", response.Code)
	}
}

func TestTopicSaveResetsChangedPairDirectionsAndKeepsHistory(t *testing.T) {
	contentService := content.NewService(t.TempDir())
	progressStore := progress.NewStore(t.TempDir())
	if err := contentService.SaveList("kid-1", content.List{Subject: "Frans", Period: "Hoofdstuk 4", Cards: []content.Card{{ID: "pair-1", Front: "bonjour", Back: "goedendag"}}}); err != nil {
		t.Fatalf("save initial list: %v", err)
	}
	if err := progressStore.Save("kid-1", progress.KidProgress{CardState: map[string]progress.CardState{review.DirectionKey("pair-1", "a"): {}, review.DirectionKey("pair-1", "b"): {}}, Sessions: []progress.SessionSummary{{ID: "history"}}}); err != nil {
		t.Fatalf("save progress: %v", err)
	}
	handler := NewTopicHandler(TopicDependencies{Sessions: kidSessions(t), Content: contentService, Progress: progressStore})
	cookie, csrfToken := topicKidLogin(t, handler)
	form := url.Values{"csrf_token": {csrfToken}, "subject": {"Frans"}, "period": {"Hoofdstuk 4"}, "id": {"pair-1"}, "side_a": {"salut"}, "side_b": {"goedendag"}}
	request := httptest.NewRequest(http.MethodPost, "/kid/topics", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("save status = %d", response.Code)
	}
	kidProgress, err := progressStore.Load("kid-1")
	if err != nil || len(kidProgress.CardState) != 0 || len(kidProgress.Sessions) != 1 {
		t.Fatalf("progress = %#v, err = %v", kidProgress, err)
	}
}

func TestTopicDeleteResetsPairDirections(t *testing.T) {
	contentService := content.NewService(t.TempDir())
	progressStore := progress.NewStore(t.TempDir())
	if err := contentService.SaveList("kid-1", content.List{Subject: "Frans", Period: "Hoofdstuk 4", Cards: []content.Card{{ID: "pair-1", Front: "bonjour", Back: "goedendag"}}}); err != nil {
		t.Fatalf("save list: %v", err)
	}
	if err := progressStore.Save("kid-1", progress.KidProgress{CardState: map[string]progress.CardState{review.DirectionKey("pair-1", "a"): {}, review.DirectionKey("pair-1", "b"): {}}}); err != nil {
		t.Fatalf("save progress: %v", err)
	}
	handler := NewTopicHandler(TopicDependencies{Sessions: kidSessions(t), Content: contentService, Progress: progressStore})
	cookie, csrfToken := topicKidLogin(t, handler)
	form := url.Values{"csrf_token": {csrfToken}, "subject": {"Frans"}, "period": {"Hoofdstuk 4"}}
	request := httptest.NewRequest(http.MethodPost, "/kid/topics/delete", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d", response.Code)
	}
	kidProgress, err := progressStore.Load("kid-1")
	if err != nil || len(kidProgress.CardState) != 0 {
		t.Fatalf("progress = %#v, err = %v", kidProgress, err)
	}
}

func TestSessionSelectionSuggestsAndPersistsKidList(t *testing.T) {
	contentService := content.NewService(t.TempDir())
	progressStore := progress.NewStore(t.TempDir())
	for _, list := range []content.List{{Subject: "Frans", Period: "Oud", UpdatedAt: time.Now().Add(-time.Hour)}, {Subject: "Duits", Period: "Nieuw", UpdatedAt: time.Now()}} {
		if err := contentService.SaveList("kid-1", list); err != nil {
			t.Fatalf("save list: %v", err)
		}
	}
	handler := NewTopicHandler(TopicDependencies{Sessions: kidSessions(t), Content: contentService, Progress: progressStore})
	cookie, csrfToken := topicKidLogin(t, handler)
	page := httptest.NewRequest(http.MethodGet, "/kid/sessions/new", nil)
	page.AddCookie(cookie)
	pageResponse := httptest.NewRecorder()
	handler.ServeHTTP(pageResponse, page)
	if pageResponse.Code != http.StatusOK || !strings.Contains(pageResponse.Body.String(), "Duits") {
		t.Fatalf("selection page = %d: %s", pageResponse.Code, pageResponse.Body.String())
	}
	form := url.Values{"csrf_token": {csrfToken}, "subject": {"Frans"}, "period": {"Oud"}}
	request := httptest.NewRequest(http.MethodPost, "/kid/sessions/select", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("selection status = %d", response.Code)
	}
	kidProgress, err := progressStore.Load("kid-1")
	if err != nil || kidProgress.LastSubject != "Frans" || kidProgress.LastPeriod != "Oud" {
		t.Fatalf("selection progress = %#v, err = %v", kidProgress, err)
	}
}

func TestStudyRouteLoadsSavedSelectedList(t *testing.T) {
	contentService := content.NewService(t.TempDir())
	progressStore := progress.NewStore(t.TempDir())
	if err := contentService.SaveList("kid-1", content.List{Subject: "Frans", Period: "Hoofdstuk 4", Cards: []content.Card{{ID: "pair-1", Front: "bonjour", Back: "goedendag"}}}); err != nil {
		t.Fatalf("save list: %v", err)
	}
	if err := progressStore.Save("kid-1", progress.KidProgress{LastSubject: "Frans", LastPeriod: "Hoofdstuk 4"}); err != nil {
		t.Fatalf("save selection: %v", err)
	}
	handler := NewTopicHandler(TopicDependencies{Sessions: kidSessions(t), Content: contentService, Progress: progressStore})
	cookie, _ := topicKidLogin(t, handler)
	request := httptest.NewRequest(http.MethodGet, "/kid/study", nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || (!strings.Contains(response.Body.String(), "bonjour") && !strings.Contains(response.Body.String(), "goedendag")) {
		t.Fatalf("study response = %d: %s", response.Code, response.Body.String())
	}
}

func TestStudyRouteKeepsPronunciationForRecognizedLanguageSubject(t *testing.T) {
	contentService := content.NewService(t.TempDir())
	progressStore := progress.NewStore(t.TempDir())
	if err := contentService.SaveList("kid-1", content.List{Subject: "Engels", Period: "1", Cards: []content.Card{{ID: "pair-1", Front: "hello", Back: "hallo"}}}); err != nil {
		t.Fatalf("save list: %v", err)
	}
	if err := progressStore.Save("kid-1", progress.KidProgress{LastSubject: "Engels", LastPeriod: "1"}); err != nil {
		t.Fatalf("save selection: %v", err)
	}
	handler := NewTopicHandler(TopicDependencies{Sessions: kidSessions(t), Content: contentService, Progress: progressStore})
	cookie, _ := topicKidLogin(t, handler)
	request := httptest.NewRequest(http.MethodGet, "/student/study", nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `id="pronunciation-button"`) {
		t.Fatalf("study response = %d: %s", response.Code, response.Body.String())
	}
}

func TestStudentSessionSelectionUsesMixedModeByDefault(t *testing.T) {
	contentService := content.NewService(t.TempDir())
	progressStore := progress.NewStore(t.TempDir())
	cards := []content.Card{{ID: "one", Front: "one", Back: "een"}, {ID: "two", Front: "two", Back: "twee"}, {ID: "three", Front: "three", Back: "drie"}, {ID: "four", Front: "four", Back: "vier"}}
	if err := contentService.SaveList("kid-1", content.List{Subject: "Engels", Period: "1", Cards: cards}); err != nil {
		t.Fatalf("save list: %v", err)
	}
	handler := NewTopicHandler(TopicDependencies{Sessions: kidSessions(t), Content: contentService, Progress: progressStore})
	cookie, csrfToken := topicKidLogin(t, handler)
	page := httptest.NewRequest(http.MethodGet, "/student/sessions/new", nil)
	page.AddCookie(cookie)
	pageResponse := httptest.NewRecorder()
	handler.ServeHTTP(pageResponse, page)
	if pageResponse.Code != http.StatusOK || !strings.Contains(pageResponse.Body.String(), `value="mixed" checked`) || !strings.Contains(pageResponse.Body.String(), "Alleen typen") {
		t.Fatalf("selection page = %d: %s", pageResponse.Code, pageResponse.Body.String())
	}
	form := url.Values{"csrf_token": {csrfToken}, "subject": {"Engels"}, "period": {"1"}, "mode": {"mixed"}, "difficulty": {"easy"}}
	request := httptest.NewRequest(http.MethodPost, "/student/sessions/select", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/student/study?mode=mixed&difficulty=easy" {
		t.Fatalf("selection redirect = %d: %s", response.Code, response.Header().Get("Location"))
	}
}

func TestStudentStudyCompletionPersistsProgress(t *testing.T) {
	contentService := content.NewService(t.TempDir())
	progressStore := progress.NewStore(t.TempDir())
	handler := NewTopicHandler(TopicDependencies{Sessions: kidSessions(t), Content: contentService, Progress: progressStore})
	cookie, csrfToken := topicKidLogin(t, handler)
	form := url.Values{"csrf_token": {csrfToken}, "subject": {"Engels"}, "period": {"1"}, "cards_seen": {"3"}, "correct_first_try": {"2"}, "total_attempts": {"4"}}
	request := httptest.NewRequest(http.MethodPost, "/student/study/complete", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("completion status = %d", response.Code)
	}
	kidProgress, err := progressStore.Load("kid-1")
	if err != nil || len(kidProgress.Sessions) != 1 || kidProgress.Sessions[0].CardsSeen != 3 || kidProgress.Sessions[0].CorrectFirstTry != 2 {
		t.Fatalf("progress = %#v, err = %v", kidProgress, err)
	}
}
