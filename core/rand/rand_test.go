package rand_test

import (
	"testing"

	"github.com/dobyte/due/v2/core/rand"
)

func Test_Shuffle(t *testing.T) {
	rand.NewRand(&rand.MathRand{}).Shuffle([]int{1, 2, 3, 4, 5})
	// t.Log()
}
