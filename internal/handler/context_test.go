package handler_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/internal/handler"
)

func TestUserContext(t *testing.T) {
	t.Parallel()
	user := &ent.User{
		ID:       "test-user-123",
		Username: "testuser",
		Email:    "test@example.com",
	}

	userCtx := handler.WithUser(context.Background(), user)
	retrievedUser, ok := handler.User(userCtx)
	assert.True(t, ok)
	assert.Equal(t, user.ID, retrievedUser.ID)
	assert.Equal(t, user.Username, retrievedUser.Username)
	assert.Equal(t, user.Email, retrievedUser.Email)

	emptyUser, ok := handler.User(context.Background())
	assert.False(t, ok)
	assert.Nil(t, emptyUser)
}

func TestMustUser(t *testing.T) {
	t.Parallel()
	user := &ent.User{
		ID:       "test-user-456",
		Username: "testuser2",
		Email:    "test2@example.com",
	}

	ctx := handler.WithUser(context.Background(), user)
	assert.Equal(t, user.ID, handler.MustUser(ctx).ID)

	assert.Panics(t, func() {
		handler.MustUser(context.Background())
	})
}
