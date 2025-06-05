package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/ent/permission"
	"github.com/liukeshao/echo-template/ent/role"
	"github.com/liukeshao/echo-template/ent/userrole"
	"github.com/liukeshao/echo-template/pkg/errors"
	"github.com/liukeshao/echo-template/pkg/types"
	"github.com/liukeshao/echo-template/pkg/utils"
)

// RoleService 角色服务
type RoleService struct {
	orm *ent.Client
}

// NewRoleService 创建角色服务
func NewRoleService(orm *ent.Client) *RoleService {
	return &RoleService{
		orm: orm,
	}
}

// CreateRole 创建角色
func (s *RoleService) CreateRole(ctx context.Context, input *types.CreateRoleInput) (*types.RoleOutput, error) {
	slog.InfoContext(ctx, "开始创建角色", "name", input.Name, "code", input.Code)

	// 检查角色代码是否已存在
	exists, err := s.orm.Role.Query().
		Where(role.CodeEQ(input.Code), role.DeletedAtEQ(0)).
		Exist(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "检查角色代码失败", "error", err)
		return nil, errors.InternalError("检查角色代码失败").With("error", err.Error())
	}
	if exists {
		return nil, errors.ConflictError("角色代码已存在").With("code", input.Code)
	}

	// 检查角色名称是否已存在
	exists, err = s.orm.Role.Query().
		Where(role.NameEQ(input.Name), role.DeletedAtEQ(0)).
		Exist(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "检查角色名称失败", "error", err)
		return nil, errors.InternalError("检查角色名称失败").With("error", err.Error())
	}
	if exists {
		return nil, errors.ConflictError("角色名称已存在").With("name", input.Name)
	}

	// 创建角色
	roleEntity, err := s.orm.Role.Create().
		SetID(utils.GenerateULID()).
		SetName(input.Name).
		SetCode(input.Code).
		SetNillableDescription(&input.Description).
		SetStatus(role.Status(input.Status)).
		SetSortOrder(input.SortOrder).
		Save(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "创建角色失败", "error", err)
		return nil, errors.InternalError("创建角色失败").With("error", err.Error())
	}

	slog.InfoContext(ctx, "角色创建成功", "role_id", roleEntity.ID, "name", roleEntity.Name)

	return s.toRoleOutput(roleEntity, nil), nil
}

// UpdateRole 更新角色
func (s *RoleService) UpdateRole(ctx context.Context, id string, input *types.UpdateRoleInput) (*types.RoleOutput, error) {
	slog.InfoContext(ctx, "开始更新角色", "role_id", id)

	// 检查角色是否存在
	roleEntity, err := s.orm.Role.Query().
		Where(role.IDEQ(id), role.DeletedAtEQ(0)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errors.NotFoundError("角色不存在").With("role_id", id)
		}
		slog.ErrorContext(ctx, "获取角色失败", "error", err)
		return nil, errors.InternalError("获取角色失败").With("error", err.Error())
	}

	// 检查是否为系统角色
	if roleEntity.IsSystem {
		return nil, errors.ForbiddenError("系统角色不允许修改").With("role_id", id)
	}

	// 构建更新查询
	update := s.orm.Role.UpdateOneID(id)

	// 检查角色名称是否已存在（排除自己）
	if input.Name != nil {
		exists, err := s.orm.Role.Query().
			Where(
				role.NameEQ(*input.Name),
				role.IDNEQ(id),
				role.DeletedAtEQ(0),
			).
			Exist(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "检查角色名称失败", "error", err)
			return nil, errors.InternalError("检查角色名称失败").With("error", err.Error())
		}
		if exists {
			return nil, errors.ConflictError("角色名称已存在").With("name", *input.Name)
		}
		update = update.SetName(*input.Name)
	}

	if input.Description != nil {
		update = update.SetNillableDescription(input.Description)
	}

	if input.Status != nil {
		update = update.SetStatus(role.Status(*input.Status))
	}

	if input.SortOrder != nil {
		update = update.SetSortOrder(*input.SortOrder)
	}

	// 执行更新
	roleEntity, err = update.Save(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "更新角色失败", "error", err)
		return nil, errors.InternalError("更新角色失败").With("error", err.Error())
	}

	// 获取权限列表
	permissions, err := s.getRolePermissions(ctx, id)
	if err != nil {
		return nil, err
	}

	slog.InfoContext(ctx, "角色更新成功", "role_id", id)

	return s.toRoleOutput(roleEntity, permissions), nil
}

// DeleteRole 删除角色
func (s *RoleService) DeleteRole(ctx context.Context, id string) error {
	slog.InfoContext(ctx, "开始删除角色", "role_id", id)

	// 检查角色是否存在
	roleEntity, err := s.orm.Role.Query().
		Where(role.IDEQ(id), role.DeletedAtEQ(0)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return errors.NotFoundError("角色不存在").With("role_id", id)
		}
		slog.ErrorContext(ctx, "获取角色失败", "error", err)
		return errors.InternalError("获取角色失败").With("error", err.Error())
	}

	// 检查是否为系统角色
	if roleEntity.IsSystem {
		return errors.ForbiddenError("系统角色不允许删除").With("role_id", id)
	}

	// 检查是否有用户关联此角色
	userCount, err := s.orm.UserRole.Query().
		Where(userrole.RoleIDEQ(id), userrole.DeletedAtEQ(0)).
		Count(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "检查角色关联用户失败", "error", err)
		return errors.InternalError("检查角色关联用户失败").With("error", err.Error())
	}
	if userCount > 0 {
		return errors.ConflictError(fmt.Sprintf("角色已被 %d 个用户使用，无法删除", userCount)).With("role_id", id, "user_count", userCount)
	}

	// 删除角色（软删除）
	err = s.orm.Role.DeleteOneID(id).Exec(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "删除角色失败", "error", err)
		return errors.InternalError("删除角色失败").With("error", err.Error())
	}

	slog.InfoContext(ctx, "角色删除成功", "role_id", id)

	return nil
}

// GetRole 获取角色详情
func (s *RoleService) GetRole(ctx context.Context, id string) (*types.RoleOutput, error) {
	slog.InfoContext(ctx, "获取角色详情", "role_id", id)

	roleEntity, err := s.orm.Role.Query().
		Where(role.IDEQ(id), role.DeletedAtEQ(0)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errors.NotFoundError("角色不存在").With("role_id", id)
		}
		slog.ErrorContext(ctx, "获取角色失败", "error", err)
		return nil, errors.InternalError("获取角色失败").With("error", err.Error())
	}

	// 获取权限列表
	permissions, err := s.getRolePermissions(ctx, id)
	if err != nil {
		return nil, err
	}

	return s.toRoleOutput(roleEntity, permissions), nil
}

// ListRoles 获取角色列表
func (s *RoleService) ListRoles(ctx context.Context, input *types.ListRolesInput) (*types.ListRolesOutput, error) {
	slog.InfoContext(ctx, "获取角色列表", "page", input.Page, "page_size", input.PageSize, "status", input.Status, "search", input.Search)

	query := s.orm.Role.Query().Where(role.DeletedAtEQ(0))

	// 状态过滤
	if input.Status != "" {
		query = query.Where(role.StatusEQ(role.Status(input.Status)))
	}

	// 搜索关键词过滤
	if input.Search != "" {
		query = query.Where(
			role.Or(
				role.NameContains(input.Search),
				role.CodeContains(input.Search),
				role.DescriptionContains(input.Search),
			),
		)
	}

	// 获取总数
	total, err := query.Count(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "获取角色总数失败", "error", err)
		return nil, errors.InternalError("获取角色总数失败").With("error", err.Error())
	}

	// 分页查询
	roles, err := query.
		Order(ent.Asc(role.FieldSortOrder), ent.Asc(role.FieldCreatedAt)).
		Offset((input.Page - 1) * input.PageSize).
		Limit(input.PageSize).
		All(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "获取角色列表失败", "error", err)
		return nil, errors.InternalError("获取角色列表失败").With("error", err.Error())
	}

	// 转换输出格式
	list := make([]*types.RoleOutput, len(roles))
	for i, r := range roles {
		// 获取每个角色的权限
		permissions, err := s.getRolePermissions(ctx, r.ID)
		if err != nil {
			return nil, err
		}
		list[i] = s.toRoleOutput(r, permissions)
	}

	return &types.ListRolesOutput{
		List:  list,
		Total: int64(total),
		Page:  input.Page,
		Size:  input.PageSize,
	}, nil
}

// AssignRoles 分配角色给用户
func (s *RoleService) AssignRoles(ctx context.Context, input *types.AssignRoleInput, granterID string) error {
	slog.InfoContext(ctx, "开始分配角色", "user_id", input.UserID, "role_ids", input.RoleIDs, "granter_id", granterID)

	// 检查用户是否存在
	userExists, err := s.orm.User.Query().
		Where(func(selector *sql.Selector) {
			selector.Where(sql.And(
				sql.EQ("id", input.UserID),
				sql.EQ("deleted_at", 0),
			))
		}).
		Exist(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "检查用户失败", "error", err)
		return errors.InternalError("检查用户失败").With("error", err.Error())
	}
	if !userExists {
		return errors.NotFoundError("用户不存在").With("user_id", input.UserID)
	}

	// 检查角色是否存在且状态为活跃
	roleCount, err := s.orm.Role.Query().
		Where(
			role.IDIn(input.RoleIDs...),
			role.StatusEQ(role.StatusActive),
			role.DeletedAtEQ(0),
		).
		Count(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "检查角色失败", "error", err)
		return errors.InternalError("检查角色失败").With("error", err.Error())
	}
	if roleCount != len(input.RoleIDs) {
		return errors.BadRequestError("存在无效或非活跃的角色").With("role_ids", input.RoleIDs)
	}

	// 开始事务
	tx, err := s.orm.Tx(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "开始事务失败", "error", err)
		return errors.InternalError("开始事务失败").With("error", err.Error())
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// 删除用户现有的角色分配（如果有的话）
	_, err = tx.UserRole.Delete().
		Where(userrole.UserIDEQ(input.UserID)).
		Exec(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "删除现有角色分配失败", "error", err)
		return errors.InternalError("删除现有角色分配失败").With("error", err.Error())
	}

	// 创建新的角色分配
	bulk := make([]*ent.UserRoleCreate, len(input.RoleIDs))
	for i, roleID := range input.RoleIDs {
		bulk[i] = tx.UserRole.Create().
			SetID(utils.GenerateULID()).
			SetUserID(input.UserID).
			SetRoleID(roleID).
			SetNillableGrantedBy(&granterID).
			SetNillableExpiresAt(input.ExpiresAt).
			SetNillableRemark(&input.Remark)
	}

	_, err = tx.UserRole.CreateBulk(bulk...).Save(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "创建角色分配失败", "error", err)
		return errors.InternalError("创建角色分配失败").With("error", err.Error())
	}

	// 提交事务
	err = tx.Commit()
	if err != nil {
		slog.ErrorContext(ctx, "提交事务失败", "error", err)
		return errors.InternalError("提交事务失败").With("error", err.Error())
	}

	slog.InfoContext(ctx, "角色分配成功", "user_id", input.UserID, "role_count", len(input.RoleIDs))

	return nil
}

// RevokeRoles 撤销用户角色
func (s *RoleService) RevokeRoles(ctx context.Context, input *types.RevokeRoleInput) error {
	slog.InfoContext(ctx, "开始撤销用户角色", "user_id", input.UserID, "role_ids", input.RoleIDs)

	// 检查用户角色关联是否存在
	count, err := s.orm.UserRole.Query().
		Where(
			userrole.UserIDEQ(input.UserID),
			userrole.RoleIDIn(input.RoleIDs...),
			userrole.DeletedAtEQ(0),
		).
		Count(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "检查用户角色关联失败", "error", err)
		return errors.InternalError("检查用户角色关联失败").With("error", err.Error())
	}
	if count == 0 {
		return errors.NotFoundError("用户角色关联不存在").With("user_id", input.UserID, "role_ids", input.RoleIDs)
	}

	// 删除用户角色关联
	_, err = s.orm.UserRole.Delete().
		Where(
			userrole.UserIDEQ(input.UserID),
			userrole.RoleIDIn(input.RoleIDs...),
		).
		Exec(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "撤销用户角色失败", "error", err)
		return errors.InternalError("撤销用户角色失败").With("error", err.Error())
	}

	slog.InfoContext(ctx, "用户角色撤销成功", "user_id", input.UserID, "revoked_count", count)

	return nil
}

// GetUserRoles 获取用户角色列表
func (s *RoleService) GetUserRoles(ctx context.Context, userID string) ([]*types.UserRoleOutput, error) {
	slog.InfoContext(ctx, "获取用户角色列表", "user_id", userID)

	userRoles, err := s.orm.UserRole.Query().
		Where(userrole.UserIDEQ(userID), userrole.DeletedAtEQ(0)).
		WithRole().
		All(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "获取用户角色失败", "error", err)
		return nil, errors.InternalError("获取用户角色失败").With("error", err.Error())
	}

	result := make([]*types.UserRoleOutput, len(userRoles))
	for i, ur := range userRoles {
		result[i] = &types.UserRoleOutput{
			ID:        ur.ID,
			UserID:    ur.UserID,
			RoleID:    ur.RoleID,
			RoleName:  ur.Edges.Role.Name,
			RoleCode:  ur.Edges.Role.Code,
			GrantedBy: ur.GrantedBy,
			GrantedAt: ur.GrantedAt,
			ExpiresAt: ur.ExpiresAt,
			Status:    string(ur.Status),
			Remark:    ur.Remark,
		}
	}

	return result, nil
}

// CheckUserPermission 检查用户是否具有指定权限
func (s *RoleService) CheckUserPermission(ctx context.Context, userID, permissionCode string) (bool, error) {
	// 查询用户活跃角色的权限
	count, err := s.orm.UserRole.Query().
		Where(
			userrole.UserIDEQ(userID),
			userrole.StatusEQ(userrole.StatusActive),
			userrole.DeletedAtEQ(0),
			func(selector *sql.Selector) {
				// 检查过期时间
				selector.Where(sql.Or(
					sql.IsNull(userrole.FieldExpiresAt),
					sql.GT(userrole.FieldExpiresAt, time.Now()),
				))
			},
		).
		QueryRole().
		Where(role.StatusEQ(role.StatusActive), role.DeletedAtEQ(0)).
		QueryPermissions().
		Where(
			permission.CodeEQ(permissionCode),
			permission.StatusEQ(permission.StatusActive),
			permission.DeletedAtEQ(0),
		).
		Count(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "检查用户权限失败", "error", err)
		return false, errors.InternalError("检查用户权限失败").With("error", err.Error())
	}

	return count > 0, nil
}

// GetUserPermissions 获取用户所有权限
func (s *RoleService) GetUserPermissions(ctx context.Context, userID string) ([]string, error) {
	permissions, err := s.orm.UserRole.Query().
		Where(
			userrole.UserIDEQ(userID),
			userrole.StatusEQ(userrole.StatusActive),
			userrole.DeletedAtEQ(0),
			func(selector *sql.Selector) {
				// 检查过期时间
				selector.Where(sql.Or(
					sql.IsNull(userrole.FieldExpiresAt),
					sql.GT(userrole.FieldExpiresAt, time.Now()),
				))
			},
		).
		QueryRole().
		Where(role.StatusEQ(role.StatusActive), role.DeletedAtEQ(0)).
		QueryPermissions().
		Where(permission.StatusEQ(permission.StatusActive), permission.DeletedAtEQ(0)).
		All(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "获取用户权限失败", "error", err)
		return nil, errors.InternalError("获取用户权限失败").With("error", err.Error())
	}

	codes := make([]string, len(permissions))
	for i, p := range permissions {
		codes[i] = p.Code
	}

	return codes, nil
}

// getRolePermissions 获取角色权限代码列表
func (s *RoleService) getRolePermissions(ctx context.Context, roleID string) ([]string, error) {
	permissions, err := s.orm.Role.Query().
		Where(role.IDEQ(roleID), role.DeletedAtEQ(0)).
		QueryPermissions().
		Where(permission.StatusEQ(permission.StatusActive), permission.DeletedAtEQ(0)).
		All(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "获取角色权限失败", "error", err)
		return nil, errors.InternalError("获取角色权限失败").With("error", err.Error())
	}

	codes := make([]string, len(permissions))
	for i, p := range permissions {
		codes[i] = p.Code
	}

	return codes, nil
}

// toRoleOutput 转换为角色输出格式
func (s *RoleService) toRoleOutput(r *ent.Role, permissions []string) *types.RoleOutput {
	if permissions == nil {
		permissions = []string{}
	}

	return &types.RoleOutput{
		ID:          r.ID,
		Name:        r.Name,
		Code:        r.Code,
		Description: r.Description,
		Status:      string(r.Status),
		IsSystem:    r.IsSystem,
		SortOrder:   r.SortOrder,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
		Permissions: permissions,
	}
}
