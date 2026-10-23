package services

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"examshield/internal/models"
)

func pool(mcq, gap, text int) []models.Question {
	var qs []models.Question
	id := int64(0)
	add := func(n int, k models.QuestionKind) {
		for i := 0; i < n; i++ {
			id++
			qs = append(qs, models.Question{ID: id, Kind: k, Marks: 1})
		}
	}
	add(mcq, models.QuestionMCQ)
	add(gap, models.QuestionGap)
	add(text, models.QuestionText)
	return qs
}

func TestDrawQuestions(t *testing.T) {
	qs := pool(160, 48, 32) // 240 questions: 2/3 MCQ, 1/5 gap, 2/15 written
	rng := rand.New(rand.NewSource(1))
	used := map[int64]int{}
	kind := map[int64]models.QuestionKind{}
	for _, q := range qs {
		kind[q.ID] = q.Kind
	}

	var papers [][]int64
	for s := 0; s < 8; s++ { // 8 x 30 = 240: the whole pool, no repeats
		ids := DrawQuestions(qs, 30, used, rng)
		require.Len(t, ids, 30)
		count := map[models.QuestionKind]int{}
		for _, id := range ids {
			count[kind[id]]++
			used[id]++
		}
		assert.Equal(t, map[models.QuestionKind]int{models.QuestionMCQ: 20, models.QuestionGap: 6, models.QuestionText: 4}, count,
			"every paper keeps the pool's mix")
		papers = append(papers, ids)
	}
	for id, n := range used {
		assert.Equal(t, 1, n, "question %d given to %d students before the pool ran out", id, n)
	}
	assert.NotEqual(t, papers[0], papers[1])

	// Once the pool is used up, the least-used questions come next.
	used[1] = 0
	ids := DrawQuestions(qs, 30, used, rng)
	assert.Contains(t, ids, int64(1))

	assert.Nil(t, DrawQuestions(qs, 0, nil, rng), "0 = everyone gets all questions")
	assert.Nil(t, DrawQuestions(qs, 240, nil, rng))

	small := pool(3, 0, 1)
	got := DrawQuestions(small, 3, map[int64]int{}, rng)
	assert.Len(t, got, 3)
}

func TestAssignedQuestions(t *testing.T) {
	qs := pool(3, 0, 0)
	assert.Equal(t, qs, AssignedQuestions(qs, nil))
	got := AssignedQuestions(qs, []int64{3, 1})
	require.Len(t, got, 2)
	assert.Equal(t, int64(3), got[0].ID, "student's own order")
}
