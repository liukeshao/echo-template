package services

import (
	"context"
	"log/slog"
	"math"

	"entgo.io/ent/dialect/sql"
	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/ent/permission"
	"github.com/liukeshao/echo-template/pkg/errors"
	"github.com/liukeshao/echo-template/pkg/types"
	"github.com/liukeshao/echo-template/pkg/utils"
)

// PermissionService 权限服务
type PermissionService struct {
	orm *ent.Client
}

// NewPermissionService 创建权限服务
func NewPermissionService(orm *ent.Client) *PermissionService {
	return &PermissionService{
		orm: orm,
	}
}

// CreatePermission 创建权限
func (s *PermissionService) CreatePermission(ctx context.Context, input *types.CreatePermissionInput) (*types.PermissionOutput, error) {
	// 检查权限代码是否已存在
	exists, err := s.orm.Permission.Query().
		Where(permission.CodeEQ(input.Code), permission.DeletedAtEQ(0)).
		Exist(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "检查权限代码失败", "error", err)
		return nil, errors.ErrInternal("检查权限代码失败").With("error", err.Error())
	}
	if exists {
		return nil, errors.ErrConflict("权限代码已存在").With("code", input.Code)
	}

	// 检查资源和操作组合是否已存在
	exists, err = s.orm.Permission.Query().
		Where(
			permission.ResourceEQ(input.Resource),
			permission.ActionEQ(input.Action),
			permission.DeletedAtEQ(0),
		).
		Exist(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "检查资源操作组合失败", "error", err)
		return nil, errors.ErrInternal("检查资源操作组合失败").With("error", err.Error())
	}
	if exists {
		return nil, errors.ErrConflict("该资源的操作权限已存在").With("resource", input.Resource, "action", input.Action)
	}

	// 创建权限
	permissionEntity, err := s.orm.Permission.Create().
		SetID(utils.GenerateULID()).
		SetName(input.Name).
		SetCode(input.Code).
		SetResource(input.Resource).
		SetAction(input.Action).
		SetNillableDescription(&input.Description).
		SetStatus(permission.Status(input.Status)).
		SetSortOrder(input.SortOrder).
		Save(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "创建权限失败", "error", err)
		return nil, errors.ErrInternal("创建权限失败").With("error", err.Error())
	}

	slog.InfoContext(ctx, "权限创建成功", "permission_id", permissionEntity.ID, "name", permissionEntity.Name)

	return s.toPermissionOutput(permissionEntity), nil
}

// UpdatePermission 更新权限
func (s *PermissionService) UpdatePermission(ctx context.Context, id string, input *types.UpdatePermissionInput) (*types.PermissionOutput, error) {
	// 检查权限是否存在
	permissionEntity, err := s.orm.Permission.Query().
		Where(permission.IDEQ(id), permission.DeletedAtEQ(0)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errors.ErrNotFound("权限不存在").With("permission_id", id)
		}
		slog.ErrorContext(ctx, "获取权限失败", "error", err)
		return nil, errors.ErrInternal("获取权限失败").With("error", err.Error())
	}

	// 检查是否为系统权限
	if permissionEntity.IsSystem {
		return nil, errors.ErrForbidden("系统权限不允许修改").With("permission_id", id)
	}

	// 构建更新查询
	update := s.orm.Permission.UpdateOneID(id)

	if input.Name != nil {
		update = update.SetName(*input.Name)
	}

	if input.Description != nil {
		update = update.SetNillableDescription(input.Description)
	}

	if input.Status != nil {
		update = update.SetStatus(permission.Status(*input.Status))
	}

	if input.SortOrder != nil {
		update = update.SetSortOrder(*input.SortOrder)
	}

	// 执行更新
	permissionEntity, err = update.Save(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "更新权限失败", "error", err)
		return nil, errors.ErrInternal("更新权限失败").With("error", err.Error())
	}

	slog.InfoContext(ctx, "权限更新成功", "permission_id", id)

	return s.toPermissionOutput(permissionEntity), nil
}

// DeletePermission 删除权限
func (s *PermissionService) DeletePermission(ctx context.Context, id string) error {
	// 检查权限是否存在
	permissionEntity, err := s.orm.Permission.Query().
		Where(permission.IDEQ(id), permission.DeletedAtEQ(0)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return errors.ErrNotFound("权限不存在").With("permission_id", id)
		}
		slog.ErrorContext(ctx, "获取权限失败", "error", err)
		return errors.ErrInternal("获取权限失败").With("error", err.Error())
	}

	// 检查是否为系统权限
	if permissionEntity.IsSystem {
		return errors.ErrForbidden("系统权限不允许删除").With("permission_id", id)
	}

	// 检查是否有角色关联此权限
	roleCount, err := s.orm.Permission.Query().
		Where(permission.IDEQ(id)).
		QueryRoles().
		Count(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "检查权限关联角色失败", "error", err)
		return errors.ErrInternal("检查权限关联角色失败").With("error", err.Error())
	}
	if roleCount > 0 {
		return errors.ErrConflict("权限已被角色使用，无法删除").With("permission_id", id, "role_count", roleCount)
	}

	// 删除权限（软删除）
	err = s.orm.Permission.DeleteOneID(id).Exec(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "删除权限失败", "error", err)
		return errors.ErrInternal("删除权限失败").With("error", err.Error())
	}

	slog.InfoContext(ctx, "权限删除成功", "permission_id", id)

	return nil
}

// GetPermission 获取权限详情
func (s *PermissionService) GetPermission(ctx context.Context, id string) (*types.PermissionOutput, error) {
	permissionEntity, err := s.orm.Permission.Query().
		Where(permission.IDEQ(id), permission.DeletedAtEQ(0)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errors.ErrNotFound("权限不存在").With("permission_id", id)
		}
		slog.ErrorContext(ctx, "获取权限失败", "error", err)
		return nil, errors.ErrInternal("获取权限失败").With("error", err.Error())
	}

	return s.toPermissionOutput(permissionEntity), nil
}

// ListPermissions 获取权限列表
func (s *PermissionService) ListPermissions(ctx context.Context, input *types.ListPermissionsInput) (*types.ListPermissionsOutput, error) {
	query := s.orm.Permission.Query().Where(permission.DeletedAtEQ(0))

	// 资源类型过滤
	if input.Resource != "" {
		query = query.Where(permission.ResourceEQ(input.Resource))
	}

	// 操作类型过滤
	if input.Action != "" {
		query = query.Where(permission.ActionEQ(input.Action))
	}

	// 状态过滤
	if input.Status != "" {
		query = query.Where(permission.StatusEQ(permission.Status(input.Status)))
	}

	// 搜索关键词过滤
	if input.Search != "" {
		query = query.Where(
			permission.Or(
				permission.NameContains(input.Search),
				permission.CodeContains(input.Search),
				permission.ResourceContains(input.Search),
				permission.ActionContains(input.Search),
				permission.DescriptionContains(input.Search),
			),
		)
	}

	// 获取总数
	total, err := query.Count(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "获取权限总数失败", "error", err)
		return nil, errors.ErrInternal("获取权限总数失败").With("error", err.Error())
	}

	// 分页查询
	permissions, err := query.
		Order(ent.Asc(permission.FieldResource), ent.Asc(permission.FieldSortOrder), ent.Asc(permission.FieldCreatedAt)).
		Offset((input.Page - 1) * input.PageSize).
		Limit(input.PageSize).
		All(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "获取权限列表失败", "error", err)
		return nil, errors.ErrInternal("获取权限列表失败").With("error", err.Error())
	}

	// 转换输出格式
	list := make([]*types.PermissionOutput, len(permissions))
	for i, p := range permissions {
		list[i] = s.toPermissionOutput(p)
	}

	// 计算总页数
	totalPages := int(math.Ceil(float64(total) / float64(input.PageSize)))

	return &types.ListPermissionsOutput{
		Permissions: list,
		Total:       int64(total),
		Page:        input.Page,
		PageSize:    input.PageSize,
		TotalPages:  totalPages,
	}, nil
}

// ListPermissionsByResource 按资源分组获取权限列表
func (s *PermissionService) ListPermissionsByResource(ctx context.Context) ([]*types.PermissionGroupOutput, error) {

	// 获取所有活跃权限
	permissions, err := s.orm.Permission.Query().
		Where(
			permission.DeletedAtEQ(0),
			permission.StatusEQ(permission.StatusActive),
		).
		Order(ent.Asc(permission.FieldResource), ent.Asc(permission.FieldSortOrder)).
		All(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "获取权限列表失败", "error", err)
		return nil, errors.ErrInternal("获取权限列表失败").With("error", err.Error())
	}

	// 按资源分组
	resourceMap := make(map[string][]*types.PermissionOutput)
	for _, p := range permissions {
		permOutput := s.toPermissionOutput(p)
		resourceMap[p.Resource] = append(resourceMap[p.Resource], permOutput)
	}

	// 转换为输出格式
	result := make([]*types.PermissionGroupOutput, 0, len(resourceMap))
	for resource, perms := range resourceMap {
		result = append(result, &types.PermissionGroupOutput{
			Resource:    resource,
			Permissions: perms,
		})
	}

	return result, nil
}

// AssignPermissions 分配权限给角色
func (s *PermissionService) AssignPermissions(ctx context.Context, input *types.AssignPermissionInput) error {

	// 检查角色是否存在且状态为活跃
	roleExists, err := s.orm.Role.Query().
		Where(func(selector *sql.Selector) {
			selector.Where(sql.And(
				sql.EQ("id", input.RoleID),
				sql.EQ("status", "active"),
				sql.EQ("deleted_at", 0),
			))
		}).
		Exist(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "检查角色失败", "error", err)
		return errors.ErrInternal("检查角色失败").With("error", err.Error())
	}
	if !roleExists {
		return errors.ErrNotFound("角色不存在或状态不活跃").With("role_id", input.RoleID)
	}

	// 检查权限是否存在且状态为活跃
	permissionCount, err := s.orm.Permission.Query().
		Where(
			permission.IDIn(input.PermissionIDs...),
			permission.StatusEQ(permission.StatusActive),
			permission.DeletedAtEQ(0),
		).
		Count(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "检查权限失败", "error", err)
		return errors.ErrInternal("检查权限失败").With("error", err.Error())
	}
	if permissionCount != len(input.PermissionIDs) {
		return errors.ErrBadRequest("存在无效或非活跃的权限").With("permission_ids", input.PermissionIDs)
	}

	// 更新角色权限关联
	err = s.orm.Role.UpdateOneID(input.RoleID).
		ClearPermissions().
		AddPermissionIDs(input.PermissionIDs...).
		Exec(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "分配权限失败", "error", err)
		return errors.ErrInternal("分配权限失败").With("error", err.Error())
	}

	slog.InfoContext(ctx, "权限分配成功", "role_id", input.RoleID, "permission_count", len(input.PermissionIDs))

	return nil
}

// GetRolePermissions 获取角色权限列表
func (s *PermissionService) GetRolePermissions(ctx context.Context, roleID string) (*types.RolePermissionOutput, error) {

	roleEntity, err := s.orm.Role.Query().
		Where(func(selector *sql.Selector) {
			selector.Where(sql.And(
				sql.EQ("id", roleID),
				sql.EQ("deleted_at", 0),
			))
		}).
		WithPermissions(func(pq *ent.PermissionQuery) {
			pq.Where(permission.DeletedAtEQ(0))
		}).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errors.ErrNotFound("角色不存在").With("role_id", roleID)
		}
		slog.ErrorContext(ctx, "获取角色权限失败", "error", err)
		return nil, errors.ErrInternal("获取角色权限失败").With("error", err.Error())
	}

	permissions := make([]*types.PermissionOutput, len(roleEntity.Edges.Permissions))
	for i, p := range roleEntity.Edges.Permissions {
		permissions[i] = s.toPermissionOutput(p)
	}

	return &types.RolePermissionOutput{
		RoleID:      roleEntity.ID,
		RoleName:    roleEntity.Name,
		Permissions: permissions,
	}, nil
}

// toPermissionOutput 转换为权限输出格式
func (s *PermissionService) toPermissionOutput(p *ent.Permission) *types.PermissionOutput {
	return &types.PermissionOutput{
		ID:          p.ID,
		Name:        p.Name,
		Code:        p.Code,
		Resource:    p.Resource,
		Action:      p.Action,
		Description: p.Description,
		Status:      string(p.Status),
		IsSystem:    p.IsSystem,
		SortOrder:   p.SortOrder,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}
