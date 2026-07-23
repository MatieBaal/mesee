package translator

import (
	"context"
	"fmt"
)

type MockTranslator struct{}

func NewMockTranslator() *MockTranslator {
	return &MockTranslator{}
}

func (m *MockTranslator) Translate(ctx context.Context, text string, targetLang string) (string, error) {
	return fmt.Sprintf("[Перевод:%s]", text), nil
}
