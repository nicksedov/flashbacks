package database

import "testing"

// TestSeedArrayLiteral guards the wire format of the neighbour query's seed
// parameter: it must be a single PostgreSQL array literal. Passing a Go slice
// made GORM expand it into one placeholder per element, so unnest() received
// thousands of scalar arguments and every clustering pass failed with
// "function pg_catalog.unnest(unknown) is not unique".
func TestSeedArrayLiteral(t *testing.T) {
	testCases := []struct {
		name  string
		seeds []int64
		want  string
	}{
		{name: "empty", seeds: nil, want: "{}"},
		{name: "one", seeds: []int64{7}, want: "{7}"},
		{name: "many", seeds: []int64{1, 2, 39024}, want: "{1,2,39024}"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := seedArrayLiteral(testCase.seeds); got != testCase.want {
				t.Fatalf("seedArrayLiteral(%v) = %q, want %q",
					testCase.seeds, got, testCase.want)
			}
		})
	}
}
