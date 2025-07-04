package services

import (
	"context"
	"log/slog"

	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/ent/menu"
	"github.com/liukeshao/echo-template/pkg/errors"
	"github.com/liukeshao/echo-template/pkg/types"
	"github.com/liukeshao/echo-template/pkg/utils"
)

// MenuService 菜单服务
type MenuService struct {
	orm *ent.Client
}

// NewMenuService 创建菜单服务实例
func NewMenuService(orm *ent.Client) *MenuService {
	return &MenuService{
		orm: orm,
	}
}

// toMenuInfo 将菜单实体转换为MenuInfo
func (s *MenuService) toMenuInfo(m *ent.Menu) *types.MenuInfo {
	// 处理可选字段的指针转换
	var parentID, path, component, icon, permission, externalLink, remark *string

	if m.ParentID != nil {
		parentID = m.ParentID
	}
	if m.Path != "" {
		path = &m.Path
	}
	if m.Component != "" {
		component = &m.Component
	}
	if m.Icon != "" {
		icon = &m.Icon
	}
	if m.Permission != "" {
		permission = &m.Permission
	}
	if m.ExternalLink != "" {
		externalLink = &m.ExternalLink
	}
	if m.Remark != "" {
		remark = &m.Remark
	}

	return &types.MenuInfo{
		ID:           m.ID,
		Name:         m.Name,
		Type:         string(m.Type),
		ParentID:     parentID,
		Path:         path,
		Component:    component,
		Icon:         icon,
		SortOrder:    m.SortOrder,
		Permission:   permission,
		Status:       string(m.Status),
		Visible:      m.Visible,
		KeepAlive:    m.KeepAlive,
		ExternalLink: externalLink,
		Remark:       remark,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}

// CreateMenu 创建菜单
func (s *MenuService) CreateMenu(ctx context.Context, input *types.CreateMenuInput) (*types.MenuOutput, error) {
	// 验证上级菜单是否存在
	if input.ParentID != nil && *input.ParentID != "" {
		exists, err := s.orm.Menu.Query().
			Where(menu.IDEQ(*input.ParentID)).
			Exist(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "检查上级菜单是否存在失败", "error", err)
			return nil, errors.ErrInternal.Wrapf(err, "检查上级菜单失败")
		}
		if !exists {
			return nil, errors.ErrNotFound.With("parent_id", *input.ParentID).Errorf("上级菜单不存在")
		}
	}

	// 检查路由地址是否已存在（如果提供了路径）
	if input.Path != nil && *input.Path != "" {
		exists, err := s.orm.Menu.Query().
			Where(menu.PathEQ(*input.Path), menu.DeletedAtEQ(0)).
			Exist(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "检查路由地址是否存在失败", "error", err)
			return nil, errors.ErrInternal.With("error", err.Error()).Errorf("检查路由地址失败")
		}
		if exists {
			return nil, errors.ErrConflict.With("path", *input.Path).Errorf("路由地址已存在")
		}
	}

	// 检查权限标识是否已存在（如果提供了权限标识）
	if input.Permission != nil && *input.Permission != "" {
		exists, err := s.orm.Menu.Query().
			Where(menu.PermissionEQ(*input.Permission), menu.DeletedAtEQ(0)).
			Exist(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "检查权限标识是否存在失败", "error", err)
			return nil, errors.ErrInternal.With("error", err.Error()).Errorf("检查权限标识失败")
		}
		if exists {
			return nil, errors.ErrConflict.With("permission", *input.Permission).Errorf("权限标识已存在")
		}
	}

	// 设置默认状态
	status := input.Status
	if status == "" {
		status = types.MenuStatusEnabled
	}

	// 创建菜单
	createQuery := s.orm.Menu.Create().
		SetID(utils.GenerateULID()).
		SetName(input.Name).
		SetType(menu.Type(input.Type)).
		SetSortOrder(input.SortOrder).
		SetStatus(menu.Status(status)).
		SetVisible(input.Visible).
		SetKeepAlive(input.KeepAlive)

	// 设置可选字段
	if input.ParentID != nil && *input.ParentID != "" {
		createQuery = createQuery.SetParentID(*input.ParentID)
	}
	if input.Path != nil {
		createQuery = createQuery.SetPath(*input.Path)
	}
	if input.Component != nil {
		createQuery = createQuery.SetComponent(*input.Component)
	}
	if input.Icon != nil {
		createQuery = createQuery.SetIcon(*input.Icon)
	}
	if input.Permission != nil {
		createQuery = createQuery.SetPermission(*input.Permission)
	}
	if input.ExternalLink != nil {
		createQuery = createQuery.SetExternalLink(*input.ExternalLink)
	}
	if input.Remark != nil {
		createQuery = createQuery.SetRemark(*input.Remark)
	}

	m, err := createQuery.Save(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "创建菜单失败", "error", err)
		return nil, errors.ErrInternal.With("error", err.Error()).Errorf("创建菜单失败")
	}

	return &types.MenuOutput{
		MenuInfo: s.toMenuInfo(m),
	}, nil
}

// GetMenuByID 根据ID获取菜单
func (s *MenuService) GetMenuByID(ctx context.Context, menuID string) (*types.MenuOutput, error) {
	m, err := s.orm.Menu.Query().
		Where(menu.IDEQ(menuID)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			slog.WarnContext(ctx, "菜单不存在", "menu_id", menuID)
			return nil, errors.ErrNotFound.With("menu_id", menuID).Errorf("菜单不存在")
		}
		slog.ErrorContext(ctx, "获取菜单失败", "error", err, "menu_id", menuID)
		return nil, errors.ErrInternal.Wrapf(err, "获取菜单失败")
	}

	return &types.MenuOutput{
		MenuInfo: s.toMenuInfo(m),
	}, nil
}

// ListMenus 获取菜单列表
func (s *MenuService) ListMenus(ctx context.Context, input *types.ListMenusInput) (*types.ListMenusOutput, error) {
	query := s.orm.Menu.Query()

	// 根据类型筛选
	if input.Type != "" {
		query = query.Where(menu.TypeEQ(menu.Type(input.Type)))
	}

	// 根据上级菜单筛选
	if input.ParentID != "" {
		if input.ParentID == "null" {
			// 查询顶级菜单
			query = query.Where(menu.ParentIDIsNil())
		} else {
			query = query.Where(menu.ParentIDEQ(input.ParentID))
		}
	}

	// 根据状态筛选
	if input.Status != "" {
		query = query.Where(menu.StatusEQ(menu.Status(input.Status)))
	}

	// 关键词搜索
	if input.Keyword != "" {
		query = query.Where(
			menu.Or(
				menu.NameContains(input.Keyword),
				menu.PermissionContains(input.Keyword),
			),
		)
	}

	// 按排序号和创建时间排序
	menus, err := query.
		Order(menu.ByParentID(), menu.BySortOrder(), menu.ByCreatedAt()).
		All(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "获取菜单列表失败", "error", err)
		return nil, errors.ErrInternal.Wrapf(err, "获取菜单列表失败")
	}

	// 转换为输出格式
	result := make([]*types.MenuInfo, len(menus))
	for i, m := range menus {
		result[i] = s.toMenuInfo(m)
	}

	return &types.ListMenusOutput{
		Menus: result,
	}, nil
}

// GetMenuTree 获取菜单树
func (s *MenuService) GetMenuTree(ctx context.Context, input *types.ListMenusInput) (*types.MenuTreeOutput, error) {
	// 获取所有菜单
	allMenus, err := s.ListMenus(ctx, input)
	if err != nil {
		return nil, err
	}

	// 构建菜单树
	tree := s.buildMenuTree(allMenus.Menus, nil)

	return &types.MenuTreeOutput{
		Tree: tree,
	}, nil
}

// buildMenuTree 构建菜单树
func (s *MenuService) buildMenuTree(menus []*types.MenuInfo, parentID *string) []*types.MenuInfo {
	var result []*types.MenuInfo

	for _, menu := range menus {
		// 判断是否为当前父级的子菜单
		if (parentID == nil && menu.ParentID == nil) ||
			(parentID != nil && menu.ParentID != nil && *menu.ParentID == *parentID) {

			// 递归构建子菜单
			menu.Children = s.buildMenuTree(menus, &menu.ID)
			result = append(result, menu)
		}
	}

	return result
}

// UpdateMenu 更新菜单
func (s *MenuService) UpdateMenu(ctx context.Context, menuID string, input *types.UpdateMenuInput) (*types.MenuOutput, error) {
	// 检查菜单是否存在
	m, err := s.orm.Menu.Query().
		Where(menu.IDEQ(menuID)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errors.ErrNotFound.With("menu_id", menuID).Errorf("菜单不存在")
		}
		return nil, errors.ErrInternal.Wrapf(err, "获取菜单失败")
	}

	// 验证上级菜单
	if input.ParentID != nil {
		if *input.ParentID != "" {
			// 检查上级菜单是否存在
			exists, err := s.orm.Menu.Query().
				Where(menu.IDEQ(*input.ParentID)).
				Exist(ctx)
			if err != nil {
				slog.ErrorContext(ctx, "检查上级菜单是否存在失败", "error", err)
				return nil, errors.ErrInternal.Wrapf(err, "检查上级菜单失败")
			}
			if !exists {
				return nil, errors.ErrNotFound.With("parent_id", *input.ParentID).Errorf("上级菜单不存在")
			}

			// 检查是否形成循环引用
			if err := s.checkCircularReference(ctx, menuID, *input.ParentID); err != nil {
				return nil, err
			}
		}
	}

	// 检查路由地址唯一性
	if input.Path != nil && *input.Path != "" && *input.Path != m.Path {
		exists, err := s.orm.Menu.Query().
			Where(menu.PathEQ(*input.Path), menu.DeletedAtEQ(0), menu.IDNEQ(menuID)).
			Exist(ctx)
		if err != nil {
			return nil, errors.ErrInternal.With("error", err.Error()).Errorf("检查路由地址失败")
		}
		if exists {
			return nil, errors.ErrConflict.With("path", *input.Path).Errorf("路由地址已存在")
		}
	}

	// 检查权限标识唯一性
	if input.Permission != nil && *input.Permission != "" && *input.Permission != m.Permission {
		exists, err := s.orm.Menu.Query().
			Where(menu.PermissionEQ(*input.Permission), menu.DeletedAtEQ(0), menu.IDNEQ(menuID)).
			Exist(ctx)
		if err != nil {
			return nil, errors.ErrInternal.With("error", err.Error()).Errorf("检查权限标识失败")
		}
		if exists {
			return nil, errors.ErrConflict.With("permission", *input.Permission).Errorf("权限标识已存在")
		}
	}

	// 更新菜单
	updateQuery := s.orm.Menu.UpdateOneID(menuID)

	if input.Name != nil {
		updateQuery = updateQuery.SetName(*input.Name)
	}
	if input.Type != nil {
		updateQuery = updateQuery.SetType(menu.Type(*input.Type))
	}
	if input.ParentID != nil {
		if *input.ParentID == "" {
			updateQuery = updateQuery.ClearParentID()
		} else {
			updateQuery = updateQuery.SetParentID(*input.ParentID)
		}
	}
	if input.Path != nil {
		updateQuery = updateQuery.SetPath(*input.Path)
	}
	if input.Component != nil {
		updateQuery = updateQuery.SetComponent(*input.Component)
	}
	if input.Icon != nil {
		updateQuery = updateQuery.SetIcon(*input.Icon)
	}
	if input.SortOrder != nil {
		updateQuery = updateQuery.SetSortOrder(*input.SortOrder)
	}
	if input.Permission != nil {
		updateQuery = updateQuery.SetPermission(*input.Permission)
	}
	if input.Status != nil {
		updateQuery = updateQuery.SetStatus(menu.Status(*input.Status))
	}
	if input.Visible != nil {
		updateQuery = updateQuery.SetVisible(*input.Visible)
	}
	if input.KeepAlive != nil {
		updateQuery = updateQuery.SetKeepAlive(*input.KeepAlive)
	}
	if input.ExternalLink != nil {
		updateQuery = updateQuery.SetExternalLink(*input.ExternalLink)
	}
	if input.Remark != nil {
		updateQuery = updateQuery.SetRemark(*input.Remark)
	}

	updatedMenu, err := updateQuery.Save(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "更新菜单失败", "error", err)
		return nil, errors.ErrInternal.With("error", err.Error()).Errorf("更新菜单失败")
	}

	return &types.MenuOutput{
		MenuInfo: s.toMenuInfo(updatedMenu),
	}, nil
}

// DeleteMenu 删除菜单
func (s *MenuService) DeleteMenu(ctx context.Context, menuID string) error {
	// 检查菜单是否存在
	exists, err := s.orm.Menu.Query().
		Where(menu.IDEQ(menuID)).
		Exist(ctx)
	if err != nil {
		return errors.ErrInternal.Wrapf(err, "检查菜单是否存在失败")
	}
	if !exists {
		return errors.ErrNotFound.With("menu_id", menuID).Errorf("菜单不存在")
	}

	// 检查是否有子菜单
	hasChildren, err := s.orm.Menu.Query().
		Where(menu.ParentIDEQ(menuID)).
		Exist(ctx)
	if err != nil {
		return errors.ErrInternal.Wrapf(err, "检查子菜单失败")
	}
	if hasChildren {
		return errors.ErrBadRequest.Errorf("该菜单存在子菜单，无法删除")
	}

	// 删除菜单
	err = s.orm.Menu.DeleteOneID(menuID).Exec(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "删除菜单失败", "error", err)
		return errors.ErrInternal.With("error", err.Error()).Errorf("删除菜单失败")
	}

	return nil
}

// CheckMenuDeletable 检查菜单是否可删除
func (s *MenuService) CheckMenuDeletable(ctx context.Context, menuID string) (*types.CheckMenuDeletableOutput, error) {
	// 检查菜单是否存在
	exists, err := s.orm.Menu.Query().
		Where(menu.IDEQ(menuID)).
		Exist(ctx)
	if err != nil {
		return nil, errors.ErrInternal.Wrapf(err, "检查菜单是否存在失败")
	}
	if !exists {
		return nil, errors.ErrNotFound.With("menu_id", menuID).Errorf("菜单不存在")
	}

	var reasons []string

	// 检查是否有子菜单
	hasChildren, err := s.orm.Menu.Query().
		Where(menu.ParentIDEQ(menuID)).
		Exist(ctx)
	if err != nil {
		return nil, errors.ErrInternal.Wrapf(err, "检查子菜单失败")
	}
	if hasChildren {
		reasons = append(reasons, "存在子菜单")
	}

	return &types.CheckMenuDeletableOutput{
		Deletable: len(reasons) == 0,
		Reasons:   reasons,
	}, nil
}

// SortMenus 菜单排序
func (s *MenuService) SortMenus(ctx context.Context, input *types.SortMenuInput) error {
	// 开始事务
	tx, err := s.orm.Tx(ctx)
	if err != nil {
		return errors.ErrInternal.Wrapf(err, "开启事务失败")
	}
	defer tx.Rollback()

	// 批量更新排序
	for _, item := range input.MenuItems {
		// 检查菜单是否存在
		exists, err := tx.Menu.Query().
			Where(menu.IDEQ(item.ID)).
			Exist(ctx)
		if err != nil {
			return errors.ErrInternal.Wrapf(err, "检查菜单是否存在失败")
		}
		if !exists {
			return errors.ErrNotFound.With("menu_id", item.ID).Errorf("菜单不存在")
		}

		// 更新排序
		err = tx.Menu.UpdateOneID(item.ID).
			SetSortOrder(item.SortOrder).
			Exec(ctx)
		if err != nil {
			return errors.ErrInternal.Wrapf(err, "更新菜单排序失败")
		}
	}

	// 提交事务
	err = tx.Commit()
	if err != nil {
		return errors.ErrInternal.Wrapf(err, "提交事务失败")
	}

	return nil
}

// MoveMenu 移动菜单
func (s *MenuService) MoveMenu(ctx context.Context, menuID string, input *types.MoveMenuInput) error {
	// 检查菜单是否存在
	exists, err := s.orm.Menu.Query().
		Where(menu.IDEQ(menuID)).
		Exist(ctx)
	if err != nil {
		return errors.ErrInternal.Wrapf(err, "检查菜单是否存在失败")
	}
	if !exists {
		return errors.ErrNotFound.With("menu_id", menuID).Errorf("菜单不存在")
	}

	// 检查目标上级菜单
	if input.ParentID != nil && *input.ParentID != "" {
		// 检查上级菜单是否存在
		exists, err := s.orm.Menu.Query().
			Where(menu.IDEQ(*input.ParentID)).
			Exist(ctx)
		if err != nil {
			return errors.ErrInternal.Wrapf(err, "检查上级菜单失败")
		}
		if !exists {
			return errors.ErrNotFound.With("parent_id", *input.ParentID).Errorf("上级菜单不存在")
		}

		// 检查循环引用
		if err := s.checkCircularReference(ctx, menuID, *input.ParentID); err != nil {
			return err
		}
	}

	// 更新上级菜单
	updateQuery := s.orm.Menu.UpdateOneID(menuID)
	if input.ParentID == nil || *input.ParentID == "" {
		updateQuery = updateQuery.ClearParentID()
	} else {
		updateQuery = updateQuery.SetParentID(*input.ParentID)
	}

	err = updateQuery.Exec(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "移动菜单失败", "error", err)
		return errors.ErrInternal.With("error", err.Error()).Errorf("移动菜单失败")
	}

	return nil
}

// checkCircularReference 检查循环引用
func (s *MenuService) checkCircularReference(ctx context.Context, menuID, parentID string) error {
	// 如果菜单ID和父级ID相同，直接返回错误
	if menuID == parentID {
		return errors.ErrBadRequest.Errorf("不能将菜单设置为自己的子菜单")
	}

	// 递归检查所有祖先节点
	visited := make(map[string]bool)
	currentID := parentID

	for currentID != "" {
		if visited[currentID] {
			return errors.ErrBadRequest.Errorf("检测到循环引用")
		}

		if currentID == menuID {
			return errors.ErrBadRequest.Errorf("不能将菜单移动到其子菜单下")
		}

		visited[currentID] = true

		// 获取当前节点的父节点
		parent, err := s.orm.Menu.Query().
			Where(menu.IDEQ(currentID)).
			First(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				break
			}
			return errors.ErrInternal.Wrapf(err, "查询父节点失败")
		}

		if parent.ParentID == nil {
			break
		}
		currentID = *parent.ParentID
	}

	return nil
}
