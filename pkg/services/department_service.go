package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/ent/department"
	"github.com/liukeshao/echo-template/ent/user"
	"github.com/liukeshao/echo-template/pkg/errors"
	"github.com/liukeshao/echo-template/pkg/types"
	"github.com/liukeshao/echo-template/pkg/utils"
)

// DepartmentService 部门服务
type DepartmentService struct {
	orm *ent.Client
}

// NewDepartmentService 创建部门服务实例
func NewDepartmentService(orm *ent.Client) *DepartmentService {
	return &DepartmentService{
		orm: orm,
	}
}

// toDepartmentInfo 将部门实体转换为DepartmentInfo
func (s *DepartmentService) toDepartmentInfo(d *ent.Department) *types.DepartmentInfo {
	var parentID, manager, managerID, phone, description *string

	if d.ParentID != nil && *d.ParentID != "" {
		parentID = d.ParentID
	}
	if d.Manager != "" {
		manager = &d.Manager
	}
	if d.ManagerID != "" {
		managerID = &d.ManagerID
	}
	if d.Phone != "" {
		phone = &d.Phone
	}
	if d.Description != "" {
		description = &d.Description
	}

	return &types.DepartmentInfo{
		ID:          d.ID,
		ParentID:    parentID,
		Name:        d.Name,
		Code:        d.Code,
		Manager:     manager,
		ManagerID:   managerID,
		Phone:       phone,
		Description: description,
		SortOrder:   d.SortOrder,
		Status:      string(d.Status),
		Level:       d.Level,
		Path:        d.Path,
		CreatedAt:   d.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:   d.UpdatedAt.Format("2006-01-02 15:04:05"),
		UserCount:   0, // 将在查询时填充
		Children:    []*types.DepartmentInfo{},
	}
}

// Create 创建部门
func (s *DepartmentService) Create(ctx context.Context, input *types.CreateDepartmentInput) (*types.DepartmentOutput, error) {
	// 检查部门编码是否已存在
	exists, err := s.orm.Department.Query().
		Where(department.CodeEQ(input.Code)).
		Exist(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "检查部门编码是否存在失败", "error", err)
		return nil, errors.ErrInternal.Wrapf(err, "检查部门编码失败")
	}
	if exists {
		return nil, errors.ErrConflict.With("code", input.Code).Errorf("部门编码已存在")
	}

	// 验证上级部门是否存在（如果指定了）
	var parentDept *ent.Department
	if input.ParentID != nil && *input.ParentID != "" {
		parentDept, err = s.orm.Department.Query().
			Where(department.IDEQ(*input.ParentID)).
			First(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return nil, errors.ErrNotFound.With("parent_id", *input.ParentID).Errorf("上级部门不存在")
			}
			slog.ErrorContext(ctx, "查询上级部门失败", "error", err)
			return nil, errors.ErrInternal.Wrapf(err, "查询上级部门失败")
		}

		// 检查上级部门状态
		if parentDept.Status != department.StatusActive {
			return nil, errors.ErrBadRequest.Errorf("上级部门已停用，无法在其下创建子部门")
		}
	}

	// 验证负责人是否存在（如果指定了）
	if input.ManagerID != nil && *input.ManagerID != "" {
		exists, err = s.orm.User.Query().
			Where(user.IDEQ(*input.ManagerID), user.DeletedAtEQ(0)).
			Exist(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "检查负责人是否存在失败", "error", err)
			return nil, errors.ErrInternal.Wrapf(err, "检查负责人失败")
		}
		if !exists {
			return nil, errors.ErrNotFound.With("manager_id", *input.ManagerID).Errorf("负责人不存在")
		}
	}

	// 计算层级和路径
	level := 0
	path := fmt.Sprintf("/%s", input.Code)
	if parentDept != nil {
		level = parentDept.Level + 1
		path = fmt.Sprintf("%s/%s", parentDept.Path, input.Code)
	}

	// 设置默认状态
	status := input.Status
	if status == "" {
		status = types.DepartmentStatusActive
	}

	// 创建部门
	createQuery := s.orm.Department.Create().
		SetID(utils.GenerateULID()).
		SetName(input.Name).
		SetCode(input.Code).
		SetStatus(department.Status(status)).
		SetSortOrder(input.SortOrder).
		SetLevel(level).
		SetPath(path)

	// 设置可选字段
	if input.ParentID != nil && *input.ParentID != "" {
		createQuery = createQuery.SetParentID(*input.ParentID)
	}
	if input.Manager != nil {
		createQuery = createQuery.SetManager(*input.Manager)
	}
	if input.ManagerID != nil {
		createQuery = createQuery.SetManagerID(*input.ManagerID)
	}
	if input.Phone != nil {
		createQuery = createQuery.SetPhone(*input.Phone)
	}
	if input.Description != nil {
		createQuery = createQuery.SetDescription(*input.Description)
	}

	d, err := createQuery.Save(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "创建部门失败", "error", err)
		return nil, errors.ErrInternal.Wrapf(err, "创建部门失败")
	}

	// 获取部门用户数量
	userCount, err := s.orm.User.Query().
		Where(user.DepartmentIDEQ(d.ID)).
		Count(ctx)
	if err != nil {
		slog.WarnContext(ctx, "获取部门用户数量失败", "error", err)
	}

	departmentInfo := s.toDepartmentInfo(d)
	departmentInfo.UserCount = int64(userCount)

	return &types.DepartmentOutput{
		DepartmentInfo: departmentInfo,
	}, nil
}

// GetByID 根据ID获取部门
func (s *DepartmentService) GetByID(ctx context.Context, departmentID string) (*types.DepartmentOutput, error) {
	d, err := s.orm.Department.Query().
		Where(department.IDEQ(departmentID)).
		WithParent().
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errors.ErrNotFound.With("department_id", departmentID).Errorf("部门不存在")
		}
		slog.ErrorContext(ctx, "获取部门失败", "error", err, "department_id", departmentID)
		return nil, errors.ErrInternal.Wrapf(err, "获取部门失败")
	}

	// 获取部门用户数量
	userCount, err := s.orm.User.Query().
		Where(user.DepartmentIDEQ(d.ID)).
		Count(ctx)
	if err != nil {
		slog.WarnContext(ctx, "获取部门用户数量失败", "error", err)
	}

	departmentInfo := s.toDepartmentInfo(d)
	departmentInfo.UserCount = int64(userCount)

	// 填充上级部门信息
	if d.Edges.Parent != nil {
		departmentInfo.Parent = s.toDepartmentInfo(d.Edges.Parent)
	}

	return &types.DepartmentOutput{
		DepartmentInfo: departmentInfo,
	}, nil
}

// List 获取部门列表
func (s *DepartmentService) List(ctx context.Context, input *types.ListDepartmentsInput) (*types.ListDepartmentsOutput, error) {
	// 构建查询条件
	query := s.orm.Department.Query()

	// 根据父部门筛选
	if input.ParentID != nil {
		if *input.ParentID == "" {
			// 获取根级部门（parent_id为null）
			query = query.Where(department.ParentIDIsNil())
		} else {
			query = query.Where(department.ParentIDEQ(*input.ParentID))
		}
	}

	// 根据状态筛选
	if input.Status != "" {
		query = query.Where(department.StatusEQ(department.Status(input.Status)))
	}

	// 根据层级筛选
	if input.Level != nil {
		query = query.Where(department.LevelEQ(*input.Level))
	}

	// 根据负责人筛选
	if input.ManagerID != nil && *input.ManagerID != "" {
		query = query.Where(department.ManagerIDEQ(*input.ManagerID))
	}

	// 根据关键词搜索
	if input.Keyword != "" {
		query = query.Where(
			department.Or(
				department.NameContains(input.Keyword),
				department.CodeContains(input.Keyword),
				department.ManagerContains(input.Keyword),
			),
		)
	}

	// 获取总数
	total, err := query.Clone().Count(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "获取部门总数失败", "error", err)
		return nil, errors.ErrInternal.Wrapf(err, "获取部门总数失败")
	}

	// 应用分页和排序
	query = query.
		Order(department.ByLevel(), department.BySortOrder(), department.ByCreatedAt()).
		Limit(input.PageSize).
		Offset((input.Page - 1) * input.PageSize)

	departments, err := query.All(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "获取部门列表失败", "error", err)
		return nil, errors.ErrInternal.Wrapf(err, "获取部门列表失败")
	}

	// 转换为输出格式并填充用户数量
	departmentInfos := make([]*types.DepartmentInfo, len(departments))
	for i, dept := range departments {
		departmentInfos[i] = s.toDepartmentInfo(dept)

		// 获取部门用户数量
		userCount, err := s.orm.User.Query().
			Where(user.DepartmentIDEQ(dept.ID)).
			Count(ctx)
		if err != nil {
			slog.WarnContext(ctx, "获取部门用户数量失败", "error", err, "department_id", dept.ID)
		} else {
			departmentInfos[i].UserCount = int64(userCount)
		}
	}

	return &types.ListDepartmentsOutput{
		Departments: departmentInfos,
		PageOutput:  types.NewPageOutput(input.PageInput, total),
	}, nil
}

// Tree 获取部门树形结构
func (s *DepartmentService) Tree(ctx context.Context) (*types.DepartmentTreeOutput, error) {
	// 获取所有启用的部门
	departments, err := s.orm.Department.Query().
		Where(department.StatusEQ(department.StatusActive)).
		Order(department.ByLevel(), department.BySortOrder()).
		All(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "获取部门树形结构失败", "error", err)
		return nil, errors.ErrInternal.Wrapf(err, "获取部门树形结构失败")
	}

	// 构建部门映射和父子关系
	departmentMap := make(map[string]*types.DepartmentInfo)
	var rootDepartments []*types.DepartmentInfo

	// 第一遍：创建所有部门信息对象
	for _, dept := range departments {
		departmentInfo := s.toDepartmentInfo(dept)

		// 获取部门用户数量
		userCount, err := s.orm.User.Query().
			Where(user.DepartmentIDEQ(dept.ID)).
			Count(ctx)
		if err != nil {
			slog.WarnContext(ctx, "获取部门用户数量失败", "error", err, "department_id", dept.ID)
		} else {
			departmentInfo.UserCount = int64(userCount)
		}

		departmentMap[dept.ID] = departmentInfo
	}

	// 第二遍：构建树形结构
	for _, dept := range departments {
		departmentInfo := departmentMap[dept.ID]

		if dept.ParentID == nil || *dept.ParentID == "" {
			// 根级部门
			rootDepartments = append(rootDepartments, departmentInfo)
		} else {
			// 子部门，添加到父部门的children中
			if parent, exists := departmentMap[*dept.ParentID]; exists {
				parent.Children = append(parent.Children, departmentInfo)
			}
		}
	}

	return &types.DepartmentTreeOutput{
		Departments: rootDepartments,
	}, nil
}

// Update 更新部门
func (s *DepartmentService) Update(ctx context.Context, departmentID string, input *types.UpdateDepartmentInput) (*types.DepartmentOutput, error) {
	// 检查部门是否存在
	d, err := s.orm.Department.Query().
		Where(department.IDEQ(departmentID)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errors.ErrNotFound.With("department_id", departmentID).Errorf("部门不存在")
		}
		slog.ErrorContext(ctx, "查询部门失败", "error", err)
		return nil, errors.ErrInternal.Wrapf(err, "查询部门失败")
	}

	// 检查部门编码是否冲突（如果要更新编码）
	if input.Code != nil && *input.Code != d.Code {
		exists, err := s.orm.Department.Query().
			Where(
				department.CodeEQ(*input.Code),
				department.DeletedAtEQ(0),
				department.IDNEQ(departmentID),
			).
			Exist(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "检查部门编码冲突失败", "error", err)
			return nil, errors.ErrInternal.Wrapf(err, "检查部门编码冲突失败")
		}
		if exists {
			return nil, errors.ErrConflict.With("code", *input.Code).Errorf("部门编码已存在")
		}
	}

	// 验证负责人是否存在（如果要更新负责人）
	if input.ManagerID != nil && *input.ManagerID != "" {
		exists, err := s.orm.User.Query().
			Where(user.IDEQ(*input.ManagerID), user.DeletedAtEQ(0)).
			Exist(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "检查负责人是否存在失败", "error", err)
			return nil, errors.ErrInternal.Wrapf(err, "检查负责人失败")
		}
		if !exists {
			return nil, errors.ErrNotFound.With("manager_id", *input.ManagerID).Errorf("负责人不存在")
		}
	}

	// 构建更新查询
	updateQuery := s.orm.Department.UpdateOneID(departmentID)

	if input.Name != nil {
		updateQuery = updateQuery.SetName(*input.Name)
	}
	if input.Code != nil {
		updateQuery = updateQuery.SetCode(*input.Code)

		// 如果编码发生变化，需要更新路径
		if *input.Code != d.Code {
			newPath := d.Path
			if d.ParentID != nil {
				parentDept, err := s.orm.Department.Get(ctx, *d.ParentID)
				if err == nil {
					newPath = fmt.Sprintf("%s/%s", parentDept.Path, *input.Code)
				}
			} else {
				newPath = fmt.Sprintf("/%s", *input.Code)
			}
			updateQuery = updateQuery.SetPath(newPath)
		}
	}
	if input.Manager != nil {
		updateQuery = updateQuery.SetManager(*input.Manager)
	}
	if input.ManagerID != nil {
		if *input.ManagerID == "" {
			updateQuery = updateQuery.ClearManagerID()
		} else {
			updateQuery = updateQuery.SetManagerID(*input.ManagerID)
		}
	}
	if input.Phone != nil {
		if *input.Phone == "" {
			updateQuery = updateQuery.ClearPhone()
		} else {
			updateQuery = updateQuery.SetPhone(*input.Phone)
		}
	}
	if input.Description != nil {
		if *input.Description == "" {
			updateQuery = updateQuery.ClearDescription()
		} else {
			updateQuery = updateQuery.SetDescription(*input.Description)
		}
	}
	if input.SortOrder != nil {
		updateQuery = updateQuery.SetSortOrder(*input.SortOrder)
	}
	if input.Status != nil {
		updateQuery = updateQuery.SetStatus(department.Status(*input.Status))
	}

	updatedDept, err := updateQuery.Save(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "更新部门失败", "error", err)
		return nil, errors.ErrInternal.Wrapf(err, "更新部门失败")
	}

	// 获取部门用户数量
	userCount, err := s.orm.User.Query().
		Where(user.DepartmentIDEQ(updatedDept.ID)).
		Count(ctx)
	if err != nil {
		slog.WarnContext(ctx, "获取部门用户数量失败", "error", err)
	}

	departmentInfo := s.toDepartmentInfo(updatedDept)
	departmentInfo.UserCount = int64(userCount)

	return &types.DepartmentOutput{
		DepartmentInfo: departmentInfo,
	}, nil
}

// Move 移动部门（调整父节点）
func (s *DepartmentService) Move(ctx context.Context, departmentID string, input *types.MoveDepartmentInput) (*types.DepartmentOutput, error) {
	// 检查部门是否存在
	d, err := s.orm.Department.Query().
		Where(department.IDEQ(departmentID)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errors.ErrNotFound.With("department_id", departmentID).Errorf("部门不存在")
		}
		slog.ErrorContext(ctx, "查询部门失败", "error", err)
		return nil, errors.ErrInternal.Wrapf(err, "查询部门失败")
	}

	// 验证新的上级部门
	var newParentDept *ent.Department
	if input.ParentID != nil && *input.ParentID != "" {
		// 检查是否试图将部门移动到其自己或其子部门下
		if *input.ParentID == departmentID {
			return nil, errors.ErrBadRequest.Errorf("不能将部门移动到自己下面")
		}

		// 检查是否试图移动到子部门下（会形成循环）
		isDescendant, err := s.isDescendant(ctx, *input.ParentID, departmentID)
		if err != nil {
			return nil, err
		}
		if isDescendant {
			return nil, errors.ErrBadRequest.Errorf("不能将部门移动到其子部门下")
		}

		newParentDept, err = s.orm.Department.Query().
			Where(department.IDEQ(*input.ParentID)).
			First(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return nil, errors.ErrNotFound.With("parent_id", *input.ParentID).Errorf("上级部门不存在")
			}
			slog.ErrorContext(ctx, "查询上级部门失败", "error", err)
			return nil, errors.ErrInternal.Wrapf(err, "查询上级部门失败")
		}

		// 检查上级部门状态
		if newParentDept.Status != department.StatusActive {
			return nil, errors.ErrBadRequest.Errorf("上级部门已停用，无法移动到其下")
		}
	}

	// 计算新的层级和路径
	newLevel := 0
	newPath := fmt.Sprintf("/%s", d.Code)
	if newParentDept != nil {
		newLevel = newParentDept.Level + 1
		newPath = fmt.Sprintf("%s/%s", newParentDept.Path, d.Code)
	}

	// 开始事务
	tx, err := s.orm.Tx(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "开始事务失败", "error", err)
		return nil, errors.ErrInternal.Wrapf(err, "开始事务失败")
	}
	defer tx.Rollback()

	// 更新部门的父级、层级和路径
	updateQuery := tx.Department.UpdateOneID(departmentID).
		SetLevel(newLevel).
		SetPath(newPath)

	if input.ParentID != nil && *input.ParentID != "" {
		updateQuery = updateQuery.SetParentID(*input.ParentID)
	} else {
		updateQuery = updateQuery.ClearParentID()
	}

	updatedDept, err := updateQuery.Save(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "更新部门失败", "error", err)
		return nil, errors.ErrInternal.Wrapf(err, "更新部门失败")
	}

	// 递归更新所有子部门的层级和路径
	err = s.updateChildrenPathAndLevel(ctx, tx, departmentID, newPath, newLevel)
	if err != nil {
		return nil, err
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		slog.ErrorContext(ctx, "提交事务失败", "error", err)
		return nil, errors.ErrInternal.Wrapf(err, "提交事务失败")
	}

	// 获取部门用户数量
	userCount, err := s.orm.User.Query().
		Where(user.DepartmentIDEQ(updatedDept.ID)).
		Count(ctx)
	if err != nil {
		slog.WarnContext(ctx, "获取部门用户数量失败", "error", err)
	}

	departmentInfo := s.toDepartmentInfo(updatedDept)
	departmentInfo.UserCount = int64(userCount)

	return &types.DepartmentOutput{
		DepartmentInfo: departmentInfo,
	}, nil
}

// isDescendant 检查 descendantID 是否是 ancestorID 的子孙部门
func (s *DepartmentService) isDescendant(ctx context.Context, descendantID, ancestorID string) (bool, error) {
	current := descendantID
	visited := make(map[string]bool)

	for current != "" {
		if visited[current] {
			// 检测到循环，防止无限循环
			return false, nil
		}
		visited[current] = true

		if current == ancestorID {
			return true, nil
		}

		// 查找当前部门的父部门
		dept, err := s.orm.Department.Query().
			Where(department.IDEQ(current)).
			First(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return false, nil
			}
			return false, errors.ErrInternal.Wrapf(err, "查询部门失败")
		}

		if dept.ParentID == nil {
			break
		}
		current = *dept.ParentID
	}

	return false, nil
}

// updateChildrenPathAndLevel 递归更新子部门的路径和层级
func (s *DepartmentService) updateChildrenPathAndLevel(ctx context.Context, tx *ent.Tx, parentID, parentPath string, parentLevel int) error {
	// 查找所有直接子部门
	children, err := tx.Department.Query().
		Where(department.ParentIDEQ(parentID)).
		All(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "查询子部门失败", "error", err)
		return errors.ErrInternal.Wrapf(err, "查询子部门失败")
	}

	for _, child := range children {
		newChildLevel := parentLevel + 1
		newChildPath := fmt.Sprintf("%s/%s", parentPath, child.Code)

		// 更新子部门
		_, err := tx.Department.UpdateOneID(child.ID).
			SetLevel(newChildLevel).
			SetPath(newChildPath).
			Save(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "更新子部门失败", "error", err, "child_id", child.ID)
			return errors.ErrInternal.Wrapf(err, "更新子部门失败")
		}

		// 递归更新子部门的子部门
		err = s.updateChildrenPathAndLevel(ctx, tx, child.ID, newChildPath, newChildLevel)
		if err != nil {
			return err
		}
	}

	return nil
}

// Sort 部门排序
func (s *DepartmentService) Sort(ctx context.Context, input *types.SortDepartmentInput) error {
	// 开始事务
	tx, err := s.orm.Tx(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "开始事务失败", "error", err)
		return errors.ErrInternal.Wrapf(err, "开始事务失败")
	}
	defer tx.Rollback()

	// 批量更新排序
	for _, sortItem := range input.DepartmentSorts {
		_, err := tx.Department.UpdateOneID(sortItem.ID).
			SetSortOrder(sortItem.SortOrder).
			Save(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "更新部门排序失败", "error", err, "department_id", sortItem.ID)
			return errors.ErrInternal.Wrapf(err, "更新部门排序失败")
		}
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		slog.ErrorContext(ctx, "提交事务失败", "error", err)
		return errors.ErrInternal.Wrapf(err, "提交事务失败")
	}

	return nil
}

// CheckDeletable 检查部门是否可删除
func (s *DepartmentService) CheckDeletable(ctx context.Context, departmentID string) (*types.CheckDepartmentDeletableOutput, error) {
	// 检查部门是否存在
	exists, err := s.orm.Department.Query().
		Where(department.IDEQ(departmentID)).
		Exist(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "检查部门是否存在失败", "error", err)
		return nil, errors.ErrInternal.Wrapf(err, "检查部门是否存在失败")
	}
	if !exists {
		return nil, errors.ErrNotFound.With("department_id", departmentID).Errorf("部门不存在")
	}

	// 检查是否有关联用户
	userCount, err := s.orm.User.Query().
		Where(user.DepartmentIDEQ(departmentID)).
		Count(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "检查部门关联用户失败", "error", err)
		return nil, errors.ErrInternal.Wrapf(err, "检查部门关联用户失败")
	}

	// 检查是否有子部门
	childrenCount, err := s.orm.Department.Query().
		Where(department.ParentIDEQ(departmentID)).
		Count(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "检查子部门失败", "error", err)
		return nil, errors.ErrInternal.Wrapf(err, "检查子部门失败")
	}

	deletable := userCount == 0 && childrenCount == 0
	reason := ""
	if !deletable {
		var reasons []string
		if userCount > 0 {
			reasons = append(reasons, fmt.Sprintf("部门下还有 %d 个用户", userCount))
		}
		if childrenCount > 0 {
			reasons = append(reasons, fmt.Sprintf("部门下还有 %d 个子部门", childrenCount))
		}
		reason = "无法删除：" + strings.Join(reasons, "，")
	}

	return &types.CheckDepartmentDeletableOutput{
		Deletable:     deletable,
		Reason:        reason,
		UserCount:     int64(userCount),
		ChildrenCount: int64(childrenCount),
	}, nil
}

// Delete 删除部门
func (s *DepartmentService) Delete(ctx context.Context, departmentID string) error {
	// 检查部门是否可删除
	check, err := s.CheckDeletable(ctx, departmentID)
	if err != nil {
		return err
	}

	if !check.Deletable {
		return errors.ErrBadRequest.Errorf("%s", check.Reason)
	}

	// 执行逻辑删除
	err = s.orm.Department.DeleteOneID(departmentID).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return errors.ErrNotFound.With("department_id", departmentID).Errorf("部门不存在")
		}
		slog.ErrorContext(ctx, "删除部门失败", "error", err)
		return errors.ErrInternal.Wrapf(err, "删除部门失败")
	}

	return nil
}

// Stats 获取部门统计信息
func (s *DepartmentService) Stats(ctx context.Context) (*types.DepartmentStatsOutput, error) {
	// 获取总部门数
	total, err := s.orm.Department.Query().
		Count(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "获取部门总数失败", "error", err)
		return nil, errors.ErrInternal.Wrapf(err, "获取部门总数失败")
	}

	// 获取启用部门数
	active, err := s.orm.Department.Query().
		Where(department.StatusEQ(department.StatusActive)).
		Count(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "获取启用部门数失败", "error", err)
		return nil, errors.ErrInternal.Wrapf(err, "获取启用部门数失败")
	}

	// 获取停用部门数
	inactive, err := s.orm.Department.Query().
		Where(department.StatusEQ(department.StatusInactive)).
		Count(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "获取停用部门数失败", "error", err)
		return nil, errors.ErrInternal.Wrapf(err, "获取停用部门数失败")
	}

	// 构建状态分布
	statusBreakdown := map[string]int64{
		types.DepartmentStatusActive:   int64(active),
		types.DepartmentStatusInactive: int64(inactive),
	}

	// 获取层级统计
	type LevelCountResult struct {
		Level int `json:"level"`
		Count int `json:"count"`
	}

	var levelCounts []LevelCountResult
	err = s.orm.Department.Query().
		GroupBy(department.FieldLevel).
		Aggregate(ent.Count()).
		Scan(ctx, &levelCounts)
	if err != nil {
		slog.WarnContext(ctx, "获取层级统计失败", "error", err)
		levelCounts = []LevelCountResult{}
	}

	// 转换为输出格式
	levelStats := make([]types.LevelStat, len(levelCounts))
	for i, lc := range levelCounts {
		levelStats[i] = types.LevelStat{
			Level:           lc.Level,
			DepartmentCount: int64(lc.Count),
		}
	}

	return &types.DepartmentStatsOutput{
		TotalDepartments:    int64(total),
		ActiveDepartments:   int64(active),
		InactiveDepartments: int64(inactive),
		StatusBreakdown:     statusBreakdown,
		LevelStats:          levelStats,
	}, nil
}
