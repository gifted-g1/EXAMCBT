package services

import (
	"context"
	"math"
	"math/rand"
	"sort"
	"time"

	"examshield/internal/models"
	"examshield/internal/repositories"
)

// AssignedQuestions returns the questions one session sees, in its order.
// Sessions without a draw (ids nil) see every question in the exam order.
func AssignedQuestions(qs []models.Question, ids []int64) []models.Question {
	if ids == nil {
		return qs
	}
	byID := make(map[int64]models.Question, len(qs))
	for _, q := range qs {
		byID[q.ID] = q
	}
	out := make([]models.Question, 0, len(ids))
	for _, id := range ids {
		if q, ok := byID[id]; ok {
			out = append(out, q)
		}
	}
	return out
}

// DrawQuestions picks n questions for a new student from the exam's pool.
//
//   - Each kind (multiple choice, fill in the gap, written) keeps its share
//     of the pool, so every student gets a comparable paper.
//   - Within a kind, the questions handed out least so far (used) are
//     preferred, ties broken at random. Students therefore share no
//     questions until the pool runs out, and overlap stays as low as the
//     pool allows after that.
//   - The result is ordered by kind (in the pool's order) and shuffled
//     within each kind.
//
// It returns nil when n is 0 or covers the whole pool: everyone gets all
// questions.
func DrawQuestions(qs []models.Question, n int, used map[int64]int, rng *rand.Rand) []int64 {
	if n <= 0 || n >= len(qs) {
		return nil
	}
	var kinds []models.QuestionKind
	groups := map[models.QuestionKind][]models.Question{}
	for _, q := range qs {
		if _, ok := groups[q.Kind]; !ok {
			kinds = append(kinds, q.Kind)
		}
		groups[q.Kind] = append(groups[q.Kind], q)
	}

	// Largest-remainder apportionment of n across the kinds.
	quota := make(map[models.QuestionKind]int, len(kinds))
	type rem struct {
		k models.QuestionKind
		f float64
	}
	var rems []rem
	given := 0
	for _, k := range kinds {
		exact := float64(n) * float64(len(groups[k])) / float64(len(qs))
		quota[k] = int(math.Floor(exact))
		given += quota[k]
		rems = append(rems, rem{k, exact - math.Floor(exact)})
	}
	sort.SliceStable(rems, func(i, j int) bool { return rems[i].f > rems[j].f })
	for i := 0; given < n; i = (i + 1) % len(rems) {
		if k := rems[i].k; quota[k] < len(groups[k]) {
			quota[k]++
			given++
		}
	}

	out := make([]int64, 0, n)
	for _, k := range kinds {
		g := append([]models.Question(nil), groups[k]...)
		rng.Shuffle(len(g), func(i, j int) { g[i], g[j] = g[j], g[i] })
		sort.SliceStable(g, func(i, j int) bool { return used[g[i].ID] < used[g[j].ID] })
		pick := g[:quota[k]]
		rng.Shuffle(len(pick), func(i, j int) { pick[i], pick[j] = pick[j], pick[i] })
		for _, q := range pick {
			out = append(out, q.ID)
		}
	}
	return out
}

// drawFor draws a new student's questions, preferring those handed out
// least so far in this exam.
func drawFor(ctx context.Context, tx *repositories.Repo, e *models.Exam) ([]int64, error) {
	qs, err := tx.Questions(ctx, e.ID)
	if err != nil {
		return nil, err
	}
	drawn, err := tx.DrawnQuestionIDs(ctx, e.ID)
	if err != nil {
		return nil, err
	}
	used := map[int64]int{}
	for _, ids := range drawn {
		for _, id := range ids {
			used[id]++
		}
	}
	return DrawQuestions(qs, e.QuestionsPerStudent, used, rand.New(rand.NewSource(time.Now().UnixNano()))), nil
}

func containsID(ids []int64, id int64) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}
