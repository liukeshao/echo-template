package appctx

import (
	"context"
	"testing"

	"github.com/liukeshao/echo-template/ent"
	"github.com/stretchr/testify/assert"
)

func TestUserContext(t *testing.T) {
	// 创建一个mock用户
	user := &ent.User{
		ID:       "test-user-123",
		Username: "testuser",
		Email:    "test@example.com",
	}

	// 测试WithUser和GetUserFromContext
	ctx := context.Background()
	userCtx := WithUser(ctx, user)

	retrievedUser, ok := GetUserFromContext(userCtx)
	assert.True(t, ok, "应该能从context中获取用户")
	assert.Equal(t, user.ID, retrievedUser.ID, "用户ID应该匹配")
	assert.Equal(t, user.Username, retrievedUser.Username, "用户名应该匹配")
	assert.Equal(t, user.Email, retrievedUser.Email, "邮箱应该匹配")

	// 测试从空context获取用户
	emptyUser, ok := GetUserFromContext(context.Background())
	assert.False(t, ok, "从空context中应该无法获取用户")
	assert.Nil(t, emptyUser, "空context中的用户应该为nil")
}

func TestMustGetUser(t *testing.T) {
	// 创建一个mock用户
	user := &ent.User{
		ID:       "test-user-456",
		Username: "testuser2",
		Email:    "test2@example.com",
	}

	// 测试MustGetUser正常情况
	ctx := WithUser(context.Background(), user)
	retrievedUser := MustGetUserFromContext(ctx)
	assert.Equal(t, user.ID, retrievedUser.ID, "MustGetUser应该返回正确的用户")

	// 测试MustGetUser panic情况
	assert.Panics(t, func() {
		MustGetUserFromContext(context.Background())
	}, "MustGetUser在没有用户时应该panic")
}

func TestRequestIDContext(t *testing.T) {
	requestID := "req-123-456"

	// 测试WithRequestID和GetRequestIDFromContext
	ctx := WithRequestID(context.Background(), requestID)
	retrievedID, ok := GetRequestIDFromContext(ctx)

	assert.True(t, ok, "应该能从context中获取request ID")
	assert.Equal(t, requestID, retrievedID, "request ID应该匹配")

	// 测试MustGetRequestIDFromContext
	mustRetrievedID := MustGetRequestIDFromContext(ctx)
	assert.Equal(t, requestID, mustRetrievedID, "MustGetRequestIDFromContext应该返回正确的ID")

	// 测试从空context获取request ID
	emptyID := MustGetRequestIDFromContext(context.Background())
	assert.Empty(t, emptyID, "空context中的request ID应该为空字符串")
}
