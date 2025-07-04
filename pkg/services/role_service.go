package services

import (
	"context"
	"fmt"
	"time"

	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/ent/role"
	"github.com/liukeshao/echo-template/ent/rolemenu"
	"github.com/liukeshao/echo-template/ent/user"
	"github.com/liukeshao/echo-template/ent/userrole"
	"github.com/liukeshao/echo-template/pkg/errors"
	"github.com/liukeshao/echo-template/pkg/types"
	"github.com/liukeshao/echo-template/pkg/utils"
)

// RoleService 角色服务
type RoleService struct {
	orm *ent.Client
}

// NewRoleService 创建角色服务实例
func NewRoleService(orm *ent.Client) *RoleService {
	return &RoleService{
		orm: orm,
	}
}

// Create 创建角色
func (s *RoleService) Create(ctx context.Context, in *types.CreateRoleInput) (*types.RoleOutput, error) {
	// 检查角色名称是否已存在
	exists, err := s.orm.Role.Query().
		Where(role.NameEQ(in.Name), role.DeletedAtEQ(0)).
		Exist(ctx)
	if err != nil {
		return nil, errors.ErrDatabase.Wrapf(err, "检查角色名称失败")
	}
	if exists {
		return nil, errors.ErrConflict.Errorf("角色名称已存在")
	}

	// 检查角色编码是否已存在
	exists, err = s.orm.Role.Query().
		Where(role.CodeEQ(in.Code), role.DeletedAtEQ(0)).
		Exist(ctx)
	if err != nil {
		return nil, errors.ErrDatabase.Wrapf(err, "检查角色编码失败")
	}
	if exists {
		return nil, errors.ErrConflict.Errorf("角色编码已存在")
	}

	// 生成ID
	id := utils.GenerateULID()

	// 开始事务
	tx, err := s.orm.Tx(ctx)
	if err != nil {
		return nil, errors.ErrDatabase.Wrapf(err, "启动事务失败")
	}
	defer tx.Rollback()

	// 设置默认值
	status := role.StatusEnabled
	if in.Status != "" {
		status = role.Status(in.Status)
	}
	dataScope := role.DataScopeAll
	if in.DataScope != "" {
		dataScope = role.DataScope(in.DataScope)
	}

	// 创建角色
	roleEntity, err := tx.Role.Create().
		SetID(id).
		SetName(in.Name).
		SetCode(in.Code).
		SetNillableDescription(in.Description).
		SetStatus(status).
		SetDataScope(dataScope).
		SetDeptIds(in.DeptIds).
		SetSortOrder(in.SortOrder).
		SetNillableRemark(in.Remark).
		Save(ctx)
	if err != nil {
		return nil, errors.ErrDatabase.Wrapf(err, "创建角色失败")
	}

	// 分配菜单权限
	if len(in.MenuIds) > 0 {
		err = s.assignMenusInTx(ctx, tx, id, in.MenuIds)
		if err != nil {
			return nil, err
		}
	}

	// 提交事务
	if err = tx.Commit(); err != nil {
		return nil, errors.ErrDatabase.Wrapf(err, "提交事务失败")
	}

	return s.convertToRoleOutput(roleEntity), nil
}

// Update 更新角色
func (s *RoleService) Update(ctx context.Context, id string, in *types.UpdateRoleInput) (*types.RoleOutput, error) {
	// 查询角色
	roleEntity, err := s.orm.Role.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errors.ErrNotFound.Errorf("角色不存在")
		}
		return nil, errors.ErrDatabase.Wrapf(err, "查询角色失败")
	}

	// 检查是否为内置角色
	if roleEntity.IsBuiltin {
		return nil, errors.ErrBusinessLogic.Errorf("系统内置角色不允许修改")
	}

	// 检查角色名称是否已存在（排除自己）
	if in.Name != nil {
		exists, err := s.orm.Role.Query().
			Where(role.NameEQ(*in.Name), role.IDNEQ(id), role.DeletedAtEQ(0)).
			Exist(ctx)
		if err != nil {
			return nil, errors.ErrDatabase.Wrapf(err, "检查角色名称失败")
		}
		if exists {
			return nil, errors.ErrConflict.Errorf("角色名称已存在")
		}
	}

	// 检查角色编码是否已存在（排除自己）
	if in.Code != nil {
		exists, err := s.orm.Role.Query().
			Where(role.CodeEQ(*in.Code), role.IDNEQ(id), role.DeletedAtEQ(0)).
			Exist(ctx)
		if err != nil {
			return nil, errors.ErrDatabase.Wrapf(err, "检查角色编码失败")
		}
		if exists {
			return nil, errors.ErrConflict.Errorf("角色编码已存在")
		}
	}

	// 开始事务
	tx, err := s.orm.Tx(ctx)
	if err != nil {
		return nil, errors.ErrDatabase.Wrapf(err, "启动事务失败")
	}
	defer tx.Rollback()

	// 更新角色
	update := tx.Role.UpdateOneID(id)

	if in.Name != nil {
		update.SetName(*in.Name)
	}
	if in.Code != nil {
		update.SetCode(*in.Code)
	}
	if in.Description != nil {
		update.SetNillableDescription(in.Description)
	}
	if in.Status != nil {
		update.SetStatus(role.Status(*in.Status))
	}
	if in.DataScope != nil {
		update.SetDataScope(role.DataScope(*in.DataScope))
	}
	if in.DeptIds != nil {
		update.SetDeptIds(in.DeptIds)
	}
	if in.SortOrder != nil {
		update.SetSortOrder(*in.SortOrder)
	}
	if in.Remark != nil {
		update.SetNillableRemark(in.Remark)
	}

	roleEntity, err = update.Save(ctx)
	if err != nil {
		return nil, errors.ErrDatabase.Wrapf(err, "更新角色失败")
	}

	// 更新菜单权限
	if in.MenuIds != nil {
		err = s.assignMenusInTx(ctx, tx, id, in.MenuIds)
		if err != nil {
			return nil, err
		}
	}

	// 提交事务
	if err = tx.Commit(); err != nil {
		return nil, errors.ErrDatabase.Wrapf(err, "提交事务失败")
	}

	return s.convertToRoleOutput(roleEntity), nil
}

// Get 获取角色详情
func (s *RoleService) Get(ctx context.Context, id string) (*types.RoleOutput, error) {
	roleEntity, err := s.orm.Role.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errors.ErrNotFound.Errorf("角色不存在")
		}
		return nil, errors.ErrDatabase.Wrapf(err, "查询角色失败")
	}

	return s.convertToRoleOutput(roleEntity), nil
}

// List 获取角色列表
func (s *RoleService) List(ctx context.Context, in *types.ListRolesInput) (*types.ListRolesOutput, error) {
	query := s.orm.Role.Query()

	// 状态筛选
	if in.Status != "" {
		query = query.Where(role.StatusEQ(role.Status(in.Status)))
	}

	// 数据权限范围筛选
	if in.DataScope != "" {
		query = query.Where(role.DataScopeEQ(role.DataScope(in.DataScope)))
	}

	// 关键词搜索
	if in.Keyword != "" {
		query = query.Where(role.Or(
			role.NameContains(in.Keyword),
			role.CodeContains(in.Keyword),
		))
	}

	// 总数
	total, err := query.Count(ctx)
	if err != nil {
		return nil, errors.ErrDatabase.Wrapf(err, "查询角色总数失败")
	}

	// 分页查询
	roles, err := query.
		Order(ent.Asc(role.FieldSortOrder), ent.Asc(role.FieldCreatedAt)).
		Offset((in.Page - 1) * in.PageSize).
		Limit(in.PageSize).
		All(ctx)
	if err != nil {
		return nil, errors.ErrDatabase.Wrapf(err, "查询角色列表失败")
	}

	// 转换结果
	var roleInfos []*types.RoleInfo
	for _, r := range roles {
		roleInfos = append(roleInfos, s.convertToRoleInfo(r))
	}

	return &types.ListRolesOutput{
		Roles: roleInfos,
		PageOutput: types.PageOutput{
			Total:    total,
			Page:     in.Page,
			PageSize: in.PageSize,
		},
	}, nil
}

// Delete 删除角色
func (s *RoleService) Delete(ctx context.Context, id string) error {
	// 查询角色
	roleEntity, err := s.orm.Role.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return errors.ErrNotFound.Errorf("角色不存在")
		}
		return errors.ErrDatabase.Wrapf(err, "查询角色失败")
	}

	// 检查是否为内置角色
	if roleEntity.IsBuiltin {
		return errors.ErrBusinessLogic.Errorf("系统内置角色不允许删除")
	}

	// 检查是否有用户使用该角色
	userCount, err := s.orm.UserRole.Query().
		Where(userrole.RoleIDEQ(id), userrole.DeletedAtEQ(0)).
		Count(ctx)
	if err != nil {
		return errors.ErrDatabase.Wrapf(err, "检查角色使用情况失败")
	}
	if userCount > 0 {
		return errors.ErrBusinessLogic.Errorf("该角色正在被用户使用，无法删除")
	}

	// 开始事务
	tx, err := s.orm.Tx(ctx)
	if err != nil {
		return errors.ErrDatabase.Wrapf(err, "启动事务失败")
	}
	defer tx.Rollback()

	now := time.Now().UnixMilli()

	// 软删除角色菜单关联
	_, err = tx.RoleMenu.Update().
		Where(rolemenu.RoleIDEQ(id), rolemenu.DeletedAtEQ(0)).
		SetDeletedAt(now).
		Save(ctx)
	if err != nil {
		return errors.ErrDatabase.Wrapf(err, "删除角色菜单关联失败")
	}

	// 软删除角色
	err = tx.Role.UpdateOneID(id).
		SetDeletedAt(now).
		Exec(ctx)
	if err != nil {
		return errors.ErrDatabase.Wrapf(err, "删除角色失败")
	}

	// 提交事务
	if err = tx.Commit(); err != nil {
		return errors.ErrDatabase.Wrapf(err, "提交事务失败")
	}

	return nil
}

// CheckDeletable 检查角色是否可删除
func (s *RoleService) CheckDeletable(ctx context.Context, id string) (*types.CheckRoleDeletableOutput, error) {
	// 查询角色
	roleEntity, err := s.orm.Role.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errors.ErrNotFound.Errorf("角色不存在")
		}
		return nil, errors.ErrDatabase.Wrapf(err, "查询角色失败")
	}

	// 检查是否为内置角色
	if roleEntity.IsBuiltin {
		return &types.CheckRoleDeletableOutput{
			Deletable: false,
			Reason:    "系统内置角色不允许删除",
		}, nil
	}

	// 检查是否有用户使用该角色
	userCount, err := s.orm.UserRole.Query().
		Where(userrole.RoleIDEQ(id), userrole.DeletedAtEQ(0)).
		Count(ctx)
	if err != nil {
		return nil, errors.ErrDatabase.Wrapf(err, "检查角色使用情况失败")
	}
	if userCount > 0 {
		return &types.CheckRoleDeletableOutput{
			Deletable: false,
			Reason:    fmt.Sprintf("该角色正在被%d个用户使用，无法删除", userCount),
		}, nil
	}

	return &types.CheckRoleDeletableOutput{
		Deletable: true,
		Reason:    "",
	}, nil
}

// AssignMenus 分配角色菜单权限
func (s *RoleService) AssignMenus(ctx context.Context, id string, in *types.AssignRoleMenusInput) error {
	// 检查角色是否存在
	exists, err := s.orm.Role.Query().
		Where(role.IDEQ(id), role.DeletedAtEQ(0)).
		Exist(ctx)
	if err != nil {
		return errors.ErrDatabase.Wrapf(err, "检查角色失败")
	}
	if !exists {
		return errors.ErrNotFound.Errorf("角色不存在")
	}

	// 开始事务
	tx, err := s.orm.Tx(ctx)
	if err != nil {
		return errors.ErrDatabase.Wrapf(err, "启动事务失败")
	}
	defer tx.Rollback()

	err = s.assignMenusInTx(ctx, tx, id, in.MenuIds)
	if err != nil {
		return err
	}

	// 提交事务
	if err = tx.Commit(); err != nil {
		return errors.ErrDatabase.Wrapf(err, "提交事务失败")
	}

	return nil
}

// GetRoleMenus 获取角色菜单权限
func (s *RoleService) GetRoleMenus(ctx context.Context, id string) (*types.RoleMenusOutput, error) {
	// 检查角色是否存在
	exists, err := s.orm.Role.Query().
		Where(role.IDEQ(id), role.DeletedAtEQ(0)).
		Exist(ctx)
	if err != nil {
		return nil, errors.ErrDatabase.Wrapf(err, "检查角色失败")
	}
	if !exists {
		return nil, errors.ErrNotFound.Errorf("角色不存在")
	}

	// 查询角色菜单关联
	roleMenus, err := s.orm.RoleMenu.Query().
		Where(rolemenu.RoleIDEQ(id), rolemenu.DeletedAtEQ(0)).
		All(ctx)
	if err != nil {
		return nil, errors.ErrDatabase.Wrapf(err, "查询角色菜单权限失败")
	}

	var menuIds []string
	for _, rm := range roleMenus {
		menuIds = append(menuIds, rm.MenuID)
	}

	return &types.RoleMenusOutput{
		MenuIds: menuIds,
	}, nil
}

// assignMenusInTx 在事务中分配菜单权限
func (s *RoleService) assignMenusInTx(ctx context.Context, tx *ent.Tx, roleID string, menuIds []string) error {
	now := time.Now().UnixMilli()

	// 删除现有的角色菜单关联
	_, err := tx.RoleMenu.Update().
		Where(rolemenu.RoleIDEQ(roleID), rolemenu.DeletedAtEQ(0)).
		SetDeletedAt(now).
		Save(ctx)
	if err != nil {
		return errors.ErrDatabase.Wrapf(err, "删除现有角色菜单关联失败")
	}

	// 添加新的角色菜单关联
	for _, menuID := range menuIds {
		_, err = tx.RoleMenu.Create().
			SetID(utils.GenerateULID()).
			SetRoleID(roleID).
			SetMenuID(menuID).
			Save(ctx)
		if err != nil {
			return errors.ErrDatabase.Wrapf(err, "创建角色菜单关联失败")
		}
	}

	return nil
}

// convertToRoleOutput 转换为角色输出
func (s *RoleService) convertToRoleOutput(r *ent.Role) *types.RoleOutput {
	return &types.RoleOutput{
		RoleInfo: s.convertToRoleInfo(r),
	}
}

// convertToRoleInfo 转换为角色信息
func (s *RoleService) convertToRoleInfo(r *ent.Role) *types.RoleInfo {
	var description *string
	if r.Description != "" {
		description = &r.Description
	}
	var remark *string
	if r.Remark != "" {
		remark = &r.Remark
	}

	return &types.RoleInfo{
		ID:          r.ID,
		Name:        r.Name,
		Code:        r.Code,
		Description: description,
		Status:      string(r.Status),
		DataScope:   string(r.DataScope),
		DeptIds:     r.DeptIds,
		IsBuiltin:   r.IsBuiltin,
		SortOrder:   r.SortOrder,
		Remark:      remark,
		UserCount:   0, // 需要单独查询
		MenuCount:   0, // 需要单独查询
		CreatedAt:   r.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   r.UpdatedAt.Format(time.RFC3339),
	}
}

// GetStats 获取角色统计
func (s *RoleService) GetStats(ctx context.Context) (*types.RoleStatsOutput, error) {
	// 获取总角色数
	totalRoles, err := s.orm.Role.Query().
		Where(role.DeletedAtEQ(0)).
		Count(ctx)
	if err != nil {
		return nil, errors.ErrDatabase.Wrapf(err, "查询总角色数失败")
	}

	// 获取启用角色数
	enabledRoles, err := s.orm.Role.Query().
		Where(role.StatusEQ(role.StatusEnabled), role.DeletedAtEQ(0)).
		Count(ctx)
	if err != nil {
		return nil, errors.ErrDatabase.Wrapf(err, "查询启用角色数失败")
	}

	// 获取停用角色数
	disabledRoles, err := s.orm.Role.Query().
		Where(role.StatusEQ(role.StatusDisabled), role.DeletedAtEQ(0)).
		Count(ctx)
	if err != nil {
		return nil, errors.ErrDatabase.Wrapf(err, "查询停用角色数失败")
	}

	// 获取内置角色数
	builtinRoles, err := s.orm.Role.Query().
		Where(role.IsBuiltinEQ(true), role.DeletedAtEQ(0)).
		Count(ctx)
	if err != nil {
		return nil, errors.ErrDatabase.Wrapf(err, "查询内置角色数失败")
	}

	// 构建状态统计
	statusStats := map[string]int64{
		"enabled":  int64(enabledRoles),
		"disabled": int64(disabledRoles),
	}

	return &types.RoleStatsOutput{
		TotalRoles:    int64(totalRoles),
		EnabledRoles:  int64(enabledRoles),
		DisabledRoles: int64(disabledRoles),
		BuiltinRoles:  int64(builtinRoles),
		StatusStats:   statusStats,
	}, nil
}

// AssignUsers 分配角色用户
func (s *RoleService) AssignUsers(ctx context.Context, roleID string, in *types.AssignRoleUsersInput) error {
	// 检查角色是否存在
	exists, err := s.orm.Role.Query().
		Where(role.IDEQ(roleID), role.DeletedAtEQ(0)).
		Exist(ctx)
	if err != nil {
		return errors.ErrDatabase.Wrapf(err, "检查角色失败")
	}
	if !exists {
		return errors.ErrNotFound.Errorf("角色不存在")
	}

	// 开始事务
	tx, err := s.orm.Tx(ctx)
	if err != nil {
		return errors.ErrDatabase.Wrapf(err, "启动事务失败")
	}
	defer tx.Rollback()

	now := time.Now().UnixMilli()

	// 删除现有的用户角色关联
	_, err = tx.UserRole.Update().
		Where(userrole.RoleIDEQ(roleID), userrole.DeletedAtEQ(0)).
		SetDeletedAt(now).
		Save(ctx)
	if err != nil {
		return errors.ErrDatabase.Wrapf(err, "删除现有用户角色关联失败")
	}

	// 添加新的用户角色关联
	for _, userID := range in.UserIds {
		_, err = tx.UserRole.Create().
			SetID(utils.GenerateULID()).
			SetUserID(userID).
			SetRoleID(roleID).
			Save(ctx)
		if err != nil {
			return errors.ErrDatabase.Wrapf(err, "创建用户角色关联失败")
		}
	}

	// 提交事务
	if err = tx.Commit(); err != nil {
		return errors.ErrDatabase.Wrapf(err, "提交事务失败")
	}

	return nil
}

// GetRoleUsers 获取角色用户列表
func (s *RoleService) GetRoleUsers(ctx context.Context, roleID string, in *types.PageInput) (*types.RoleUsersOutput, error) {
	// 检查角色是否存在
	exists, err := s.orm.Role.Query().
		Where(role.IDEQ(roleID), role.DeletedAtEQ(0)).
		Exist(ctx)
	if err != nil {
		return nil, errors.ErrDatabase.Wrapf(err, "检查角色失败")
	}
	if !exists {
		return nil, errors.ErrNotFound.Errorf("角色不存在")
	}

	// 查询用户角色关联，获取用户ID列表
	userRoles, err := s.orm.UserRole.Query().
		Where(userrole.RoleIDEQ(roleID), userrole.DeletedAtEQ(0)).
		All(ctx)
	if err != nil {
		return nil, errors.ErrDatabase.Wrapf(err, "查询用户角色关联失败")
	}

	if len(userRoles) == 0 {
		return &types.RoleUsersOutput{
			Users:      []*types.UserInfo{},
			PageOutput: types.NewPageOutput(*in, 0),
		}, nil
	}

	// 获取用户ID列表
	var userIds []string
	for _, ur := range userRoles {
		userIds = append(userIds, ur.UserID)
	}

	// 查询用户列表（使用PageInput的方法）
	users, err := s.orm.User.Query().
		Where(user.IDIn(userIds...), user.DeletedAtEQ(0)).
		Offset(in.Offset()).
		Limit(in.Limit()).
		All(ctx)
	if err != nil {
		return nil, errors.ErrDatabase.Wrapf(err, "查询用户列表失败")
	}

	// 转换为用户信息
	var userInfos []*types.UserInfo
	for _, u := range users {
		userInfos = append(userInfos, convertToUserInfo(u))
	}

	return &types.RoleUsersOutput{
		Users:      userInfos,
		PageOutput: types.NewPageOutput(*in, len(userIds)),
	}, nil
}

// convertToUserInfo 转换为用户信息（简化版本，可能需要根据实际UserInfo结构调整）
func convertToUserInfo(u *ent.User) *types.UserInfo {
	var realName *string
	if u.RealName != "" {
		realName = &u.RealName
	}
	var phone *string
	if u.Phone != "" {
		phone = &u.Phone
	}
	var department *string
	if u.Department != "" {
		department = &u.Department
	}
	var position *string
	if u.Position != "" {
		position = &u.Position
	}
	var lastLoginIP *string
	if u.LastLoginIP != "" {
		lastLoginIP = &u.LastLoginIP
	}

	// TODO: 从 UserRole 关联表获取角色信息
	// 注意：这里简化处理，实际使用时需要预加载角色信息
	var roles []string = []string{}

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
