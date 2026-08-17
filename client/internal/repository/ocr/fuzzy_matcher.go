package ocr

import (
	"image"
	"sort"
	"strings"

	"github.com/lithammer/fuzzysearch/fuzzy"
)

type CandidateResult struct {
	Text       string
	Score      int
	Confidence int
	Count      int
	Bounds     image.Rectangle
}

func AggregateCandidates(candidates []CandidateResult) []CandidateResult {
	if len(candidates) == 0 {
		return nil
	}

	grouped := make(map[string]*CandidateResult)
	for _, candidate := range candidates {
		text := strings.TrimSpace(candidate.Text)
		if text == "" {
			continue
		}

		key := strings.ToLower(text)
		if existing, ok := grouped[key]; ok {
			existing.Count++
			if candidate.Score > existing.Score {
				existing.Score = candidate.Score
			}
			if candidate.Confidence > existing.Confidence {
				existing.Confidence = candidate.Confidence
			}
			if len(candidate.Text) > len(existing.Text) {
				existing.Text = candidate.Text
			}
			if candidate.Bounds != (image.Rectangle{}) && (existing.Bounds == (image.Rectangle{}) || candidate.Confidence >= existing.Confidence) {
				existing.Bounds = candidate.Bounds
			}
			continue
		}

		copyCandidate := candidate
		copyCandidate.Count = 1
		if copyCandidate.Text == "" {
			copyCandidate.Text = text
		}
		grouped[key] = &copyCandidate
	}

	if len(grouped) == 0 {
		return nil
	}

	result := make([]CandidateResult, 0, len(grouped))
	for _, candidate := range grouped {
		result = append(result, *candidate)
	}

	sort.Slice(result, func(i, j int) bool {
		leftTotal := result[i].Score + result[i].Confidence + result[i].Count*20
		rightTotal := result[j].Score + result[j].Confidence + result[j].Count*20
		if leftTotal != rightTotal {
			return leftTotal > rightTotal
		}
		if result[i].Count != result[j].Count {
			return result[i].Count > result[j].Count
		}
		return len(result[i].Text) > len(result[j].Text)
	})

	return result
}

func ChooseBestCandidate(candidates []CandidateResult) CandidateResult {
	aggregated := AggregateCandidates(candidates)
	if len(aggregated) == 0 {
		return CandidateResult{}
	}

	return aggregated[0]
}

// FuzzyMatcher помогает находить похожие слова в распознанном тексте
type FuzzyMatcher struct {
	words         []string
	caseSensitive bool
}

// NewFuzzyMatcher создаёт новый matcher
func NewFuzzyMatcher(caseSensitive bool) *FuzzyMatcher {
	return &FuzzyMatcher{
		caseSensitive: caseSensitive,
	}
}

// LoadWords загружает список слов для поиска
func (fm *FuzzyMatcher) LoadWords(words []string) {
	fm.words = words
}

// FindMatches находит слова похожие на query с указанной точностью
// threshold: 0-100, где 100 - точное совпадение, 0 - максимально нечеткий поиск
func (fm *FuzzyMatcher) FindMatches(query string, threshold int) []string {
	if threshold < 0 {
		threshold = 0
	}
	if threshold > 100 {
		threshold = 100
	}

	// Ограничение расстояния редактирования (чем больше threshold, тем меньше ошибок допускаем)
	maxDistance := (100 - threshold) / 10
	if maxDistance == 0 && threshold < 100 {
		maxDistance = 1
	}

	var matches []string

	for _, word := range fm.words {
		searchWord := word
		searchQuery := query

		if !fm.caseSensitive {
			searchWord = strings.ToLower(word)
			searchQuery = strings.ToLower(query)
		}

		// Проверяем точное совпадение (вероятность 100%)
		if searchWord == searchQuery {
			matches = append(matches, word)
			continue
		}

		// Проверяем нечеткое совпадение
		if fuzzy.Match(searchQuery, searchWord) {
			// fuzzy.RankMatch дает оценку - чем выше, тем лучше совпадение
			rank := fuzzy.RankMatch(searchQuery, searchWord)
			if rank > -maxDistance {
				matches = append(matches, word)
			}
		}
	}

	return matches
}

// FindBestMatch находит самое похожее слово
func (fm *FuzzyMatcher) FindBestMatch(query string) (string, int) {
	if len(fm.words) == 0 {
		return "", 0
	}

	type Match struct {
		word  string
		score int
	}

	var matches []Match

	for _, word := range fm.words {
		searchWord := word
		searchQuery := query

		if !fm.caseSensitive {
			searchWord = strings.ToLower(word)
			searchQuery = strings.ToLower(query)
		}

		if searchWord == searchQuery {
			return word, 100
		}

		if fuzzy.Match(searchQuery, searchWord) {
			rank := fuzzy.RankMatch(searchQuery, searchWord)
			matches = append(matches, Match{word, rank})
		}
	}

	if len(matches) == 0 {
		return "", 0
	}

	// Сортируем по оценке (по убыванию)
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].score > matches[j].score
	})

	// Преобразуем оценку в процент (примерно)
	score := ((matches[0].score + 100) * 100) / 300
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	return matches[0].word, score
}

// MatchWithThreshold проверяет, совпадает ли слово с query с минимальной точностью
func (fm *FuzzyMatcher) MatchWithThreshold(query, word string, threshold int) bool {
	if threshold < 0 {
		threshold = 0
	}
	if threshold > 100 {
		threshold = 100
	}

	searchWord := word
	searchQuery := query

	if !fm.caseSensitive {
		searchWord = strings.ToLower(word)
		searchQuery = strings.ToLower(query)
	}

	if searchWord == searchQuery {
		return true
	}

	if !fuzzy.Match(searchQuery, searchWord) {
		return false
	}

	maxDistance := (100 - threshold) / 10
	if maxDistance == 0 && threshold < 100 {
		maxDistance = 1
	}

	rank := fuzzy.RankMatch(searchQuery, searchWord)
	return rank > -maxDistance
}
