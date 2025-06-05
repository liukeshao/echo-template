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

// NewMenuService 创建菜单服务
func NewMenuService(orm *ent.Client) *MenuService {
	return &MenuService{
		orm: orm,
	}
}

// CreateMenu 创建菜单
func (s *MenuService) CreateMenu(ctx context.Context, input *types.CreateMenuInput) (*types.MenuOutput, error) {
	slog.InfoContext(ctx, "开始创建菜单", "name", input.Name)

	// 检查菜单名称是否已存在
	exists, err := s.orm.Menu.Query().
		Where(menu.NameEQ(input.Name)).
		Exist(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "检查菜单名称时发生错误", "error", err)
		return nil, errors.InternalError("创建菜单失败")
	}
	if exists {
		return nil, errors.BadRequestError("菜单名称已存在")
	}

	// 如果指定了父菜单，检查父菜单是否存在
	if input.ParentID != nil {
		parentExists, err := s.orm.Menu.Query().
			Where(menu.IDEQ(*input.ParentID)).
			Exist(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "检查父菜单时发生错误", "error", err)
			return nil, errors.InternalError("创建菜单失败")
		}
		if !parentExists {
			return nil, errors.BadRequestError("父菜单不存在")
		}
	}

	// 生成ULID
	id := utils.GenerateULID()

	// 创建菜单
	query := s.orm.Menu.Create().
		SetID(id).
		SetName(input.Name).
		SetTitle(input.Title).
		SetType(menu.Type(input.Type)).
		SetStatus(menu.Status(input.Status)).
		SetHidden(input.Hidden).
		SetSortOrder(input.SortOrder).
		SetKeepAlive(input.KeepAlive).
		SetHideBreadcrumb(input.HideBreadcrumb).
		SetAlwaysShow(input.AlwaysShow)

	if input.Icon != nil {
		query.SetIcon(*input.Icon)
	}
	if input.Path != nil {
		query.SetPath(*input.Path)
	}
	if input.Component != nil {
		query.SetComponent(*input.Component)
	}
	if input.ParentID != nil {
		query.SetParentID(*input.ParentID)
	}
	if input.Permission != nil {
		query.SetPermission(*input.Permission)
	}
	if input.Description != nil {
		query.SetDescription(*input.Description)
	}
	if input.ExternalLink != nil {
		query.SetExternalLink(*input.ExternalLink)
	}

	menuEntity, err := query.Save(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "创建菜单失败", "error", err)
		return nil, errors.InternalError("创建菜单失败")
	}

	slog.InfoContext(ctx, "菜单创建成功", "id", menuEntity.ID)

	return &types.MenuOutput{
		MenuInfo: s.convertToMenuInfo(menuEntity),
	}, nil
}

// GetMenuByID 根据ID获取菜单
func (s *MenuService) GetMenuByID(ctx context.Context, id string) (*types.MenuOutput, error) {
	slog.InfoContext(ctx, "根据ID获取菜单", "id", id)

	menuEntity, err := s.orm.Menu.Query().
		Where(menu.IDEQ(id)).
		WithChildren(func(q *ent.MenuQuery) {
			q.Where(menu.StatusEQ(menu.StatusActive)).
				Order(ent.Asc(menu.FieldSortOrder))
		}).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errors.NotFoundError("菜单不存在")
		}
		slog.ErrorContext(ctx, "获取菜单失败", "error", err)
		return nil, errors.InternalError("获取菜单失败")
	}

	return &types.MenuOutput{
		MenuInfo: s.convertToMenuInfo(menuEntity),
	}, nil
}

// UpdateMenu 更新菜单
func (s *MenuService) UpdateMenu(ctx context.Context, id string, input *types.UpdateMenuInput) (*types.MenuOutput, error) {
	slog.InfoContext(ctx, "开始更新菜单", "id", id)

	// 检查菜单是否存在
	exists, err := s.orm.Menu.Query().
		Where(menu.IDEQ(id)).
		Exist(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "检查菜单时发生错误", "error", err)
		return nil, errors.InternalError("更新菜单失败")
	}
	if !exists {
		return nil, errors.NotFoundError("菜单不存在")
	}

	// 检查菜单名称是否已被其他菜单使用
	if input.Name != nil {
		nameExists, err := s.orm.Menu.Query().
			Where(menu.NameEQ(*input.Name), menu.IDNEQ(id)).
			Exist(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "检查菜单名称时发生错误", "error", err)
			return nil, errors.InternalError("更新菜单失败")
		}
		if nameExists {
			return nil, errors.BadRequestError("菜单名称已存在")
		}
	}

	// 如果更新父菜单，检查父菜单是否存在，以及避免循环引用
	if input.ParentID != nil {
		if *input.ParentID == id {
			return nil, errors.BadRequestError("不能将自己设为父菜单")
		}

		// 检查是否会造成循环引用
		if err := s.checkCircularReference(ctx, id, *input.ParentID); err != nil {
			return nil, err
		}

		parentExists, err := s.orm.Menu.Query().
			Where(menu.IDEQ(*input.ParentID)).
			Exist(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "检查父菜单时发生错误", "error", err)
			return nil, errors.InternalError("更新菜单失败")
		}
		if !parentExists {
			return nil, errors.BadRequestError("父菜单不存在")
		}
	}

	// 构建更新查询
	query := s.orm.Menu.UpdateOneID(id)

	if input.Name != nil {
		query.SetName(*input.Name)
	}
	if input.Title != nil {
		query.SetTitle(*input.Title)
	}
	if input.Icon != nil {
		query.SetIcon(*input.Icon)
	}
	if input.Path != nil {
		query.SetPath(*input.Path)
	}
	if input.Component != nil {
		query.SetComponent(*input.Component)
	}
	if input.ParentID != nil {
		query.SetParentID(*input.ParentID)
	}
	if input.Type != nil {
		query.SetType(menu.Type(*input.Type))
	}
	if input.Status != nil {
		query.SetStatus(menu.Status(*input.Status))
	}
	if input.Hidden != nil {
		query.SetHidden(*input.Hidden)
	}
	if input.SortOrder != nil {
		query.SetSortOrder(*input.SortOrder)
	}
	if input.Permission != nil {
		query.SetPermission(*input.Permission)
	}
	if input.Description != nil {
		query.SetDescription(*input.Description)
	}
	if input.ExternalLink != nil {
		query.SetExternalLink(*input.ExternalLink)
	}
	if input.KeepAlive != nil {
		query.SetKeepAlive(*input.KeepAlive)
	}
	if input.HideBreadcrumb != nil {
		query.SetHideBreadcrumb(*input.HideBreadcrumb)
	}
	if input.AlwaysShow != nil {
		query.SetAlwaysShow(*input.AlwaysShow)
	}

	// 执行更新
	menuEntity, err := query.Save(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "更新菜单失败", "error", err)
		return nil, errors.InternalError("更新菜单失败")
	}

	slog.InfoContext(ctx, "菜单更新成功", "id", id)

	return &types.MenuOutput{
		MenuInfo: s.convertToMenuInfo(menuEntity),
	}, nil
}

// DeleteMenu 删除菜单
func (s *MenuService) DeleteMenu(ctx context.Context, id string) error {
	slog.InfoContext(ctx, "开始删除菜单", "id", id)

	// 检查菜单是否存在
	exists, err := s.orm.Menu.Query().
		Where(menu.IDEQ(id)).
		Exist(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "检查菜单时发生错误", "error", err)
		return errors.InternalError("删除菜单失败")
	}
	if !exists {
		return errors.NotFoundError("菜单不存在")
	}

	// 检查是否有子菜单
	hasChildren, err := s.orm.Menu.Query().
		Where(menu.ParentIDEQ(id)).
		Exist(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "检查子菜单时发生错误", "error", err)
		return errors.InternalError("删除菜单失败")
	}
	if hasChildren {
		return errors.BadRequestError("存在子菜单，无法删除")
	}

	// 执行删除（软删除）
	err = s.orm.Menu.DeleteOneID(id).Exec(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "删除菜单失败", "error", err)
		return errors.InternalError("删除菜单失败")
	}

	slog.InfoContext(ctx, "菜单删除成功", "id", id)
	return nil
}

// ListMenus 获取菜单列表
func (s *MenuService) ListMenus(ctx context.Context, input *types.ListMenusInput) (*types.ListMenusOutput, error) {
	slog.InfoContext(ctx, "获取菜单列表", "tree_mode", input.TreeMode)

	if input.TreeMode {
		return s.getMenuTree(ctx, input)
	}

	// 构建查询
	query := s.orm.Menu.Query()

	// 添加筛选条件
	if input.ParentID != nil {
		query.Where(menu.ParentIDEQ(*input.ParentID))
	}
	if input.Type != nil {
		query.Where(menu.TypeEQ(menu.Type(*input.Type)))
	}
	if input.Status != nil {
		query.Where(menu.StatusEQ(menu.Status(*input.Status)))
	}
	if input.Keyword != nil && *input.Keyword != "" {
		query.Where(menu.Or(
			menu.NameContains(*input.Keyword),
			menu.TitleContains(*input.Keyword),
		))
	}

	// 统计总数
	total, err := query.Count(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "统计菜单总数失败", "error", err)
		return nil, errors.InternalError("获取菜单列表失败")
	}

	// 分页查询
	offset := (input.Page - 1) * input.PageSize
	menus, err := query.
		Order(ent.Asc(menu.FieldSortOrder), ent.Asc(menu.FieldCreatedAt)).
		Offset(offset).
		Limit(input.PageSize).
		All(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "获取菜单列表失败", "error", err)
		return nil, errors.InternalError("获取菜单列表失败")
	}

	// 转换为输出类型
	menuInfos := make([]*types.MenuInfo, len(menus))
	for i, menuEntity := range menus {
		menuInfos[i] = s.convertToMenuInfo(menuEntity)
	}

	// 计算总页数
	totalPages := (int(total) + input.PageSize - 1) / input.PageSize

	return &types.ListMenusOutput{
		Menus:      menuInfos,
		Total:      int64(total),
		Page:       input.Page,
		PageSize:   input.PageSize,
		TotalPages: totalPages,
	}, nil
}

// getMenuTree 获取菜单树
func (s *MenuService) getMenuTree(ctx context.Context, input *types.ListMenusInput) (*types.ListMenusOutput, error) {
	// 构建查询
	query := s.orm.Menu.Query()

	// 添加筛选条件
	if input.Type != nil {
		query.Where(menu.TypeEQ(menu.Type(*input.Type)))
	}
	if input.Status != nil {
		query.Where(menu.StatusEQ(menu.Status(*input.Status)))
	}
	if input.Keyword != nil && *input.Keyword != "" {
		query.Where(menu.Or(
			menu.NameContains(*input.Keyword),
			menu.TitleContains(*input.Keyword),
		))
	}

	// 获取所有菜单
	allMenus, err := query.
		Order(ent.Asc(menu.FieldSortOrder), ent.Asc(menu.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "获取菜单树失败", "error", err)
		return nil, errors.InternalError("获取菜单树失败")
	}

	// 转换为MenuInfo并构建树形结构
	menuInfos := make([]*types.MenuInfo, len(allMenus))
	menuMap := make(map[string]*types.MenuInfo)

	for i, menuEntity := range allMenus {
		menuInfo := s.convertToMenuInfo(menuEntity)
		menuInfos[i] = menuInfo
		menuMap[menuInfo.ID] = menuInfo
	}

	// 构建树形结构
	var rootMenus []*types.MenuInfo
	for _, menuInfo := range menuInfos {
		if menuInfo.ParentID == nil {
			rootMenus = append(rootMenus, menuInfo)
		} else {
			if parent, exists := menuMap[*menuInfo.ParentID]; exists {
				if parent.Children == nil {
					parent.Children = make([]*types.MenuInfo, 0)
				}
				parent.Children = append(parent.Children, menuInfo)
			}
		}
	}

	return &types.ListMenusOutput{
		Menus:      rootMenus,
		Total:      int64(len(allMenus)),
		Page:       1,
		PageSize:   len(allMenus),
		TotalPages: 1,
	}, nil
}

// UpdateMenuOrder 更新菜单排序
func (s *MenuService) UpdateMenuOrder(ctx context.Context, input *types.UpdateMenuOrderInput) error {
	slog.InfoContext(ctx, "开始更新菜单排序", "count", len(input.MenuOrders))

	// 开启事务
	tx, err := s.orm.Tx(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "开启事务失败", "error", err)
		return errors.InternalError("更新菜单排序失败")
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	// 批量更新菜单排序
	for _, order := range input.MenuOrders {
		query := tx.Menu.UpdateOneID(order.ID).
			SetSortOrder(order.SortOrder)

		if order.ParentID != nil {
			query.SetParentID(*order.ParentID)
		} else {
			query.ClearParentID()
		}

		err := query.Exec(ctx)
		if err != nil {
			tx.Rollback()
			slog.ErrorContext(ctx, "更新菜单排序失败", "id", order.ID, "error", err)
			return errors.InternalError("更新菜单排序失败")
		}
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		slog.ErrorContext(ctx, "提交事务失败", "error", err)
		return errors.InternalError("更新菜单排序失败")
	}

	slog.InfoContext(ctx, "菜单排序更新成功")
	return nil
}

// GetMenuStats 获取菜单统计
func (s *MenuService) GetMenuStats(ctx context.Context) (*types.MenuStatsOutput, error) {
	slog.InfoContext(ctx, "获取菜单统计")

	// 总菜单数
	totalMenus, err := s.orm.Menu.Query().Count(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "统计总菜单数失败", "error", err)
		return nil, errors.InternalError("获取菜单统计失败")
	}

	// 活跃菜单数
	activeMenus, err := s.orm.Menu.Query().
		Where(menu.StatusEQ(menu.StatusActive)).
		Count(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "统计活跃菜单数失败", "error", err)
		return nil, errors.InternalError("获取菜单统计失败")
	}

	// 非活跃菜单数
	inactiveMenus, err := s.orm.Menu.Query().
		Where(menu.StatusEQ(menu.StatusInactive)).
		Count(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "统计非活跃菜单数失败", "error", err)
		return nil, errors.InternalError("获取菜单统计失败")
	}

	// 按类型统计
	menusByType := make(map[string]int64)

	menuCount, err := s.orm.Menu.Query().
		Where(menu.TypeEQ(menu.TypeMenu)).
		Count(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "统计菜单类型失败", "error", err)
		return nil, errors.InternalError("获取菜单统计失败")
	}
	menusByType["menu"] = int64(menuCount)

	buttonCount, err := s.orm.Menu.Query().
		Where(menu.TypeEQ(menu.TypeButton)).
		Count(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "统计按钮类型失败", "error", err)
		return nil, errors.InternalError("获取菜单统计失败")
	}
	menusByType["button"] = int64(buttonCount)

	linkCount, err := s.orm.Menu.Query().
		Where(menu.TypeEQ(menu.TypeLink)).
		Count(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "统计链接类型失败", "error", err)
		return nil, errors.InternalError("获取菜单统计失败")
	}
	menusByType["link"] = int64(linkCount)

	return &types.MenuStatsOutput{
		TotalMenus:    int64(totalMenus),
		ActiveMenus:   int64(activeMenus),
		InactiveMenus: int64(inactiveMenus),
		MenusByType:   menusByType,
	}, nil
}

// checkCircularReference 检查循环引用
func (s *MenuService) checkCircularReference(ctx context.Context, menuID, parentID string) error {
	visited := make(map[string]bool)
	current := parentID

	for current != "" {
		if visited[current] {
			return errors.BadRequestError("检测到循环引用")
		}
		if current == menuID {
			return errors.BadRequestError("不能将子菜单设为父菜单")
		}

		visited[current] = true

		parent, err := s.orm.Menu.Query().
			Where(menu.IDEQ(current)).
			Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				break
			}
			slog.ErrorContext(ctx, "检查循环引用时查询菜单失败", "error", err)
			return errors.InternalError("检查循环引用失败")
		}

		if parent.ParentID == nil {
			break
		}
		current = *parent.ParentID
	}

	return nil
}

// convertToMenuInfo 转换为MenuInfo
func (s *MenuService) convertToMenuInfo(menuEntity *ent.Menu) *types.MenuInfo {
	menuInfo := &types.MenuInfo{
		ID:             menuEntity.ID,
		Name:           menuEntity.Name,
		Title:          menuEntity.Title,
		Type:           string(menuEntity.Type),
		Status:         string(menuEntity.Status),
		Hidden:         menuEntity.Hidden,
		SortOrder:      menuEntity.SortOrder,
		KeepAlive:      menuEntity.KeepAlive,
		HideBreadcrumb: menuEntity.HideBreadcrumb,
		AlwaysShow:     menuEntity.AlwaysShow,
		CreatedAt:      menuEntity.CreatedAt,
		UpdatedAt:      menuEntity.UpdatedAt,
	}

	if menuEntity.Icon != "" {
		menuInfo.Icon = &menuEntity.Icon
	}
	if menuEntity.Path != "" {
		menuInfo.Path = &menuEntity.Path
	}
	if menuEntity.Component != "" {
		menuInfo.Component = &menuEntity.Component
	}
	if menuEntity.ParentID != nil {
		menuInfo.ParentID = menuEntity.ParentID
	}
	if menuEntity.Permission != "" {
		menuInfo.Permission = &menuEntity.Permission
	}
	if menuEntity.Description != "" {
		menuInfo.Description = &menuEntity.Description
	}
	if menuEntity.ExternalLink != "" {
		menuInfo.ExternalLink = &menuEntity.ExternalLink
	}

	// 处理子菜单
	if menuEntity.Edges.Children != nil {
		children := make([]*types.MenuInfo, len(menuEntity.Edges.Children))
		for i, child := range menuEntity.Edges.Children {
			children[i] = s.convertToMenuInfo(child)
		}
		menuInfo.Children = children
	}

	return menuInfo
}
