package content

import (
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const typedOnlyMinCards = 4

var languageSubjects = map[string]string{
	"frans":      "fr",
	"duits":      "de",
	"engels":     "en",
	"nederlands": "nl",
}

// Card represents a single flashcard in a list.
type Card struct {
	ID             string `json:"id,omitempty"`
	Front          string `json:"front"`
	Back           string `json:"back"`
	SideALanguage  string `json:"side_a_language,omitempty"`
	SideBLanguage  string `json:"side_b_language,omitempty"`
	IsLanguageCard bool   `json:"is_language_card,omitempty"`
}

// List is a single subject-period collection.
type List struct {
	Subject              string    `json:"subject"`
	Period               string    `json:"period"`
	UpdatedAt            time.Time `json:"updated_at"`
	TypingMatch          string    `json:"typing_match,omitempty"`
	SideADefaultLanguage string    `json:"side_a_language,omitempty"`
	SideBDefaultLanguage string    `json:"side_b_language,omitempty"`
	Cards                []Card    `json:"cards"`
}

// ImportPreview holds the imported cards and validation issues.
type ImportPreview struct {
	Cards  []Card
	Errors []string
}

// Service manages content on disk per kid.
type Service struct {
	root string
}

func NewService(root string) *Service {
	return &Service{root: root}
}

func (s *Service) CreateList(kidID, subjectName, periodName string) (*List, error) {
	if strings.TrimSpace(subjectName) == "" || strings.TrimSpace(periodName) == "" {
		return nil, fmt.Errorf("subject and period are required")
	}

	subjectSlug := slugify(subjectName)
	periodSlug := slugify(periodName)

	listPath := filepath.Join(s.root, kidID, subjectSlug, periodSlug+".json")
	if err := os.MkdirAll(filepath.Dir(listPath), 0o755); err != nil {
		return nil, err
	}

	list := &List{
		Subject:     subjectName,
		Period:      periodName,
		UpdatedAt:   time.Now().UTC(),
		TypingMatch: "normalized",
		Cards:       []Card{},
	}

	if _, err := os.Stat(listPath); err == nil {
		return list, nil
	}

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(listPath, data, 0o600); err != nil {
		return nil, err
	}
	return list, nil
}

// SaveList validates, normalizes, and writes a subject-period list for one kid.
func (s *Service) SaveList(kidID string, list List) error {
	if strings.TrimSpace(list.Subject) == "" || strings.TrimSpace(list.Period) == "" {
		return fmt.Errorf("subject and period are required")
	}
	if err := NormalizeList(&list); err != nil {
		return err
	}
	list.UpdatedAt = time.Now().UTC()
	return s.writeList(kidID, list)
}

// LoadList returns a saved subject-period list for one kid.
func (s *Service) LoadList(kidID, subject, period string) (*List, error) {
	data, err := os.ReadFile(s.listPath(kidID, subject, period))
	if err != nil {
		return nil, err
	}
	var list List
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	return &list, nil
}

// ListReference identifies one saved source list for selection controls.
type ListReference struct {
	Subject   string
	Period    string
	UpdatedAt time.Time
}

// ListReferences discovers every saved list for a kid.
func (s *Service) ListReferences(kidID string) ([]ListReference, error) {
	root := filepath.Join(s.root, kidID)
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return []ListReference{}, nil
	}
	if err != nil {
		return nil, err
	}
	references := []ListReference{}
	for _, subjectEntry := range entries {
		if !subjectEntry.IsDir() {
			continue
		}
		periodEntries, err := os.ReadDir(filepath.Join(root, subjectEntry.Name()))
		if err != nil {
			return nil, err
		}
		for _, periodEntry := range periodEntries {
			if periodEntry.IsDir() || filepath.Ext(periodEntry.Name()) != ".json" {
				continue
			}
			data, err := os.ReadFile(filepath.Join(root, subjectEntry.Name(), periodEntry.Name()))
			if err != nil {
				return nil, err
			}
			var list List
			if err := json.Unmarshal(data, &list); err != nil {
				return nil, err
			}
			references = append(references, ListReference{Subject: list.Subject, Period: list.Period, UpdatedAt: list.UpdatedAt})
		}
	}
	return references, nil
}

// DeleteList explicitly removes one saved source list.
func (s *Service) DeleteList(kidID, subject, period string) error {
	return os.Remove(s.listPath(kidID, subject, period))
}

func (s *Service) writeList(kidID string, list List) error {
	path := s.listPath(kidID, list.Subject, list.Period)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func (s *Service) listPath(kidID, subject, period string) string {
	return filepath.Join(s.root, kidID, slugify(subject), slugify(period)+".json")
}

func ParseCSVPreview(input string) (ImportPreview, error) {
	return ParseCSVPreviewForSubject(input, "")
}

// ParseCSVPreviewForSubject applies language-card defaults for the list subject.
func ParseCSVPreviewForSubject(input, subject string) (ImportPreview, error) {
	preview := ImportPreview{}
	reader := csv.NewReader(strings.NewReader(input))
	reader.Comma = ';'
	reader.FieldsPerRecord = -1

	rows, err := reader.ReadAll()
	if err != nil {
		return preview, err
	}
	if len(rows) == 0 {
		return preview, nil
	}

	for lineNo, row := range rows {
		if len(row) == 0 {
			continue
		}
		if len(row) == 2 && isPairHeader(row[0], row[1]) {
			continue
		}
		if len(row) != 2 {
			preview.Errors = append(preview.Errors, fmt.Sprintf("regel %d: verwacht 2 velden", lineNo+1))
			continue
		}
		front := strings.TrimSpace(row[0])
		back := strings.TrimSpace(row[1])
		if front == "" || back == "" {
			preview.Errors = append(preview.Errors, fmt.Sprintf("regel %d: front en back zijn verplicht", lineNo+1))
			continue
		}
		preview.Cards = append(preview.Cards, NewCard(subject, front, back))
	}
	return preview, nil
}

func ParseJSONPreview(input string) (ImportPreview, error) {
	return ParseJSONPreviewForSubject(input, "")
}

// ParseJSONPreviewForSubject preserves explicit language-card flags from imports.
func ParseJSONPreviewForSubject(input, subject string) (ImportPreview, error) {
	preview := ImportPreview{}
	var raw []struct {
		Front          string `json:"front"`
		Back           string `json:"back"`
		SideA          string `json:"side_a"`
		SideB          string `json:"side_b"`
		SideALanguage  string `json:"side_a_language"`
		SideBLanguage  string `json:"side_b_language"`
		IsLanguageCard *bool  `json:"is_language_card"`
	}
	if err := json.Unmarshal([]byte(input), &raw); err != nil {
		return preview, err
	}
	for i, item := range raw {
		front, back := item.Front, item.Back
		if front == "" && back == "" {
			front, back = item.SideA, item.SideB
		}
		if strings.TrimSpace(front) == "" || strings.TrimSpace(back) == "" {
			preview.Errors = append(preview.Errors, fmt.Sprintf("item %d: front en back zijn verplicht", i+1))
			continue
		}
		card := NewCard(subject, front, back)
		card.SideALanguage = item.SideALanguage
		card.SideBLanguage = item.SideBLanguage
		if item.IsLanguageCard != nil {
			card.IsLanguageCard = *item.IsLanguageCard
		}
		preview.Cards = append(preview.Cards, card)
	}
	return preview, nil
}

func isPairHeader(left, right string) bool {
	left = strings.ToLower(strings.TrimSpace(left))
	right = strings.ToLower(strings.TrimSpace(right))
	return (left == "front" && right == "back") || (left == "side_a" && right == "side_b")
}

// LanguageCode returns the browser speech language for a recognized subject.
func LanguageCode(subject string) (string, bool) {
	code, ok := languageSubjects[strings.ToLower(strings.TrimSpace(subject))]
	return code, ok
}

// NewCard creates a card with the language-card default for its subject.
func NewCard(subject, front, back string) Card {
	_, isLanguageCard := LanguageCode(subject)
	return Card{Front: front, Back: back, IsLanguageCard: isLanguageCard}
}

// NormalizeList assigns missing pair IDs and validates inherited side languages.
func NormalizeList(list *List) error {
	if !validLanguage(list.SideADefaultLanguage) || !validLanguage(list.SideBDefaultLanguage) {
		return fmt.Errorf("unsupported default pair language")
	}
	for index := range list.Cards {
		card := &list.Cards[index]
		if card.ID == "" {
			id, err := randomCardID()
			if err != nil {
				return err
			}
			card.ID = id
		}
		if !validLanguage(card.SideALanguage) || !validLanguage(card.SideBLanguage) {
			return fmt.Errorf("unsupported pair language")
		}
		card.IsLanguageCard = EffectiveSideLanguage(*card, list, "a") != "" || EffectiveSideLanguage(*card, list, "b") != ""
	}
	return nil
}

// EffectiveSideLanguage returns an override or the list default for one pair side.
func EffectiveSideLanguage(card Card, list *List, side string) string {
	if side == "a" {
		if card.SideALanguage != "" {
			return card.SideALanguage
		}
		return list.SideADefaultLanguage
	}
	if card.SideBLanguage != "" {
		return card.SideBLanguage
	}
	return list.SideBDefaultLanguage
}

func validLanguage(language string) bool {
	return language == "" || language == "fr" || language == "de" || language == "en" || language == "nl"
}

func randomCardID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func NeedsTypedOnlyFallback(cards []Card) bool {
	return len(cards) < typedOnlyMinCards
}

func DistractorBacks(cards []Card) []string {
	if NeedsTypedOnlyFallback(cards) {
		return nil
	}
	pool := make([]string, 0, len(cards)-1)
	for i, card := range cards {
		if i == 0 || card.Back == "" {
			continue
		}
		pool = append(pool, card.Back)
	}
	if len(pool) < typedOnlyMinCards-1 {
		return nil
	}
	return pool
}

var nonLetterPattern = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(value string) string {
	lower := strings.ToLower(strings.TrimSpace(value))
	lower = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		if r == ' ' || r == '-' || r == '_' {
			return '-'
		}
		return '-'
	}, lower)
	lower = strings.Trim(lower, "-")
	if lower == "" {
		return "untitled"
	}
	lower = nonLetterPattern.ReplaceAllString(lower, "-")
	lower = strings.Trim(lower, "-")
	if lower == "" {
		return "untitled"
	}
	return lower
}
