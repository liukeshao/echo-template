package service_test

import (
	"context"
	"testing"
	"uuid"

	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/internal/service"
	"github.com/liukeshao/echo-template/internal/testutil"
	"github.com/liukeshao/echo-template/pkg/api"
	"github.com/liukeshao/echo-template/pkg/rest"
	"github.com/liukeshao/echo-template/pkg/validator"
)

func TestBook_CRUDAndSoftDelete(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := testutil.New(t)
	bookSvc := do.MustInvoke[*service.Book](env.Injector)

	// Empty list short-circuits cleanly (nil request uses defaults).
	res, err := bookSvc.List(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, res.Items)
	assert.Equal(t, int32(0), res.TotalSize)
	assert.Equal(t, int32(1), res.Page)
	assert.Equal(t, int32(50), res.PageSize)

	// Create with custom UUID and verify duplicate returns BOOK_ALREADY_EXISTS.
	customID := uuid.NewV7().String()
	initialPrice := 49.99
	created, err := bookSvc.Create(ctx, &api.CreateBookRequest{
		BookID:      customID,
		Title:       "Go Programming",
		Author:      "Alan Donovan",
		Description: "Classic",
		Price:       &initialPrice,
	})
	require.NoError(t, err)
	assert.Equal(t, customID, created.ID)
	require.NotNil(t, created.Price)
	assert.InDelta(t, 49.99, *created.Price, 1e-9)

	// Free book (price = 0) must be distinct from unset (nil).
	zeroPrice := 0.0
	freeBook, err := bookSvc.Create(ctx, &api.CreateBookRequest{
		Title:  "Free Go Book",
		Author: "Community",
		Price:  &zeroPrice,
	})
	require.NoError(t, err)
	require.NotNil(t, freeBook.Price)
	assert.Equal(t, 0.0, *freeBook.Price)

	_, err = bookSvc.Create(ctx, &api.CreateBookRequest{
		BookID: customID,
		Title:  "Duplicate",
		Author: "Someone",
	})
	testutil.AssertOopsCode(t, err, rest.CodeConflict)

	// Invalid order_by returns *validator.FieldViolations with rule codes & metadata.
	_, err = bookSvc.List(ctx, &api.ListBooksRequest{PageSize: 10, OrderBy: "unknown_col"})
	var fv *validator.FieldViolations
	require.ErrorAs(t, err, &fv)
	require.Len(t, fv.Items(), 1)
	assert.Equal(t, validator.CodeUnknownOrderByField, fv.Items()[0].Code)
	assert.Equal(t, "unknown_col", fv.Items()[0].Metadata["value"])

	// Comma-only order_by falls back to default sort without error.
	res, err = bookSvc.List(ctx, &api.ListBooksRequest{PageSize: 10, OrderBy: " , , "})
	require.NoError(t, err)
	assert.Equal(t, int32(2), res.TotalSize)
	assert.Len(t, res.Items, 2)

	// Replace clears omitted optional fields (including nullable price → nil).
	replaced, err := bookSvc.Replace(ctx, customID, &api.ReplaceBookRequest{
		Book:   customID,
		Title:  "Go Programming 2nd",
		Author: "Alan Donovan",
	})
	require.NoError(t, err)
	assert.Equal(t, "Go Programming 2nd", replaced.Title)
	assert.Empty(t, replaced.Description)
	assert.Nil(t, replaced.Price)

	// Soft delete must succeed (regression test for SetDeleteTime → SetDeletedAt).
	require.NoError(t, bookSvc.Delete(ctx, customID))

	// Soft-deleted book is hidden from Get, Replace, and Delete.
	_, err = bookSvc.Get(ctx, customID)
	testutil.AssertOopsCode(t, err, rest.CodeNotFound)

	_, err = bookSvc.Replace(ctx, customID, &api.ReplaceBookRequest{
		Book:   customID,
		Title:  "Ghost",
		Author: "Ghost",
	})
	testutil.AssertOopsCode(t, err, rest.CodeNotFound)

	err = bookSvc.Delete(ctx, customID)
	testutil.AssertOopsCode(t, err, rest.CodeNotFound)

	// Even a direct Ent UpdateOneID must refuse to modify a soft-deleted row.
	err = env.ORM().Book.UpdateOneID(customID).SetTitle("Mutated").Exec(ctx)
	assert.True(t, ent.IsNotFound(err), "expected NotFoundError on soft-deleted row, got %v", err)
}

func TestBook_ListQueryFieldsAndOrderBy(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := testutil.New(t)
	bookSvc := do.MustInvoke[*service.Book](env.Injector)

	p1, p2 := 10.0, 50.0
	_, err := bookSvc.Create(ctx, &api.CreateBookRequest{Title: "Alpha Go", Author: "Alice", Price: &p1})
	require.NoError(t, err)
	_, err = bookSvc.Create(ctx, &api.CreateBookRequest{Title: "Beta Rust", Author: "Bob", Price: &p2})
	require.NoError(t, err)

	// Filter by explicit request fields: title + author + price range.
	maxPrice := 20.0
	minPrice := 5.0
	res, err := bookSvc.List(ctx, &api.ListBooksRequest{
		PageSize: 10,
		Title:    "go",
		Author:   "ali",
		MinPrice: &minPrice,
		MaxPrice: &maxPrice,
	})
	require.NoError(t, err)
	assert.Equal(t, int32(1), res.TotalSize)
	require.Len(t, res.Items, 1)
	assert.Equal(t, "Alpha Go", res.Items[0].Title)

	// All supported order_by fields.
	for _, ob := range []string{"title", "author desc", "price desc, title", "created_at", "updated_at desc", "published_at"} {
		_, err := bookSvc.List(ctx, &api.ListBooksRequest{PageSize: 10, OrderBy: ob})
		require.NoError(t, err, "order_by=%s", ob)
	}

	// Invalid order_by clause & direction return *validator.FieldViolations.
	var fv *validator.FieldViolations
	_, err = bookSvc.List(ctx, &api.ListBooksRequest{PageSize: 10, OrderBy: "title asc extra"})
	require.ErrorAs(t, err, &fv)
	assert.Equal(t, validator.CodeInvalidOrderByClause, fv.Items()[0].Code)

	_, err = bookSvc.List(ctx, &api.ListBooksRequest{PageSize: 10, OrderBy: "title asc"})
	require.ErrorAs(t, err, &fv)
	assert.Equal(t, validator.CodeInvalidOrderByDirection, fv.Items()[0].Code)
}
