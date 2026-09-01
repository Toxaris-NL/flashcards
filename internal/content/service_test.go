package content

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateListCreatesEmptyListFile(t *testing.T) {
	dir := t.TempDir()
	service := NewService(dir)

	list, err := service.CreateList("kid-1", "Frans", "Hoofdstuk 4")
	if err != nil {
		t.Fatalf("create list failed: %v", err)
	}
	if list.Subject != "Frans" {
		t.Fatalf("expected subject Frans, got %q", list.Subject)
	}
	if len(list.Cards) != 0 {
		t.Fatalf("expected empty cards, got %d", len(list.Cards))
	}

	_, err = os.Stat(filepath.Join(dir, "kid-1", "frans", "hoofdstuk-4.json"))
	if err != nil {
		t.Fatalf("expected list file to exist: %v", err)
	}
}

func TestCSVPreviewRejectsMalformedRows(t *testing.T) {
	preview, err := ParseCSVPreview("front;back\nle chat;de kat\ninvalid\n")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(preview.Cards) != 1 {
		t.Fatalf("expected one valid card, got %d", len(preview.Cards))
	}
	if len(preview.Errors) != 1 {
		t.Fatalf("expected one row error, got %d", len(preview.Errors))
	}
}

func TestJSONPreviewAcceptsArrayOfFrontBackObjects(t *testing.T) {
	preview, err := ParseJSONPreview(`[{"front":"bonjour","back":"hallo"},{"front":"merci","back":"dank je"}]`)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(preview.Cards) != 2 {
		t.Fatalf("expected two cards, got %d", len(preview.Cards))
	}
	if len(preview.Errors) != 0 {
		t.Fatalf("expected no errors, got %d", len(preview.Errors))
	}
}

func TestLanguageSubjectsMapCaseInsensitively(t *testing.T) {
	for subject, want := range map[string]string{"Frans": "fr", " duits ": "de", "ENGELS": "en", "Nederlands": "nl"} {
		got, ok := LanguageCode(subject)
		if !ok || got != want {
			t.Fatalf("LanguageCode(%q) = %q, %t; want %q, true", subject, got, ok, want)
		}
	}
	if _, ok := LanguageCode("Geschiedenis"); ok {
		t.Fatal("non-language subject must not have a language code")
	}
}

func TestSubjectAwareImportsApplyDefaultsAndKeepOverrides(t *testing.T) {
	csvPreview, err := ParseCSVPreviewForSubject("bonjour;hallo", "Frans")
	if err != nil || !csvPreview.Cards[0].IsLanguageCard {
		t.Fatalf("French CSV card = %#v, err = %v", csvPreview.Cards, err)
	}
	nonLanguagePreview, err := ParseCSVPreviewForSubject("jaartal;1940", "Geschiedenis")
	if err != nil || nonLanguagePreview.Cards[0].IsLanguageCard {
		t.Fatalf("history CSV card = %#v, err = %v", nonLanguagePreview.Cards, err)
	}
	jsonPreview, err := ParseJSONPreviewForSubject(`[ {"front":"bonjour","back":"hallo","is_language_card":false} ]`, "Frans")
	if err != nil || jsonPreview.Cards[0].IsLanguageCard {
		t.Fatalf("explicit JSON override = %#v, err = %v", jsonPreview.Cards, err)
	}
}

func TestNeutralPairImportsAcceptSideNamesAndLanguageOverrides(t *testing.T) {
	csvPreview, err := ParseCSVPreviewForSubject("side_a;side_b\nbonjour;goedendag", "Geschiedenis")
	if err != nil || len(csvPreview.Cards) != 1 || csvPreview.Cards[0].Front != "bonjour" || csvPreview.Cards[0].Back != "goedendag" {
		t.Fatalf("neutral CSV preview = %#v, err = %v", csvPreview, err)
	}
	jsonPreview, err := ParseJSONPreviewForSubject(`[ {"side_a":"bonjour","side_b":"goedendag","side_a_language":"fr","side_b_language":"nl"} ]`, "Geschiedenis")
	if err != nil || len(jsonPreview.Cards) != 1 || jsonPreview.Cards[0].SideALanguage != "fr" || jsonPreview.Cards[0].SideBLanguage != "nl" {
		t.Fatalf("neutral JSON preview = %#v, err = %v", jsonPreview, err)
	}
}

func TestNormalizeListAssignsPairIDsAndUsesSideLanguageOverrides(t *testing.T) {
	list := List{
		SideADefaultLanguage: "fr",
		SideBDefaultLanguage: "nl",
		Cards:                []Card{{Front: "bonjour", Back: "goedendag", SideBLanguage: "de"}},
	}
	if err := NormalizeList(&list); err != nil {
		t.Fatalf("normalize list: %v", err)
	}
	if list.Cards[0].ID == "" || EffectiveSideLanguage(list.Cards[0], &list, "a") != "fr" || EffectiveSideLanguage(list.Cards[0], &list, "b") != "de" || !list.Cards[0].IsLanguageCard {
		t.Fatalf("normalized list = %#v", list)
	}
}

func TestNormalizeListRejectsUnsupportedLanguage(t *testing.T) {
	list := List{SideADefaultLanguage: "es"}
	if err := NormalizeList(&list); err == nil {
		t.Fatal("expected unsupported language error")
	}
}

func TestListLifecycleMigratesIDsAndDiscoversLists(t *testing.T) {
	service := NewService(t.TempDir())
	legacy := List{Subject: "Frans", Period: "Hoofdstuk 4", Cards: []Card{{Front: "bonjour", Back: "goedendag"}}}
	if err := service.SaveList("kid-1", legacy); err != nil {
		t.Fatalf("save legacy list: %v", err)
	}
	loaded, err := service.LoadList("kid-1", "Frans", "Hoofdstuk 4")
	if err != nil || loaded.Cards[0].ID == "" {
		t.Fatalf("loaded list = %#v, err = %v", loaded, err)
	}
	if err := service.SaveList("kid-1", List{Subject: "Frans", Period: "Hoofdstuk 5"}); err != nil {
		t.Fatalf("save second list: %v", err)
	}
	references, err := service.ListReferences("kid-1")
	if err != nil || len(references) != 2 {
		t.Fatalf("references = %#v, err = %v", references, err)
	}
	if err := service.DeleteList("kid-1", "Frans", "Hoofdstuk 5"); err != nil {
		t.Fatalf("delete list: %v", err)
	}
	references, err = service.ListReferences("kid-1")
	if err != nil || len(references) != 1 {
		t.Fatalf("references after delete = %#v, err = %v", references, err)
	}
}

func TestDistractorPoolIsNotEnoughForShortLists(t *testing.T) {
	cards := []Card{
		{Front: "one", Back: "een"},
		{Front: "two", Back: "twee"},
		{Front: "three", Back: "drie"},
	}
	if !NeedsTypedOnlyFallback(cards) {
		t.Fatal("expected typed-only fallback for a short deck")
	}
	if len(DistractorBacks(cards)) != 0 {
		t.Fatal("expected no distractor pool for a short deck")
	}
}

func TestDistractorPoolUsesOtherCardBackValues(t *testing.T) {
	cards := []Card{
		{Front: "one", Back: "een"},
		{Front: "two", Back: "twee"},
		{Front: "three", Back: "drie"},
		{Front: "four", Back: "vier"},
		{Front: "five", Back: "vijf"},
	}
	pool := DistractorBacks(cards)
	if len(pool) != 4 {
		t.Fatalf("expected 4 distractors, got %d", len(pool))
	}
	if NeedsTypedOnlyFallback(cards) {
		t.Fatal("expected mixed mode for a large enough deck")
	}
}
