package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolvePagination(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                             string
		rawPage, rawSize                 int32
		wantOffset, wantLimit, wantPage  int32
	}{
		{"defaults on zero", 0, 0, 0, defaultPageSize, 1},
		{"negative normalized", -2, -5, 0, defaultPageSize, 1},
		{"normal page 3", 3, 20, 40, 20, 3},
		{"clamps over max", 2, 5000, maxPageSize, maxPageSize, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			offset, limit, page := resolvePagination(tc.rawPage, tc.rawSize, defaultPageSize, maxPageSize)
			assert.Equal(t, tc.wantOffset, offset)
			assert.Equal(t, tc.wantLimit, limit)
			assert.Equal(t, tc.wantPage, page)
		})
	}
}
