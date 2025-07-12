package services

import (
	"context"
	"log/slog"

	"golang.org/x/crypto/bcrypt"

	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/ent/user"
	"github.com/liukeshao/echo-template/pkg/errors"
	"github.com/liukeshao/echo-template/pkg/types"
)

// MeService 用户服务
type MeService struct {
	orm *ent.Client
}

// NewMeService 创建用户服务实例
func NewMeService(orm *ent.Client) *MeService {
	return &MeService{
		orm: orm,
	}
}

// GetByID 根据ID获取用户
func (s *MeService) GetByID(ctx context.Context, userID string) (*types.UserOutput, error) {
	u, err := s.orm.User.Query().
		Where(user.IDEQ(userID)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			slog.WarnContext(ctx, "用户不存在", "user_id", userID)
			return nil, errors.ErrNotFound.With("user_id", userID).Errorf("用户不存在")
		}
		slog.ErrorContext(ctx, "获取用户失败", "error", err, "user_id", userID)
		return nil, errors.ErrInternal.Wrapf(err, "获取用户失败")
	}

	return &types.UserOutput{
		UserInfo: &types.UserInfo{
			ID:          u.ID,
			Username:    u.Username,
			Email:       u.Email,
			Status:      string(u.Status),
			LastLoginAt: u.LastLoginAt,
			CreatedAt:   u.CreatedAt,
		},
	}, nil
}

// updateUsername 更新用户名
func (s *MeService) updateUsername(ctx context.Context, userID string, username string) error {
	// 检查用户名是否已被其他用户使用
	exists, err := s.orm.User.Query().
		Where(
			user.UsernameEQ(username),
			user.IDNEQ(userID),
		).
		Exist(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "检查用户名是否存在失败", "error", err)
		return errors.ErrInternal.With("error", err.Error()).Errorf("检查用户名失败")
	}
	if exists {
		return errors.ErrConflict.With("username", username).Errorf("用户名已存在")
	}

	// 更新用户名
	err = s.orm.User.UpdateOneID(userID).
		SetUsername(username).
		Exec(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "更新用户名失败", "error", err, "user_id", userID)
		return errors.ErrInternal.Wrapf(err, "更新用户名失败")
	}

	return nil
}

// updateEmail 更新邮箱
func (s *MeService) updateEmail(ctx context.Context, userID string, email string) error {
	// 检查邮箱是否已被其他用户使用
	exists, err := s.orm.User.Query().
		Where(
			user.EmailEQ(email),
			user.IDNEQ(userID),
		).
		Exist(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "检查邮箱是否存在失败", "error", err)
		return errors.ErrInternal.With("error", err.Error()).Errorf("检查邮箱失败")
	}
	if exists {
		return errors.ErrConflict.With("email", email).Errorf("邮箱已存在")
	}

	// 更新邮箱
	err = s.orm.User.UpdateOneID(userID).
		SetEmail(email).
		Exec(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "更新邮箱失败", "error", err, "user_id", userID)
		return errors.ErrInternal.Wrapf(err, "更新邮箱失败")
	}

	return nil
}

// UpdateUsername 更新用户名
func (s *MeService) UpdateUsername(ctx context.Context, userID string, input *types.UpdateUsernameInput) (*types.UserOutput, error) {
	// 检查用户是否存在
	_, err := s.orm.User.Query().
		Where(user.IDEQ(userID)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			slog.WarnContext(ctx, "用户不存在", "user_id", userID)
			return nil, errors.ErrNotFound.With("user_id", userID).Errorf("用户不存在")
		}
		slog.ErrorContext(ctx, "获取用户失败", "error", err, "user_id", userID)
		return nil, errors.ErrInternal.Wrapf(err, "获取用户失败")
	}

	// 更新用户名
	err = s.updateUsername(ctx, userID, input.Username)
	if err != nil {
		return nil, err
	}

	// 获取更新后的用户信息
	updatedUser, err := s.orm.User.Query().
		Where(user.IDEQ(userID)).
		First(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "获取更新后用户信息失败", "error", err, "user_id", userID)
		return nil, errors.ErrInternal.Wrapf(err, "获取更新后用户信息失败")
	}

	return &types.UserOutput{
		UserInfo: &types.UserInfo{
			ID:          updatedUser.ID,
			Username:    updatedUser.Username,
			Email:       updatedUser.Email,
			Status:      string(updatedUser.Status),
			LastLoginAt: updatedUser.LastLoginAt,
			CreatedAt:   updatedUser.CreatedAt,
		},
	}, nil
}

// UpdateEmail 更新邮箱
func (s *MeService) UpdateEmail(ctx context.Context, userID string, input *types.UpdateEmailInput) (*types.UserOutput, error) {
	// 检查用户是否存在
	_, err := s.orm.User.Query().
		Where(user.IDEQ(userID)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			slog.WarnContext(ctx, "用户不存在", "user_id", userID)
			return nil, errors.ErrNotFound.With("user_id", userID).Errorf("用户不存在")
		}
		slog.ErrorContext(ctx, "获取用户失败", "error", err, "user_id", userID)
		return nil, errors.ErrInternal.Wrapf(err, "获取用户失败")
	}

	// 更新邮箱
	err = s.updateEmail(ctx, userID, input.Email)
	if err != nil {
		return nil, err
	}

	// 获取更新后的用户信息
	updatedUser, err := s.orm.User.Query().
		Where(user.IDEQ(userID)).
		First(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "获取更新后用户信息失败", "error", err, "user_id", userID)
		return nil, errors.ErrInternal.Wrapf(err, "获取更新后用户信息失败")
	}

	return &types.UserOutput{
		UserInfo: &types.UserInfo{
			ID:          updatedUser.ID,
			Username:    updatedUser.Username,
			Email:       updatedUser.Email,
			Status:      string(updatedUser.Status),
			LastLoginAt: updatedUser.LastLoginAt,
			CreatedAt:   updatedUser.CreatedAt,
		},
	}, nil
}

// ChangePassword 修改用户密码
func (s *MeService) ChangePassword(ctx context.Context, userID string, input *types.ChangePasswordInput) error {
	// 获取用户
	u, err := s.orm.User.Query().
		Where(user.IDEQ(userID)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			slog.WarnContext(ctx, "用户不存在", "user_id", userID)
			return errors.ErrNotFound.With("user_id", userID).Errorf("用户不存在")
		}
		slog.ErrorContext(ctx, "获取用户失败", "error", err, "user_id", userID)
		return errors.ErrInternal.Wrapf(err, "获取用户失败")
	}

	// 验证旧密码
	err = bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(input.OldPassword))
	if err != nil {
		slog.WarnContext(ctx, "旧密码验证失败", "user_id", userID)
		return errors.ErrUnauthorized.Errorf("旧密码不正确")
	}

	// 生成新密码哈希
	newPasswordHash, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		slog.ErrorContext(ctx, "生成新密码哈希失败", "error", err)
		return errors.ErrInternal.Wrapf(err, "密码加密失败")
	}

	// 更新密码
	err = s.orm.User.UpdateOneID(userID).
		SetPasswordHash(string(newPasswordHash)).
		Exec(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "更新密码失败", "error", err, "user_id", userID)
		return errors.ErrInternal.Wrapf(err, "更新密码失败")
	}

	return nil
}
