package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStudyHandlerRendersLanguageCardWithMappedSpeechLanguage(t *testing.T) {
	handler := NewStudyHandler(StudyCard{Subject: "Frans", Front: "bonjour", Back: "hallo", IsLanguageCard: true})
	request := httptest.NewRequest(http.MethodGet, "/study", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, `data-language="fr"`) || !strings.Contains(body, `id="pronunciation-button"`) || strings.Contains(body, `id="pronunciation-button" class="pronunciation-button" type="button" hidden`) {
		t.Fatalf("language response = %d: %s", response.Code, body)
	}
}

func TestStudyHandlerRendersDistinctPairSideLanguages(t *testing.T) {
	handler := NewStudyHandler(StudyCard{Front: "bonjour", Back: "goedendag", IsLanguageCard: true, FrontLanguage: "fr", BackLanguage: "nl"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/study", nil))
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, `data-language="fr"`) || !strings.Contains(body, `data-language="nl"`) {
		t.Fatalf("side language response = %d: %s", response.Code, body)
	}
}

func TestStudyHandlerOmitsControlForNonLanguageCard(t *testing.T) {
	handler := NewStudyHandler(StudyCard{Subject: "Geschiedenis", Front: "1940", Back: "begin oorlog"})
	request := httptest.NewRequest(http.MethodGet, "/study", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	body := response.Body.String()
	if response.Code != http.StatusOK || strings.Contains(body, `id="pronunciation-button"`) {
		t.Fatalf("non-language response = %d: %s", response.Code, body)
	}
}

func TestStudyHandlerRendersControlForExplicitlyEnabledNonLanguageCard(t *testing.T) {
	handler := NewStudyHandler(StudyCard{Subject: "Geschiedenis", Front: "1940", Back: "begin oorlog", IsLanguageCard: true})
	request := httptest.NewRequest(http.MethodGet, "/study", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `id="pronunciation-button"`) {
		t.Fatalf("explicit language card response = %d: %s", response.Code, response.Body.String())
	}
}

func TestStudyHandlerRendersInteractiveSessionProgress(t *testing.T) {
	handler := NewStudyHandler(StudyCard{Subject: "Frans", Front: "bonjour", Back: "hallo"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/study", nil))
	body := response.Body.String()
	for _, expected := range []string{"data-flashcard", "data-flip", "data-score", "data-remaining", "data-stop-session", "study-session.js"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("study page does not contain %q: %s", expected, body)
		}
	}
}

func TestStudyHandlerServesPronunciationAssets(t *testing.T) {
	handler := NewStudyHandler(StudyCard{})
	request := httptest.NewRequest(http.MethodGet, "/static/pronunciation.js", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "speechSynthesis") || !strings.Contains(response.Body.String(), "[data-study-session]") || !strings.Contains(response.Header().Get("Content-Type"), "javascript") {
		t.Fatalf("asset response = %d: %s", response.Code, response.Body.String())
	}
}

func TestStudyHandlerServesStylesheetWithCSSContentType(t *testing.T) {
	handler := NewStudyHandler(StudyCard{})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/static/study.css", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Header().Get("Content-Type"), "text/css") {
		t.Fatalf("stylesheet response = %d: %s", response.Code, response.Header().Get("Content-Type"))
	}
}
