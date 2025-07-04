package services

import (
	"context"
	"log/slog"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/ent/user"
	"github.com/liukeshao/echo-template/pkg/errors"
	"github.com/liukeshao/echo-template/pkg/types"
	"github.com/liukeshao/echo-template/pkg/utils"
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

// toUserInfo 将用户实体转换为UserInfo
func (s *UserService) toUserInfo(u *ent.User) *types.UserInfo {
	// TODO: 从 UserRole 关联表获取角色信息
	// 注意：这里简化处理，实际使用时需要预加载角色信息或者在上层查询时一起获取
	var roles []string = []string{}

	// 处理可选字段的指针转换
	var realName, phone, department, position, lastLoginIP *string
	if u.RealName != "" {
		realName = &u.RealName
	}
	if u.Phone != "" {
		phone = &u.Phone
	}
	if u.Department != "" {
		department = &u.Department
	}
	if u.Position != "" {
		position = &u.Position
	}
	if u.LastLoginIP != "" {
		lastLoginIP = &u.LastLoginIP
	}

	return &types.UserInfo{
		ID:                  u.ID,
		Username:            u.Username,
		Email:               u.Email,
		RealName:            realName,
		Phone:               phone,
		Department:          department,
		Position:            position,
		Roles:               roles,
		Status:              string(u.Status),
		ForceChangePassword: u.ForceChangePassword,
		AllowMultiLogin:     u.AllowMultiLogin,
		LastLoginAt:         u.LastLoginAt,
		LastLoginIP:         lastLoginIP,
		CreatedAt:           u.CreatedAt,
	}
}

// CreateUser 创建用户
func (s *MeService) CreateUser(ctx context.Context, input *types.CreateUserInput) (*types.UserOutput, error) {
	// 检查用户名是否已存在
	exists, err := s.orm.User.Query().
		Where(user.UsernameEQ(input.Username)).
		Exist(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "检查用户名是否存在失败", "error", err)
		return nil, errors.ErrInternal.Wrapf(err, "检查用户名失败")
	}
	if exists {
		return nil, errors.ErrConflict.With("username", input.Username).Errorf("用户名已存在")
	}

	// 检查邮箱是否已存在
	exists, err = s.orm.User.Query().
		Where(user.EmailEQ(input.Email)).
		Exist(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "检查邮箱是否存在失败", "error", err)
		return nil, errors.ErrInternal.With("error", err.Error()).Errorf("检查邮箱失败")
	}
	if exists {
		return nil, errors.ErrConflict.With("email", input.Email).Errorf("邮箱已存在")
	}

	// 检查手机号是否已存在（如果提供了手机号）
	if input.Phone != nil && *input.Phone != "" {
		exists, err = s.orm.User.Query().
			Where(user.PhoneEQ(*input.Phone), user.DeletedAtEQ(0)).
			Exist(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "检查手机号是否存在失败", "error", err)
			return nil, errors.ErrInternal.With("error", err.Error()).Errorf("检查手机号失败")
		}
		if exists {
			return nil, errors.ErrConflict.With("phone", *input.Phone).Errorf("手机号已存在")
		}
	}

	// 生成密码哈希
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		slog.ErrorContext(ctx, "生成密码哈希失败", "error", err)
		return nil, errors.ErrInternal.With("error", err.Error()).Errorf("密码加密失败")
	}

	// 设置默认状态
	status := input.Status
	if status == "" {
		status = types.UserStatusActive
	}

	// 创建用户
	createQuery := s.orm.User.Create().
		SetID(utils.GenerateULID()).
		SetUsername(input.Username).
		SetEmail(input.Email).
		SetPasswordHash(string(passwordHash)).
		SetStatus(user.Status(status)).
		SetForceChangePassword(input.ForceChangePassword).
		SetAllowMultiLogin(input.AllowMultiLogin)

	// 设置可选字段
	if input.RealName != nil {
		createQuery = createQuery.SetRealName(*input.RealName)
	}
	if input.Phone != nil {
		createQuery = createQuery.SetPhone(*input.Phone)
	}
	if input.Department != nil {
		createQuery = createQuery.SetDepartment(*input.Department)
	}
	if input.Position != nil {
		createQuery = createQuery.SetPosition(*input.Position)
	}

	u, err := createQuery.Save(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "创建用户失败", "error", err)
		return nil, errors.ErrInternal.With("error", err.Error()).Errorf("创建用户失败")
	}

	return &types.UserOutput{
		UserInfo: s.toUserInfo(u),
	}, nil
}

// GetUserByID 根据ID获取用户
func (s *MeService) GetUserByID(ctx context.Context, userID string) (*types.UserOutput, error) {
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
		UserInfo: s.toUserInfo(u),
	}, nil
}

// ListUsers 获取用户列表
func (s *MeService) ListUsers(ctx context.Context, input *types.ListUsersInput) (*types.ListUsersOutput, error) {
	// 构建查询条件
	query := s.orm.User.Query()

	// 根据状态筛选
	if input.Status != "" {
		query = query.Where(user.StatusEQ(user.Status(input.Status)))
	}

	// 根据部门筛选
	if input.Department != "" {
		query = query.Where(user.DepartmentEQ(input.Department))
	}

	// 根据岗位筛选
	if input.Position != "" {
		query = query.Where(user.PositionEQ(input.Position))
	}

	// TODO: 根据角色筛选 - 需要使用 UserRole 关联表实现
	// if input.Role != "" {
	//     // 需要通过 UserRole 关联表来实现角色筛选
	// }

	// 根据关键词搜索（用户名、邮箱、真实姓名、手机号）
	if input.Keyword != "" {
		keyword := strings.TrimSpace(input.Keyword)
		query = query.Where(
			user.Or(
				user.UsernameContains(keyword),
				user.EmailContains(keyword),
				user.RealNameContains(keyword),
				user.PhoneContains(keyword),
			),
		)
	}

	// 获取总数
	total, err := query.Count(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "获取用户总数失败", "error", err)
		return nil, errors.ErrInternal.With("error", err.Error()).Errorf("获取用户总数失败")
	}

	// 获取用户列表
	users, err := query.
		Offset(input.PageInput.Offset()).
		Limit(input.PageInput.Limit()).
		Order(ent.Desc(user.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "获取用户列表失败", "error", err)
		return nil, errors.ErrInternal.With("error", err.Error()).Errorf("获取用户列表失败")
	}

	// 转换为输出格式
	userInfos := make([]*types.UserInfo, 0, len(users))
	for _, u := range users {
		userInfos = append(userInfos, s.toUserInfo(u))
	}

	return &types.ListUsersOutput{
		Users:      userInfos,
		PageOutput: types.NewPageOutput(input.PageInput, total),
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
		UserInfo: s.toUserInfo(updatedUser),
	}, nil
}

// DeleteUser 删除用户（逻辑删除）
func (s *MeService) DeleteUser(ctx context.Context, userID string) error {
	// 检查用户是否存在
	exists, err := s.orm.User.Query().
		Where(user.IDEQ(userID)).
		Exist(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "检查用户是否存在失败", "error", err, "user_id", userID)
		return errors.ErrInternal.Wrapf(err, "检查用户失败")
	}
	if !exists {
		slog.WarnContext(ctx, "用户不存在", "user_id", userID)
		return errors.ErrNotFound.With("user_id", userID).Errorf("用户不存在")
	}

	// 执行逻辑删除（使用毫秒级时间戳）
	err = s.orm.User.DeleteOneID(userID).Exec(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "删除用户失败", "error", err, "user_id", userID)
		return errors.ErrInternal.Wrapf(err, "删除用户失败")
	}

	return nil
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

// UpdateUser 更新用户（管理员版本）
func (s *UserService) UpdateUser(ctx context.Context, userID string, input *types.UpdateUserInput) (*types.UserOutput, error) {
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

	// 构建更新查询
	updateQuery := s.orm.User.UpdateOneID(userID)

	// 更新用户名
	if input.Username != nil {
		// 检查用户名是否已被其他用户使用
		exists, err := s.orm.User.Query().
			Where(
				user.UsernameEQ(*input.Username),
				user.DeletedAtEQ(0),
				user.IDNEQ(userID),
			).
			Exist(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "检查用户名是否存在失败", "error", err)
			return nil, errors.ErrInternal.With("error", err.Error()).Errorf("检查用户名失败")
		}
		if exists {
			return nil, errors.ErrConflict.With("username", *input.Username).Errorf("用户名已存在")
		}
		updateQuery = updateQuery.SetUsername(*input.Username)
	}

	// 更新邮箱
	if input.Email != nil {
		// 检查邮箱是否已被其他用户使用
		exists, err := s.orm.User.Query().
			Where(
				user.EmailEQ(*input.Email),
				user.DeletedAtEQ(0),
				user.IDNEQ(userID),
			).
			Exist(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "检查邮箱是否存在失败", "error", err)
			return nil, errors.ErrInternal.With("error", err.Error()).Errorf("检查邮箱失败")
		}
		if exists {
			return nil, errors.ErrConflict.With("email", *input.Email).Errorf("邮箱已存在")
		}
		updateQuery = updateQuery.SetEmail(*input.Email)
	}

	// 更新手机号
	if input.Phone != nil {
		if *input.Phone != "" {
			// 检查手机号是否已被其他用户使用
			exists, err := s.orm.User.Query().
				Where(
					user.PhoneEQ(*input.Phone),
					user.DeletedAtEQ(0),
					user.IDNEQ(userID),
				).
				Exist(ctx)
			if err != nil {
				slog.ErrorContext(ctx, "检查手机号是否存在失败", "error", err)
				return nil, errors.ErrInternal.With("error", err.Error()).Errorf("检查手机号失败")
			}
			if exists {
				return nil, errors.ErrConflict.With("phone", *input.Phone).Errorf("手机号已存在")
			}
		}
		updateQuery = updateQuery.SetPhone(*input.Phone)
	}

	// 更新其他字段
	if input.RealName != nil {
		updateQuery = updateQuery.SetRealName(*input.RealName)
	}
	if input.Department != nil {
		updateQuery = updateQuery.SetDepartment(*input.Department)
	}
	if input.Position != nil {
		updateQuery = updateQuery.SetPosition(*input.Position)
	}
	// TODO: 角色更新 - 需要使用 UserRole 关联表实现
	// if len(input.Roles) > 0 {
	//     // 需要通过 UserRole 关联表来更新角色
	// }
	if input.Status != nil {
		updateQuery = updateQuery.SetStatus(user.Status(*input.Status))
	}
	if input.ForceChangePassword != nil {
		updateQuery = updateQuery.SetForceChangePassword(*input.ForceChangePassword)
	}
	if input.AllowMultiLogin != nil {
		updateQuery = updateQuery.SetAllowMultiLogin(*input.AllowMultiLogin)
	}

	// 执行更新
	updatedUser, err := updateQuery.Save(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "更新用户失败", "error", err, "user_id", userID)
		return nil, errors.ErrInternal.Wrapf(err, "更新用户失败")
	}

	slog.InfoContext(ctx, "用户更新成功", "user_id", userID)

	return &types.UserOutput{
		UserInfo: s.toUserInfo(updatedUser),
	}, nil
}

// ResetPassword 重置用户密码（管理员功能）
func (s *UserService) ResetPassword(ctx context.Context, userID string, input *types.ResetPasswordInput) error {
	// 检查用户是否存在
	exists, err := s.orm.User.Query().
		Where(user.IDEQ(userID)).
		Exist(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "检查用户是否存在失败", "error", err, "user_id", userID)
		return errors.ErrInternal.Wrapf(err, "检查用户失败")
	}
	if !exists {
		slog.WarnContext(ctx, "用户不存在", "user_id", userID)
		return errors.ErrNotFound.With("user_id", userID).Errorf("用户不存在")
	}

	// 生成新密码哈希
	newPasswordHash, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		slog.ErrorContext(ctx, "生成新密码哈希失败", "error", err)
		return errors.ErrInternal.Wrapf(err, "密码加密失败")
	}

	// 更新密码和强制修改密码标志
	updateQuery := s.orm.User.UpdateOneID(userID).
		SetPasswordHash(string(newPasswordHash)).
		SetForceChangePassword(input.ForceChangePassword)

	err = updateQuery.Exec(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "重置密码失败", "error", err, "user_id", userID)
		return errors.ErrInternal.Wrapf(err, "重置密码失败")
	}

	slog.InfoContext(ctx, "重置密码成功", "user_id", userID)
	return nil
}

// SetUserStatus 设置用户状态
func (s *UserService) SetUserStatus(ctx context.Context, userID string, input *types.SetUserStatusInput) error {
	// 检查用户是否存在
	exists, err := s.orm.User.Query().
		Where(user.IDEQ(userID)).
		Exist(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "检查用户是否存在失败", "error", err, "user_id", userID)
		return errors.ErrInternal.Wrapf(err, "检查用户失败")
	}
	if !exists {
		slog.WarnContext(ctx, "用户不存在", "user_id", userID)
		return errors.ErrNotFound.With("user_id", userID).Errorf("用户不存在")
	}

	// 更新用户状态
	err = s.orm.User.UpdateOneID(userID).
		SetStatus(user.Status(input.Status)).
		Exec(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "设置用户状态失败", "error", err, "user_id", userID)
		return errors.ErrInternal.Wrapf(err, "设置用户状态失败")
	}

	slog.InfoContext(ctx, "设置用户状态成功", "user_id", userID, "status", input.Status)
	return nil
}

// BatchUpdateStatus 批量更新用户状态
func (s *UserService) BatchUpdateStatus(ctx context.Context, input *types.BatchUpdateStatusInput) error {
	// 检查用户是否都存在
	count, err := s.orm.User.Query().
		Where(user.IDIn(input.UserIDs...)).
		Count(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "检查用户是否存在失败", "error", err)
		return errors.ErrInternal.Wrapf(err, "检查用户失败")
	}
	if count != len(input.UserIDs) {
		return errors.ErrBadRequest.Errorf("部分用户不存在")
	}

	// 批量更新状态
	_, err = s.orm.User.Update().
		Where(user.IDIn(input.UserIDs...)).
		SetStatus(user.Status(input.Status)).
		Save(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "批量更新用户状态失败", "error", err)
		return errors.ErrInternal.Wrapf(err, "批量更新用户状态失败")
	}

	slog.InfoContext(ctx, "批量更新用户状态成功", "user_count", len(input.UserIDs), "status", input.Status)
	return nil
}

// BatchDeleteUsers 批量删除用户
func (s *UserService) BatchDeleteUsers(ctx context.Context, input *types.BatchOperationInput) error {
	// 检查用户是否都存在
	count, err := s.orm.User.Query().
		Where(user.IDIn(input.UserIDs...)).
		Count(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "检查用户是否存在失败", "error", err)
		return errors.ErrInternal.Wrapf(err, "检查用户失败")
	}
	if count != len(input.UserIDs) {
		return errors.ErrBadRequest.Errorf("部分用户不存在")
	}

	// 批量逻辑删除
	_, err = s.orm.User.Delete().
		Where(user.IDIn(input.UserIDs...)).
		Exec(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "批量删除用户失败", "error", err)
		return errors.ErrInternal.Wrapf(err, "批量删除用户失败")
	}

	slog.InfoContext(ctx, "批量删除用户成功", "user_count", len(input.UserIDs))
	return nil
}

// GetUserStats 获取用户统计信息
func (s *UserService) GetUserStats(ctx context.Context) (*types.UserStatsOutput, error) {
	// 总用户数
	totalUsers, err := s.orm.User.Query().
		Count(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "获取用户总数失败", "error", err)
		return nil, errors.ErrInternal.Wrapf(err, "获取用户总数失败")
	}

	// 各状态用户数
	activeUsers, err := s.orm.User.Query().
		Where(user.StatusEQ(user.StatusActive)).
		Count(ctx)
	if err != nil {
		return nil, errors.ErrInternal.Wrapf(err, "获取活跃用户数失败")
	}

	inactiveUsers, err := s.orm.User.Query().
		Where(user.StatusEQ(user.StatusInactive)).
		Count(ctx)
	if err != nil {
		return nil, errors.ErrInternal.Wrapf(err, "获取非活跃用户数失败")
	}

	suspendedUsers, err := s.orm.User.Query().
		Where(user.StatusEQ(user.StatusSuspended)).
		Count(ctx)
	if err != nil {
		return nil, errors.ErrInternal.Wrapf(err, "获取停用用户数失败")
	}

	// 部门统计
	var departmentStats []types.DepartmentStat
	users, err := s.orm.User.Query().
		All(ctx)
	if err != nil {
		return nil, errors.ErrInternal.Wrapf(err, "获取用户列表失败")
	}

	// 统计各部门用户数
	deptCount := make(map[string]int64)
	for _, u := range users {
		dept := u.Department
		if dept == "" {
			dept = "未分配"
		}
		deptCount[dept]++
	}

	for dept, count := range deptCount {
		departmentStats = append(departmentStats, types.DepartmentStat{
			Department: dept,
			UserCount:  count,
		})
	}

	return &types.UserStatsOutput{
		TotalUsers:     int64(totalUsers),
		ActiveUsers:    int64(activeUsers),
		InactiveUsers:  int64(inactiveUsers),
		SuspendedUsers: int64(suspendedUsers),
		StatusBreakdown: map[string]int64{
			types.UserStatusActive:    int64(activeUsers),
			types.UserStatusInactive:  int64(inactiveUsers),
			types.UserStatusSuspended: int64(suspendedUsers),
		},
		DepartmentStats: departmentStats,
	}, nil
}
